package repo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gocql/gocql"
	uuidLib "github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"

	"notiboy/config"
	"notiboy/pkg/cache"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/utilities"
)

type ChatRepo struct {
	Db   *gocql.Session
	Conf *config.NotiboyConfModel
}

func (c ChatRepo) IsBlockedUser(ctx context.Context, chain, user, blockedUser string) (bool, error) {
	if cache.BlockedUserCache.IsBlockedUserInCache(chain, user, blockedUser) {
		return true, nil
	}

	return false, nil
}

func (c ChatRepo) StorePersonalChat(ctx context.Context, msg *entities.UserChat, ttl int) (*entities.Response, error) {
	batch := c.Db.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	chain := msg.Chain
	userB := msg.UserB
	userA := msg.UserA
	message := msg.Message
	msgTime := msg.SentTime
	sender := msg.Sender
	uuid := msg.Uuid
	status := msg.Status

	query := fmt.Sprintf(
		`INSERT INTO %s.%s (chain, user_a, user_b, sender, message, uuid, status, sent_time) VALUES %s USING TTL %d`,
		config.GetConfig().DB.Keyspace, consts.ChatUserTable, utilities.DBMultiValuePlaceholders(8), ttl,
	)

	batch.Query(query, chain, userA, userB, sender, message, uuid, status, msgTime)
	batch.Query(query, chain, userB, userA, sender, message, uuid, status, msgTime)

	contactsQuery := fmt.Sprintf(
		"UPDATE %s.%s SET contacts = contacts + ? WHERE chain = ? AND user = ?",
		config.GetConfig().DB.Keyspace, consts.ChatUserContactsTable,
	)
	if !cache.UserContactsCache.IsUserInContactsCache(chain, userA, userB) {
		batch.Query(contactsQuery, []string{userA}, chain, userB)
		cache.UserContactsCache.AddUserContactsCache(chain, userA, userB)
	}
	if !cache.UserContactsCache.IsUserInContactsCache(chain, userB, userA) {
		batch.Query(contactsQuery, []string{userB}, chain, userA)
		cache.UserContactsCache.AddUserContactsCache(chain, userB, userA)
	}

	err := c.Db.ExecuteBatch(batch)
	if err != nil {
		return nil, fmt.Errorf("failed to store chat %s: %w", query, err)
	}

	return &entities.Response{Message: "Successfully stored personal chat"}, nil
}

func (c ChatRepo) GetPersonalChat(ctx context.Context, chain, user string, from, until int64) (
	[]*entities.UserChat, error,
) {
	var (
		userB, sender, message, uuid, status string
		sentTime                             int64
		data                                 []*entities.UserChat
	)

	contacts := cache.UserContactsCache.GetUserContactsFromCache(chain, user)
	if len(contacts) == 0 {
		log.Warnf("No contacts for user %s", user)
		return data, nil
	}

	query := fmt.Sprintf(
		`SELECT user_b, sender, message, uuid,
status, sent_time FROM %s.%s WHERE chain = ? AND user_a = ? AND user_b IN ?`,
		config.GetConfig().DB.Keyspace, consts.ChatUserTable,
	)

	args := []interface{}{chain, user, contacts}

	var keys string
	if from != 0 {
		keys = "AND sent_time >= ?"
		args = append(args, from)
	}
	if until != 0 {
		keys += "AND sent_time <= ?"
		args = append(args, until)
	}

	if keys != "" {
		query = fmt.Sprintf("%s %s", query, keys)
	}

	iter := c.Db.Query(query, args...).Iter()
	for iter.Scan(&userB, &sender, &message, &uuid, &status, &sentTime) {
		data = append(
			data, &entities.UserChat{
				Chain:    chain,
				UserA:    user,
				UserB:    userB,
				Sender:   sender,
				Message:  message,
				Uuid:     uuid,
				Status:   status,
				SentTime: sentTime,
			},
		)
	}

	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to retrieve personal msgs")
			return data, fmt.Errorf("failed to retrieve personal msgs: %w", err)
		}
	}

	return data, nil
}

func (c ChatRepo) GetPersonalChatByUser(
	ctx context.Context, chain, user, rcvr string, pageSize int, PageState []byte,
) ([]*entities.UserChat, []byte, error) {
	var (
		userB, sender, message, uuid, status string
		sentTime                             int64
	)

	query := fmt.Sprintf(
		`SELECT user_b, sender, message, uuid, status, 
sent_time FROM %s.%s WHERE chain = ? AND user_a = ? AND user_b = ?`,
		config.GetConfig().DB.Keyspace, consts.ChatUserTable,
	)

	args := []interface{}{chain, user, rcvr}

	iter := c.Db.Query(query, args...).PageSize(pageSize).PageState(PageState).Iter()
	currPageState := iter.PageState()

	var data []*entities.UserChat
	for iter.Scan(&userB, &sender, &message, &uuid, &status, &sentTime) {
		data = append(
			data, &entities.UserChat{
				Chain:    chain,
				UserA:    user,
				UserB:    userB,
				Sender:   sender,
				Message:  message,
				Uuid:     uuid,
				Status:   status,
				SentTime: sentTime,
			},
		)
	}

	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to retrieve personal msgs")
			return nil, nil, fmt.Errorf("failed to retrieve personal msgs: %w", err)
		}
	}

	return data, currPageState, nil
}

func (c ChatRepo) BlockUser(_ context.Context, chain, user, blockedUser string) (*entities.Response, error) {
	query := fmt.Sprintf(
		`UPDATE %s.%s SET blocked_users = blocked_users + ? WHERE chain = ? AND user = ?`,
		config.GetConfig().DB.Keyspace, consts.ChatUserBlockTable,
	)

	err := c.Db.Query(query, []string{blockedUser}, chain, user).Exec()
	if err != nil {
		return &entities.Response{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to block user %s by %s: %w", blockedUser, user, err)
	}

	cache.BlockedUserCache.AddBlockedUserCache(chain, user, blockedUser)

	return &entities.Response{
		StatusCode: http.StatusOK,
		Message:    "Blocked user successfully",
	}, err
}

func (c ChatRepo) UnblockUser(_ context.Context, chain, user, blockedUser string) (*entities.Response, error) {
	query := fmt.Sprintf(
		`UPDATE %s.%s SET blocked_users = blocked_users - ? WHERE chain = ? AND user = ?`,
		config.GetConfig().DB.Keyspace, consts.ChatUserBlockTable,
	)

	err := c.Db.Query(query, []string{blockedUser}, chain, user).Exec()
	if err != nil {
		return &entities.Response{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("failed to unblock user %s by %s: %w", blockedUser, user, err)
	}

	cache.BlockedUserCache.RemoveBlockedUserCache(chain, user, blockedUser)

	return &entities.Response{
		StatusCode: http.StatusOK,
		Message:    "Unblocked user successfully",
	}, err
}

func (c ChatRepo) StoreGroupChat(ctx context.Context, msg *entities.GroupChat, ttl int) (*entities.Response, error) {
	chain := msg.Chain
	gid := msg.GID
	sender := msg.Sender
	message := msg.Message
	msgTime := utilities.UnixTime()
	uuid := uuidLib.NewString()
	status := msg.Status

	query := fmt.Sprintf(
		`INSERT INTO %s.%s (chain, gid, sender, message, uuid, status, sent_time) VALUES %s USING TTL %d`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupTable, utilities.DBMultiValuePlaceholders(7), ttl,
	)

	err := c.Db.Query(query, chain, gid, sender, message, uuid, status, msgTime).Exec()
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}

	return &entities.Response{Message: "Successfully stored group chat"}, nil
}

func (c ChatRepo) StoreGroupInfo(ctx context.Context, data *entities.GroupChatInfo) (
	*entities.Response,
	error,
) {
	chain := data.Chain
	gid := uuidLib.NewString()
	name := data.Name
	description := data.Description
	owner := data.Owner
	admins := []string{owner}
	users := []string{owner}
	createdAt := utilities.TimeNow()
	var blockedUsers []string

	query := fmt.Sprintf(
		`INSERT INTO %s.%s (chain, gid, name, description, owner, admins, users, created_at, blocked_users) VALUES %s`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupInfoTable, utilities.DBMultiValuePlaceholders(9),
	)

	err := c.Db.Query(query, chain, gid, name, description, owner, admins, users, createdAt, blockedUsers).Exec()
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}

	return &entities.Response{Message: "Successfully stored group chat info"}, nil
}

func (c ChatRepo) UpdateGroupInfo(ctx context.Context, data *entities.GroupChatInfo) (
	*entities.Response,
	error,
) {
	user := cast.ToString(ctx.Value(consts.UserAddress))

	chain := data.Chain
	gid := data.GID
	name := data.Name
	description := data.Description

	var (
		keys []string
		args []interface{}
	)
	if name != "" {
		keys = append(keys, fmt.Sprintf("name = ?"))
		args = append(args, name)
	}
	if description != "" {
		keys = append(keys, fmt.Sprintf("description = ?"))
		args = append(args, description)
	}

	if len(keys) == 0 {
		return &entities.Response{Message: "Nothing to update"}, nil
	}

	var owner string
	query := fmt.Sprintf(
		`SELECT owner FROM %s.%s WHERE chain = %s AND gid = %s`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupInfoTable, chain, gid,
	)
	err := c.Db.Query(query).Scan(&owner)
	if err != nil {
		return nil, fmt.Errorf("failed to run query to get owner: %w", err)
	}

	if owner != user {
		return nil, fmt.Errorf("user %s is not owner of this group %s", user, gid)
	}

	query = fmt.Sprintf(
		`UPDATE %s.%s SET %s WHERE chain = %s AND gid = %s IF EXISTS`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupInfoTable, strings.Join(keys, ","), chain, gid,
	)

	err = c.Db.Query(query, args).Exec()
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}

	return &entities.Response{Message: "Successfully stored group chat info"}, nil
}

func (c ChatRepo) DeleteGroup(ctx context.Context, chain, gid string) (
	*entities.Response,
	error,
) {
	user := cast.ToString(ctx.Value(consts.UserAddress))

	var owner string
	query := fmt.Sprintf(
		`SELECT owner FROM %s.%s WHERE chain = %s AND gid = %s`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupInfoTable, chain, gid,
	)
	err := c.Db.Query(query).Scan(&owner)
	if err != nil {
		return nil, fmt.Errorf("failed to run query to get owner: %w", err)
	}

	if owner != user {
		return nil, fmt.Errorf("user %s is not owner of this group %s", user, gid)
	}

	query = fmt.Sprintf(
		`DELETE FROM %s.%s WHERE chain = %s AND gid = %s`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupInfoTable, chain, gid,
	)

	err = c.Db.Query(query).Exec()
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}

	return &entities.Response{Message: "Successfully delete group"}, nil
}
func (c ChatRepo) JoinGroup(ctx context.Context, chain, gid string, users []string) (
	*entities.Response,
	error,
) {
	if len(users) == 0 {
		return &entities.Response{Message: "Nothing to update"}, nil
	}

	query := fmt.Sprintf(
		`UPDATE %s.%s SET users = users + ? WHERE chain = %s AND gid = %s IF EXISTS`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupInfoTable, chain, gid,
	)
	err := c.Db.Query(query, users).Exec()
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}

	batch := c.Db.NewBatch(gocql.UnloggedBatch).WithContext(ctx)
	groupsQuery := fmt.Sprintf(
		"UPDATE %s.%s SET groups = groups + ? WHERE chain = ? AND user = ?",
		config.GetConfig().DB.Keyspace, consts.ChatUserGroupTable,
	)
	for _, user := range users {
		if !cache.UserGroupsCache.IsUserInGroupCache(chain, user, gid) {
			batch.Query(groupsQuery, gid, chain, user)
			cache.UserGroupsCache.AddUserGroupsCache(chain, user, gid)
		}
	}

	err = c.Db.ExecuteBatch(batch)
	if err != nil {
		return nil, fmt.Errorf("failed to add users %s to group %s: %w", strings.Join(users, ","), gid, err)
	}

	return &entities.Response{Message: "Users successfully joined group"}, nil
}

func (c ChatRepo) LeaveGroup(ctx context.Context, chain, gid string, users []string) (
	*entities.Response,
	error,
) {

	if len(users) == 0 {
		return &entities.Response{Message: "Nothing to update"}, nil
	}

	query := fmt.Sprintf(
		`UPDATE %s.%s SET users = users - ? WHERE chain = %s AND gid = %s IF EXISTS`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupInfoTable, chain, gid,
	)

	err := c.Db.Query(query, users).Exec()
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}

	batch := c.Db.NewBatch(gocql.UnloggedBatch).WithContext(ctx)
	groupsQuery := fmt.Sprintf(
		"UPDATE %s.%s SET groups = groups - ? WHERE chain = ? AND user = ?",
		config.GetConfig().DB.Keyspace, consts.ChatUserGroupTable,
	)
	for _, user := range users {
		if cache.UserGroupsCache.IsUserInGroupCache(chain, user, gid) {
			batch.Query(groupsQuery, gid, chain, user)
			cache.UserGroupsCache.RemoveUserGroupsCache(chain, user, gid)
		}
	}

	err = c.Db.ExecuteBatch(batch)
	if err != nil {
		return nil, fmt.Errorf("failed to remove users %s to group %s: %w", strings.Join(users, ","), gid, err)
	}

	return &entities.Response{Message: "Users successfully left group"}, nil
}

func (c ChatRepo) GetGroupChats(
	ctx context.Context, chain, user string, from, to int64,
	limit int,
) ([]*entities.GroupChat, error) {
	var (
		sender, message, uuid, status string
		sentTime                      int64
		data                          []*entities.GroupChat
	)

	gids := cache.UserGroupsCache.GetUserGroupsFromCache(chain, user)
	if len(gids) == 0 {
		log.Warnf("No groups for user %s", user)
		return data, nil
	}

	var args []interface{}
	var keys string
	if from != 0 {
		keys = " AND sent_time >= ?"
		args = append(args, from)
	}
	if to != 0 {
		keys += " AND sent_time <= ?"
		args = append(args, to)
	}
	if limit != 0 {
		keys += " LIMIT ?"
		args = append(args, limit)
	}

	query := fmt.Sprintf(
		`SELECT sender, message, uuid, status, sent_time FROM %s.%s WHERE chain = ? AND gid = ?`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupTable,
	)
	if keys != "" {
		query = fmt.Sprintf("%s %s", query, keys)
	}

	for _, gid := range gids {
		var updatedArgs = []interface{}{chain, gid}
		updatedArgs = append(updatedArgs, args...)

		iter := c.Db.Query(query, updatedArgs...).Iter()
		for iter.Scan(&sender, &message, &uuid, &status, &sentTime) {
			data = append(
				data, &entities.GroupChat{
					Chain:    chain,
					GID:      gid,
					Sender:   sender,
					Message:  message,
					UUID:     uuid,
					Status:   status,
					SentTime: sentTime,
				},
			)
		}

		if err := iter.Close(); err != nil {
			if !errors.Is(err, gocql.ErrNotFound) {
				log.WithError(err).Error("failed to retrieve group msgs")
				return data, fmt.Errorf("failed to retrieve group msgs: %w", err)
			}
		}
	}

	return data, nil
}

func (c ChatRepo) GetGroupChatByGroup(
	ctx context.Context, chain, user, gid string, pageSize int, pageState []byte,
) ([]*entities.GroupChat, []byte, error) {
	var (
		sender, message, uuid, status string
		sentTime                      int64
		data                          []*entities.GroupChat
	)

	if !cache.UserGroupsCache.IsUserInGroupCache(chain, user, gid) {
		log.Warnf("User not present in group %s", gid)
		return data, nil, nil
	}

	query := fmt.Sprintf(
		`SELECT sender, message, uuid, status, sent_time FROM %s.%s WHERE chain = ? AND gid = ?`,
		config.GetConfig().DB.Keyspace, consts.ChatGroupTable,
	)

	var updatedArgs = []interface{}{chain, gid}

	iter := c.Db.Query(query, updatedArgs...).PageSize(pageSize).PageState(pageState).Iter()
	currPageState := iter.PageState()
	for iter.Scan(&sender, &message, &uuid, &status, &sentTime) {
		data = append(
			data, &entities.GroupChat{
				Chain:    chain,
				GID:      gid,
				Sender:   sender,
				Message:  message,
				UUID:     uuid,
				Status:   status,
				SentTime: sentTime,
			},
		)
	}

	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to retrieve group msgs")
			return data, nil, fmt.Errorf("failed to retrieve group msgs: %w", err)
		}
	}

	return data, currPageState, nil
}

func (c ChatRepo) GetDNSContactsList(ctx context.Context, chain, originUser, lookupAddr string) (map[string]string, error) {
	var (
		user, dns string
		args      []interface{}
	)

	domains := make(map[string]string)

	query := fmt.Sprintf(
		"SELECT user, dns FROM %s.%s WHERE chain = ? AND user = ?", config.GetConfig().DB.Keyspace, consts.UserDNSTable,
	)
	if lookupAddr != "" {
		args = append(args, chain, lookupAddr)
	}

	if lookupAddr == "" {
		var contacts []string

		contactsQuery := fmt.Sprintf(
			`SELECT contacts FROM %s.%s WHERE chain = ? AND user = ?`,
			config.GetConfig().DB.Keyspace, consts.ChatUserContactsTable,
		)

		err := c.Db.Query(contactsQuery, chain, originUser).Scan(&contacts)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch contacts: %w", err)
		}

		query = fmt.Sprintf(
			"SELECT user, dns FROM %s.%s WHERE chain = ? AND user IN ?", config.GetConfig().DB.Keyspace, consts.UserDNSTable,
		)
		args = append(args, chain, contacts)
	}

	iter := c.Db.Query(query, args...).Iter()
	for iter.Scan(&user, &dns) {
		domains[user] = dns
	}

	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to retrieve dns names")
			return domains, fmt.Errorf("failed to retrieve group msgs: %w", err)
		}
	}

	return domains, nil
}

type ChatRepoImpl interface {
	StorePersonalChat(ctx context.Context, data *entities.UserChat, ttl int) (*entities.Response, error)
	GetPersonalChat(ctx context.Context, chain, user string, from, until int64) ([]*entities.UserChat, error)
	GetPersonalChatByUser(
		ctx context.Context, chain, user, rcvr string, numPageSize int, currPageState []byte,
	) ([]*entities.UserChat, []byte, error)
	BlockUser(ctx context.Context, chain, user, blockedUser string) (*entities.Response, error)
	IsBlockedUser(ctx context.Context, chain, user, blockedUser string) (bool, error)
	UnblockUser(ctx context.Context, chain, user, blockedUser string) (*entities.Response, error)

	GetDNSContactsList(ctx context.Context, chain, originUser, lookupAddr string) (map[string]string, error)

	StoreGroupInfo(ctx context.Context, data *entities.GroupChatInfo) (
		*entities.Response,
		error,
	)
	UpdateGroupInfo(ctx context.Context, data *entities.GroupChatInfo) (
		*entities.Response,
		error,
	)
	DeleteGroup(ctx context.Context, chain, gid string) (
		*entities.Response,
		error,
	)
	JoinGroup(ctx context.Context, chain, gid string, users []string) (
		*entities.Response,
		error,
	)
	LeaveGroup(ctx context.Context, chain, gid string, users []string) (
		*entities.Response,
		error,
	)
	StoreGroupChat(ctx context.Context, msg *entities.GroupChat, ttl int) (*entities.Response, error)
	GetGroupChats(
		ctx context.Context, chain, user string, from, to int64,
		limit int,
	) ([]*entities.GroupChat, error)
	GetGroupChatByGroup(
		ctx context.Context, chain, user, gid string, pageSize int, pageState []byte,
	) ([]*entities.GroupChat, []byte, error)
}

func NewChatRepo(db *gocql.Session, conf *config.NotiboyConfModel) ChatRepoImpl {
	return &ChatRepo{Db: db, Conf: conf}
}
