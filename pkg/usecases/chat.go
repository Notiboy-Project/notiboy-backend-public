package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"firebase.google.com/go/v4/messaging"
	uuidLib "github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"notiboy/config"
	"notiboy/pkg/cache"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/repo"
	"notiboy/pkg/repo/driver/db"
	"notiboy/pkg/repo/driver/medium"
)

type ChatUseCases struct {
	repo          repo.ChatRepoImpl
	userRepo      repo.UserRepoImply
	ws            *medium.Socket
	notifications chan messaging.Message
}

func (c *ChatUseCases) IsBlockedUser(ctx context.Context, chain, user, blockedUser string) (bool, error) {
	return c.repo.IsBlockedUser(ctx, chain, user, blockedUser)
}

func (c *ChatUseCases) GetDNSContactsList(ctx context.Context, chain, originUser, lookupAddr string) (map[string]string, error) {
	return c.repo.GetDNSContactsList(ctx, chain, originUser, lookupAddr)
}

func (c *ChatUseCases) DeleteGroup(ctx context.Context, chain, gid string) (*entities.Response, error) {
	return c.repo.DeleteGroup(ctx, chain, gid)
}

func (c *ChatUseCases) StoreGroupInfo(ctx context.Context, data *entities.GroupChatInfo) (*entities.Response, error) {
	return c.repo.StoreGroupInfo(ctx, data)
}

func (c *ChatUseCases) UpdateGroupInfo(ctx context.Context, data *entities.GroupChatInfo) (*entities.Response, error) {
	return c.repo.UpdateGroupInfo(ctx, data)
}

func (c *ChatUseCases) JoinGroup(ctx context.Context, chain, gid string, users []string) (*entities.Response, error) {
	return c.repo.JoinGroup(ctx, chain, gid, users)
}

func (c *ChatUseCases) LeaveGroup(ctx context.Context, chain, gid string, users []string) (*entities.Response, error) {
	return c.repo.LeaveGroup(ctx, chain, gid, users)
}

func (c *ChatUseCases) GetGroupChats(ctx context.Context, chain, user string, from, to int64, limit int) ([]*entities.GroupChat, error) {
	return c.repo.GetGroupChats(ctx, chain, user, from, to, limit)
}

func (c *ChatUseCases) GetGroupChatByGroup(ctx context.Context, chain, user, gid string, pageSize int, pageState []byte) (
	[]*entities.GroupChat, []byte, error,
) {
	return c.repo.GetGroupChatByGroup(ctx, chain, user, gid, pageSize, pageState)
}

func (c *ChatUseCases) GetPersonalChat(
	ctx context.Context, chain, user string, from, until int64,
) ([]*entities.UserChat, error) {
	return c.repo.GetPersonalChat(ctx, chain, user, from, until)
}

func (c *ChatUseCases) GetPersonalChatByUser(
	ctx context.Context, chain, user, receiver string, numPageSize int, currPageState []byte,
) ([]*entities.UserChat, []byte, error) {
	return c.repo.GetPersonalChatByUser(ctx, chain, user, receiver, numPageSize, currPageState)
}

func (c *ChatUseCases) BlockUser(ctx context.Context, chain, user, blockedUser string) (*entities.Response, error) {
	return c.repo.BlockUser(ctx, chain, user, blockedUser)
}

func (c *ChatUseCases) UnblockUser(ctx context.Context, chain, user, blockedUser string) (*entities.Response, error) {
	return c.repo.UnblockUser(ctx, chain, user, blockedUser)
}

func (c *ChatUseCases) userOnlineStatus(ctx context.Context, msg medium.Message) error {
	chain := msg.Chain
	rcvr := msg.Receiver
	sender := msg.Sender

	rcvrID := medium.FormatIdentifier(chain, rcvr)
	connObj := c.ws.ConnSet[rcvrID]

	statusData := &entities.UserStatus{
		Chain: chain,
		User:  rcvr,
	}

	if connObj != nil && time.Since(connObj.LastChecked).Seconds() <= float64(30) && connObj.IsOnline {
		statusData.Online = true
	}

	mStatusData, err := json.Marshal(statusData)
	if err != nil {
		return err
	}

	senderID := medium.FormatIdentifier(chain, sender)
	return c.ws.PushMessage(senderID, mStatusData, true)
}

func (c *ChatUseCases) sendChat(ctx context.Context, msg medium.Message) error {
	chatStatus := entities.MsgSubmitted

	chain := msg.Chain
	rcvr := msg.Receiver
	data := msg.Data
	msgTime := msg.Time
	sender := msg.Sender

	uuid := uuidLib.NewString()

	isBlocked := false
	if cache.BlockedUserCache.IsBlockedUserInCache(chain, rcvr, sender) {
		logrus.Warnf("User %s is blocked from chatting with %s, chain: %s", rcvr, sender, chain)
		isBlocked = true
	}

	// store in DB
	chatData := &entities.UserChat{
		Chain:    chain,
		UserA:    sender,
		UserB:    rcvr,
		Sender:   sender,
		Message:  data,
		Uuid:     uuid,
		SentTime: msgTime,
	}

	defer func() {
		if isBlocked {
			return
		}

		chatData.Status = chatStatus
		// 7 days ttl
		ttl := config.GetConfig().Chat.Personal.Ttl
		_, err := c.repo.StorePersonalChat(ctx, chatData, ttl)
		if err != nil {
			logrus.WithError(err).Errorf("failed to store personal chat %+v", *chatData)
		}

		notification := messaging.Message{
			Notification: &messaging.Notification{
				Title: fmt.Sprintf("Chat from %s", chatData.UserA),
				Body:  chatData.Message,
			},
			Data: map[string]string{
				"click_action": "FLUTTER_NOTIFICATION_CLICK",
				"chain":        chatData.Chain,
				"user_a":       chatData.UserA,
				"user_b":       chatData.UserB,
				"sender":       chatData.Sender,
				"message":      chatData.Message,
				"uuid":         chatData.Uuid,
				"status":       chatData.Status,
				"sent_time":    strconv.FormatInt(chatData.SentTime, 10),
			},
			APNS: &messaging.APNSConfig{Payload: &messaging.APNSPayload{Aps: &messaging.Aps{Sound: "default"}}},
		}
		c.notifications <- notification
	}()

	mChatData, err := json.Marshal(*chatData)
	if err != nil {
		return fmt.Errorf("failed to marshal chat data %+v: %w", *chatData, err)
	}

	var errRcvrSend error
	if !isBlocked {
		rcvrID := medium.FormatIdentifier(chain, rcvr)
		errRcvrSend = c.ws.PushMessage(rcvrID, mChatData, true)
		if errRcvrSend != nil {
			chatStatus = entities.MsgSubmitted
			errRcvrSend = fmt.Errorf("failed to send chat message to %s: %w", rcvrID, errRcvrSend)
		} else {
			chatStatus = entities.MsgDelivered
		}
	} else {
		chatStatus = entities.MsgBlocked
	}

	chatResp := &entities.UserChatResponse{
		Uuid:   uuid,
		Status: chatStatus,
	}

	// auto acknowledge
	senderID := medium.FormatIdentifier(chain, sender)
	mChatResp, err := json.Marshal(*chatResp)
	if err != nil {
		if errRcvrSend != nil {
			return fmt.Errorf("failed to marshal chat response %+v: %w: %w", *chatResp, err, errRcvrSend)
		}
		return fmt.Errorf("failed to marshal chat response %+v: %w", *chatResp, err)
	}

	errSndrSend := c.ws.PushMessage(senderID, mChatResp, true)
	if errSndrSend != nil {
		errSndrSend = fmt.Errorf("failed to send chat auto ack message to %s: %w", senderID, errSndrSend)
	}

	if errSndrSend == nil && errRcvrSend == nil {
		return nil
	} else if errSndrSend != nil && errRcvrSend != nil {
		return fmt.Errorf("%s, %s", errSndrSend, errRcvrSend)
	} else if errSndrSend != nil {
		return errSndrSend
	} else if errRcvrSend != nil {
		return errRcvrSend
	}

	return nil
}

func (c *ChatUseCases) sendChatAck(ctx context.Context, msg medium.Message) error {
	chain := msg.Chain
	rcvr := msg.Receiver
	msgTime := msg.Time
	uuid := msg.UUID
	sender := msg.Sender

	rcvrID := medium.FormatIdentifier(chain, rcvr)
	chatResp := &entities.UserChatResponse{
		Uuid:   uuid,
		Status: entities.UserChatAckKind,
		Sender: sender,
		Time:   msgTime,
	}

	defer func() {
		// update receiver's DB status (here receiver is the one who sent the original msg and is now the recipient of the
		// ack message
		query := fmt.Sprintf(
			`UPDATE %s.%s SET status = ? WHERE chain = ? AND user_a = ? AND user_b = ? AND sent_time = ? IF EXISTS`,
			config.GetConfig().DB.Keyspace, consts.ChatUserTable,
		)

		if err := db.GetCassandraSession().Query(
			query, entities.UserChatAckKind, chain,
			rcvr, sender, msgTime,
		).Exec(); err != nil {
			logrus.WithError(err).Errorf(
				"failed to update DB with chat ack, sender %s, rcvr %s, uuid %s", sender,
				rcvr, uuid,
			)
		}

		// update sender's DB status (here sender is the one who received the original msg and is now the sender of the
		// ack message
		query = fmt.Sprintf(
			`UPDATE %s.%s SET status = ? WHERE chain = ? AND user_a = ? AND user_b = ? AND sent_time = ? IF EXISTS`,
			config.GetConfig().DB.Keyspace, consts.ChatUserTable,
		)

		if err := db.GetCassandraSession().Query(
			query, entities.UserChatSeenKind, chain,
			sender, rcvr, msgTime,
		).Exec(); err != nil {
			logrus.WithError(err).Errorf(
				"failed to update DB with chat seen, sender %s, rcvr %s, uuid %s", sender,
				rcvr, uuid,
			)
		}
	}()

	mChatResp, err := json.Marshal(*chatResp)
	if err != nil {
		return fmt.Errorf("failed to marshal chat response %+v: %w", *chatResp, err)
	}

	err = c.ws.PushMessage(rcvrID, mChatResp, true)
	if err != nil {
		return fmt.Errorf("failed to send chat ack message to %s: %w", rcvrID, err)
	}

	return nil
}

func (c *ChatUseCases) NotificationProcessor(ctx context.Context) {
	for msg := range c.notifications {
		tokens, err := c.userRepo.GetFCMTokens(
			ctx, entities.UserIdentifier{
				Chain:   msg.Data["chain"],
				Address: msg.Data["user_b"],
			},
		)
		if err != nil {
			logrus.WithError(err).Errorf("failed to get fcm token for %s:%s", msg.Data["chain"], msg.Data["user_b"])
			continue
		}

		if err = medium.GetFirebaseClient().PushMessageToClient(ctx, msg.Data["chain"], msg.Data["user_b"], msg, tokens); err != nil {
			logrus.WithError(err).Errorf("failed to push chat notification")
		}
	}
}

func (c *ChatUseCases) ChatProcessor(ctx context.Context) {
	logrus.Info("Starting chat processor")
	for msg := range c.ws.GetReadChannel() {
		chain := msg.Chain
		rcvr := msg.Receiver
		data := msg.Data
		kind := msg.Kind
		sender := msg.Sender
		uuid := msg.UUID
		sentTime := msg.Time
		logrus.Debugf(
			"Received message '%s' of kind '%s' from user '%s' to '%s' in chain '%s', uuid: %s, sent_time: %d", data, kind, sender,
			rcvr, chain, uuid, sentTime,
		)

		switch msg.Kind {
		case entities.UserChatStatusKind:
			/*
				{
				    "kind": "status",
				    "receiver": "VRRIASVEG3U3LHECHXINO2XIY7MV6P4YHR7C3XLDOKR7YFF3LOW3UD46AM",
				    "chain": "algorand"
				}
			*/
			if err := c.userOnlineStatus(ctx, msg); err != nil {
				logrus.WithError(err).Errorf("unable to check and send user online status for %s:%s", chain, rcvr)
			}
		case entities.UserChatMsgKind:
			/*
				{
				    "data": "hello dpak",
				    "receiver": "VRRIASVEG3U3LHECHXINO2XIY7MV6P4YHR7C3XLDOKR7YFF3LOW3UD46AM",
				    "chain": "algorand",
					"kind": "chat"
				}
			*/
			if err := c.sendChat(ctx, msg); err != nil {
				logrus.WithError(err).Errorf("unable to send chat to %s:%s", chain, rcvr)
			}
		case entities.UserChatAckKind:
			/*
				{
				    "uuid": "xxxxx",
					"time": "2023-12-22T11:37:41Z",
				    "receiver": "VRRIASVEG3U3LHECHXINO2XIY7MV6P4YHR7C3XLDOKR7YFF3LOW3UD46AM",
				    "chain": "algorand",
					"kind": "ack"
				}
			*/
			if err := c.sendChatAck(ctx, msg); err != nil {
				logrus.WithError(err).Errorf("unable to send chat ack to %s:%s", chain, rcvr)
			}
		}
	}
}

type ChatUseCaseImply interface {
	GetPersonalChat(ctx context.Context, chain, user string, from, until int64) ([]*entities.UserChat, error)
	GetPersonalChatByUser(
		ctx context.Context, chain, user, receiver string, numPageSize int, currPageState []byte,
	) ([]*entities.UserChat, []byte, error)
	BlockUser(ctx context.Context, chain, user, blockedUser string) (*entities.Response, error)
	UnblockUser(ctx context.Context, chain, user, blockedUser string) (*entities.Response, error)
	IsBlockedUser(ctx context.Context, chain, user, blockedUser string) (bool, error)

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
	GetGroupChats(
		ctx context.Context, chain, user string, from, to int64,
		limit int,
	) ([]*entities.GroupChat, error)
	GetGroupChatByGroup(
		ctx context.Context, chain, user, gid string, pageSize int, pageState []byte,
	) ([]*entities.GroupChat, []byte, error)

	ChatProcessor(ctx context.Context)
	NotificationProcessor(ctx context.Context)
}

func NewChatUseCases(repo repo.ChatRepoImpl, userRepo repo.UserRepoImply, ws *medium.Socket) ChatUseCaseImply {
	return &ChatUseCases{
		repo:          repo,
		userRepo:      userRepo,
		ws:            ws,
		notifications: make(chan messaging.Message, 1000),
	}
}
