package repo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cast"

	"notiboy/config"
	"notiboy/pkg/cache"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/repo/driver/db"
	"notiboy/utilities"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
)

type ChannelRepo struct {
	Db   *gocql.Session
	Conf *config.NotiboyConfModel
}

// ChannelRepoImpl is an interface that defines methods for interacting with channel-related data.
type ChannelRepoImpl interface {
	ChannelUsers(ctx context.Context, r *http.Request, req *entities.ListChannelUsersRequest) (
		*entities.Response, error,
	)
	ChannelCreate(context.Context, entities.ChannelInfo, string) (*entities.Response, error)
	ChannelUpdate(context.Context, entities.ChannelInfo) error
	ListOptedInChannels(
		ctx context.Context, chain, address string, optedInChannels []string, withLogo bool,
	) (*entities.Response, error)
	ListChannels(ctx context.Context, req *entities.ListChannelRequest) (*entities.Response, error)
	GetChannel(ctx context.Context, chain, chanel string, withLogo bool) (*entities.Response, error)
	ListUserOwnedChannels(
		ctx context.Context, r *http.Request, chain string, address string, withLogo bool,
	) (*entities.Response, error)
	DeleteChannel(ctx context.Context, chain string, appId string, address string) error
	ChannelStatistics(
		ctx context.Context, r *http.Request, chain string, typeStr, startDate, endDate string,
	) ([]entities.ChannelActivity, error)
	ChannelReadSentStatistics(
		ctx context.Context, r *http.Request, chain, channel, fetchKind, startDate, endDate string,
	) ([]entities.ChannelReadSentResponse, error)
	ChannelNotificationStatistics(
		ctx context.Context, r *http.Request, chain string, channel string, address string,
		typeStr, startDate, endDate string, limit int, page int,
	) (*ChannelStats, error)
	VerifyChannel(ctx context.Context, chain, channel string) error
	RetrieveChannelUsers(ctx context.Context, chain, appID string) ([]string, error)
}

func NewChannelRepo(db *gocql.Session, conf *config.NotiboyConfModel) ChannelRepoImpl {
	return &ChannelRepo{Db: db, Conf: conf}
}

type ChannelStats struct {
	Data        []DailyStats `json:"data"`
	TotalSent   int          `json:"total_sent"`
	StatusCode  int          `json:"status_code"`
	Message     string       `json:"message"`
	Mediums     []string     `json:"mediums"`
	TotalCount  int          `json:"total_count"`
	TotalPages  int          `json:"total_pages"`
	CurrentPage int          `json:"current_page"`
	PerPage     int          `json:"per_page"`
}

type DailyStats struct {
	Sent    int    `json:"sent"`
	Email   int    `json:"email"`
	Discord int    `json:"discord"`
	Date    string `json:"date"`
}

// ChannelReadSentStatistics returns the total sent and read counter for a channel
func (repo *ChannelRepo) ChannelReadSentStatistics(
	_ context.Context, _ *http.Request, chain, appID, fetchKind, startDate, endDate string,
) ([]entities.ChannelReadSentResponse, error) {
	log := utilities.NewLogger("ChannelReadSentStatistics")

	var respData = make([]entities.ChannelReadSentResponse, 0)

	tbl := fmt.Sprintf("%s.%s", repo.Conf.DB.Keyspace, consts.ChannelSentReadMetrics)
	query := fmt.Sprintf(
		`SELECT event_date, SUM("read") AS read, SUM("sent") AS sent FROM %s WHERE chain = ? AND channel = ?`, tbl,
	)
	if fetchKind == "range" {
		query = fmt.Sprintf(
			"%s AND event_date >= '%s' AND event_date <= '%s' GROUP BY event_date ORDER BY event_date ASC", query,
			startDate, endDate,
		)
	} else {
		query = fmt.Sprintf("%s GROUP BY event_date ORDER BY event_date ASC", query)
	}

	var (
		date string
		read int
		sent int
	)

	iter := repo.Db.Query(query, chain, appID).Iter()
	for iter.Scan(&date, &read, &sent) {
		respData = append(
			respData, entities.ChannelReadSentResponse{
				EventDate: date,
				Read:      read,
				Sent:      sent,
			},
		)
	}
	if err := iter.Close(); err != nil {
		log.WithError(err).Error("failed to close iterator for channel read sent metrics")
		return nil, err
	}

	return respData, nil
}

// ChannelNotificationStatistics retrieves the notification statistics for a specific channel and user.
func (repo *ChannelRepo) ChannelNotificationStatistics(
	_ context.Context, _ *http.Request, chain string, channel string, address string,
	typeStr, startDate, endDate string, limit int, page int,
) (*ChannelStats, error) {

	var appId string
	log := utilities.NewLogger("ChannelNotificationStatistics")

	query := fmt.Sprintf(
		`SELECT channels[?] FROM %s.%s WHERE chain = ? AND address = ?`,
		config.GetConfig().DB.Keyspace, consts.UserInfo,
	)

	if err := repo.Db.Query(query, channel, chain, address).Scan(&appId); err != nil {
		log.WithError(err).Errorf("user_info fetching fails fails")
		return nil, fmt.Errorf("no matching record found in user_info table:%w", err)
	}
	if appId != channel {
		log.Errorf("Authorization fails")
		return nil, errors.New("you are not authorised to get statistics")
	}

	query = fmt.Sprintf(
		`SELECT allowed_mediums FROM %s.%s WHERE chain = ? AND address = ?`,
		config.GetConfig().DB.Keyspace, consts.UserInfo,
	)

	result := repo.Db.Query(query, chain, address)
	var allowedMediums []string

	if err := result.Scan(&allowedMediums); err != nil {
		// No matching record found
		if errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Errorf("record not found in user_info table")
			return nil, fmt.Errorf("no matching record found in user_info table:%w", err)
		}
		return nil, err
	}

	var infoQuery string
	var infoIter *gocql.Iter
	switch typeStr {
	case "all":
		infoQuery = fmt.Sprintf(
			`SELECT event_time, sent, type FROM %s.%s WHERE chain = ? AND channel = ? AND user = ? LIMIT ?`,
			config.GetConfig().DB.Keyspace, consts.UserNotificationChannelMetrics,
		)

		infoIter = repo.Db.Query(infoQuery, chain, channel, address, limit).Iter()

	case "range":
		infoQuery = fmt.Sprintf(
			`SELECT event_time, sent, type FROM %s.%s WHERE chain = ? AND channel = ?
		AND user = ? AND event_time >= ? AND event_time < ? LIMIT ?`,
			config.GetConfig().DB.Keyspace, consts.UserNotificationChannelMetrics,
		)

		start, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			log.WithError(err).Errorf("An error occurred while parsing the start date: %v", err)
			return nil, err
		}
		end, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			log.WithError(err).Errorf("An error occurred while parsing the end date: %v", err)
			return nil, err
		}
		// Adjust the end time to the end of the day
		end = end.Add(time.Hour * 24).Add(-time.Nanosecond)
		infoIter = repo.Db.Query(infoQuery, chain, channel, address, start, end, limit).Iter()

	default:
		log.Errorf("Invalid parameter")
		return nil, errors.New("invalid type parameter")
	}

	var eventTime time.Time
	var sent int
	var medium string

	data := make(map[string]map[string]int)

	for infoIter.Scan(&eventTime, &sent, &medium) {
		dateStr := eventTime.Format("02-01-2006")

		if _, ok := data[dateStr]; !ok {
			data[dateStr] = make(map[string]int)
			data[dateStr]["sent"] = 0
			data[dateStr]["email"] = 0
			data[dateStr]["discord"] = 0
		}

		data[dateStr]["sent"] += sent

		if medium == "email" && sent == 1 {
			data[dateStr]["email"]++
		} else if medium == "discord" && sent == 1 {
			data[dateStr]["discord"]++
		}
	}

	if err := infoIter.Close(); err != nil {
		log.WithError(err).Errorf("An error occurred while closing the info iterator: %v", err)
		return nil, err
	}

	totalSent := 0
	for _, dailyData := range data {
		totalSent += dailyData["sent"]
	}

	var dates []string
	for dateStr := range data {
		dates = append(dates, dateStr)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	stats := &ChannelStats{
		Data:        make([]DailyStats, 0),
		Mediums:     allowedMediums,
		TotalCount:  len(dates),
		CurrentPage: page,
		PerPage:     limit,
		TotalPages:  int(math.Ceil(float64(len(dates)) / float64(limit))),
		TotalSent:   totalSent,
	}
	for _, dateStr := range dates {
		dailyData := data[dateStr]

		dateStats := DailyStats{
			Sent:    dailyData["sent"],
			Email:   dailyData["email"],
			Discord: dailyData["discord"],
			Date:    dateStr,
		}

		stats.Data = append(stats.Data, dateStats)
	}

	return stats, nil
}

// ChannelUsers retrieves the users associated with a specific channel.
func (repo *ChannelRepo) ChannelUsers(
	_ context.Context, _ *http.Request, req *entities.ListChannelUsersRequest,
) (*entities.Response, error) {
	chain := req.Chain
	appId := req.AppId
	address := req.Address

	log := utilities.NewLoggerWithFields(
		"ChannelUsers",
		map[string]interface{}{
			"chain":   chain,
			"appID":   appId,
			"address": address,
		},
	)

	_ = req.WithLogo
	addrOnly := req.AddressOnly

	// get list of user addresses for the given chain and appId from the channel_users table
	userQuery := fmt.Sprintf(
		`SELECT users FROM %s.%s WHERE chain = ? AND app_id = ?`,
		config.GetConfig().DB.Keyspace, consts.ChannelUsers,
	)

	userIter := repo.Db.Query(userQuery, chain, appId).Iter()

	var userAddresses []string

	for userIter.Scan(&userAddresses) {
		// do nothing here, just scan the result into the `userAddresses` slice
	}

	if err := userIter.Close(); err != nil {
		log.WithError(err).Error("failed to close iter while trying to fetch userAddresses of a channel")
		return nil, err
	}

	userInfos := make([]map[string]interface{}, 0)

	if addrOnly {
		for _, userAddr := range userAddresses {
			userInfos = append(
				userInfos, map[string]interface{}{
					"address": userAddr,
				},
			)
		}
		response := entities.Response{
			Data: userInfos,
		}

		return &response, nil
	}

	var allowedMediums, supportedMediums, channels, optins []string
	var userAddr, status, membership string

	infoQuery := fmt.Sprintf(
		"SELECT address, channels, optins, membership, status, allowed_mediums, supported_mediums FROM %s.%s WHERE chain = ? AND address in ?",
		config.GetConfig().DB.Keyspace, consts.UserInfo,
	)
	infoIter := repo.Db.Query(infoQuery, chain, userAddresses).Iter()

	for infoIter.Scan(&userAddr, &channels, &optins, &membership, &status, &allowedMediums, &supportedMediums) {
		userInfo := map[string]interface{}{
			"address":           userAddr,
			"chain":             chain,
			"allowed_mediums":   allowedMediums,
			"supported_mediums": supportedMediums,
			"status":            status,
			"membership":        membership,
			"channels":          channels,
			"optins":            optins,
		}

		userInfos = append(userInfos, userInfo)
	}

	if err := infoIter.Close(); err != nil {
		log.WithError(err).Error("failed to close iter while trying to fetch user info")
		return nil, err
	}

	// construct the response object
	response := entities.Response{
		MetaData: &entities.MetaData{},
		Data:     userInfos,
	}

	return &response, nil
}

// ChannelCreate creates a new channel with the provided information.
func (repo *ChannelRepo) ChannelCreate(_ context.Context, data entities.ChannelInfo, chain string) (
	*entities.Response, error,
) {
	type Response struct {
		AppID string `json:"app_id"`
	}

	log := utilities.NewLogger("ChannelCreate")

	now := utilities.TimeNow()
	appID := uuid.New().String()
	response := entities.Response{
		Data: Response{
			AppID: appID,
		},
	}

	if data.Logo != "" {
		logoConfig := repo.Conf.Logo
		err := utilities.ValidateImage(
			data.Logo, logoConfig.MaxX, logoConfig.MaxY,
			logoConfig.MaxSize, logoConfig.SupportedTypes,
		)
		if err != nil {
			log.WithError(err).Error("Logo validation failed")
			return nil, fmt.Errorf("logo validation failed: %w", err)
		}
	}

	query := fmt.Sprintf(
		`INSERT INTO %s.%s (chain, name, app_id, description, logo, status, owner, verified, created) VALUES %s`,
		config.GetConfig().DB.Keyspace, consts.UnverifiedChannelInfo, utilities.DBMultiValuePlaceholders(9),
	)

	if err := repo.Db.Query(
		query, chain, data.Name, appID, data.Description, data.Logo, consts.STATUS_ACTIVE, data.Address, false, now,
	).Exec(); err != nil {
		log.WithError(err).Error("Logo validation failed")
		return &response, fmt.Errorf("failed to insert channel info: %w", err)
	}

	updateQuery := fmt.Sprintf(
		`UPDATE %s.%s SET channels = channels + {'%s'} WHERE address = ? AND chain = ?`,
		config.GetConfig().DB.Keyspace, consts.UserInfo, appID,
	)

	if err := repo.Db.Query(updateQuery, data.Address, chain).Exec(); err != nil {
		log.WithError(err).Error("Failed to update user info")
		return &response, fmt.Errorf("failed to update user_info: %w", err)
	}

	updateQuery = fmt.Sprintf(
		`UPDATE %s.%s SET app_id = app_id + {'%s'} WHERE chain = ? AND name = ?`,
		config.GetConfig().DB.Keyspace, consts.ChannelName, appID,
	)

	if err := repo.Db.Query(updateQuery, chain, strings.ToLower(data.Name)).Exec(); err != nil {
		log.WithError(err).Error("failed to update channel name mapping")
		return &response, fmt.Errorf("failed to update channel name mapping: %w", err)
	}

	query = fmt.Sprintf(
		`INSERT INTO %s.%s (chain, event_date, "create", "delete", event_time) VALUES (?, ?, ?, ?, ?) USING TTL %d`,
		config.GetConfig().DB.Keyspace, consts.ChannelActivityMetrics, config.GetConfig().TTL.Metrics,
	)

	if err := repo.Db.Query(query, chain, utilities.ToDate(now), 1, 0, now).Exec(); err != nil {
		log.WithError(err).Error("Failed to insert channel activity metrics into the database")
		return &response, fmt.Errorf("failed to insert channel activity metrics: %w", err)
	}

	cache.GetChannelNameCache().Add(chain, data.Name, appID)

	return &response, nil
}

// ChannelUpdate updates the information of an existing channel.
func (repo *ChannelRepo) ChannelUpdate(ctx context.Context, data entities.ChannelInfo) error {
	log := utilities.NewLoggerWithFields(
		"ChannelUpdate", map[string]interface{}{
			"app-id": data.AppID,
			"chain":  data.Chain,
			"name":   data.Name,
		},
	)

	if data.Logo != "" {
		logoConfig := repo.Conf.Logo
		err := utilities.ValidateImage(
			data.Logo, logoConfig.MaxX, logoConfig.MaxY,
			logoConfig.MaxSize, logoConfig.SupportedTypes,
		)
		if err != nil {
			return fmt.Errorf("logo validation failed: %w", err)
		}
	}

	userModel, err := db.GetUserModel(ctx, data.Chain, data.Address)
	if err != nil {
		return fmt.Errorf("getting user model failed")
	}

	if !utilities.ContainsString(userModel.Channels, data.AppID) {
		return fmt.Errorf("user %s is not owner of channel %s", data.Address, data.AppID)
	}

	var (
		currentName string
		currentDesc string
		currentLogo string
	)

	tbl := consts.UnverifiedChannelInfo
	if cache.GetChannelVerifyCache().IsVerified(data.Chain, data.AppID) {
		tbl = consts.VerifiedChannelInfo
	}

	channelQuery := fmt.Sprintf(
		"SELECT name, description, logo FROM %s.%s where app_id = ? AND chain = ?",
		config.GetConfig().DB.Keyspace, tbl,
	)
	err = repo.Db.Query(channelQuery, data.AppID, data.Chain).Scan(&currentName, &currentDesc, &currentLogo)
	if err != nil {
		return fmt.Errorf("failed to retrieve channel details: %w", err)
	}

	setClause := make([]string, 0)
	args := make([]interface{}, 0)
	if data.Description != "" && currentDesc != data.Description {
		setClause = append(setClause, "description = ?")
		args = append(args, data.Description)
	}
	if data.Logo != "" && currentLogo != data.Logo {
		setClause = append(setClause, "logo = ?")
		args = append(args, data.Logo)
	}
	if data.Name != "" && currentName != data.Name {
		memTier := consts.MembershipStringToEnum(userModel.Membership)
		if !consts.ChannelRename[memTier] {
			return fmt.Errorf("membership tier %s doesn't allow setting channel name", memTier)
		}

		setClause = append(setClause, "name = ?")
		args = append(args, data.Name)
	}

	if len(setClause) == 0 {
		log.Infof("nothing to update here")
		return nil
	}

	updateQuery := fmt.Sprintf(
		`UPDATE %s.%s SET %s WHERE app_id = ? and chain = ?`,
		config.GetConfig().DB.Keyspace, tbl, strings.Join(setClause, ","),
	)

	args = append(args, data.AppID, data.Chain)
	if err = repo.Db.Query(updateQuery, args...).Exec(); err != nil {
		return fmt.Errorf("failed to execute update query: %w", err)
	}

	//this is only for unverifying the channel
	if data.Name != "" {
		if !cache.GetChannelVerifyCache().IsVerified(data.Chain, data.AppID) {
			return nil
		}

		if err = repo.unVerifyChannel(ctx, data.Chain, data.AppID); err != nil {
			return fmt.Errorf("failed to unverify the channel: %w", err)
		}
	}

	log.Info("Channel updated successfully")

	return nil
}

// DeleteChannel deletes a channel with the specified chain, appID, and address.
func (repo *ChannelRepo) DeleteChannel(ctx context.Context, chain, appID, address string) error {

	log := utilities.NewLogger("DeleteChannel")

	now := utilities.TimeNow()
	var (
		owner       string
		channelName string
	)

	tbl := consts.UnverifiedChannelInfo
	if cache.GetChannelVerifyCache().IsVerified(chain, appID) {
		tbl = consts.VerifiedChannelInfo
	}

	query := fmt.Sprintf(
		`SELECT owner, name FROM %s.%s WHERE chain=? AND app_id=?`,
		config.GetConfig().DB.Keyspace, tbl,
	)

	err := repo.Db.Query(query, chain, appID).Scan(&owner, &channelName)
	if err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			log.Errorf("No record found for chain %s and app_id %s", chain, appID)
			return fmt.Errorf("no record found for chain %s and app_id %s", chain, appID)
		}
		return fmt.Errorf("failed to get owner information of channel %s: %w", appID, err)
	}

	if owner != address {
		log.Errorf("user %s not an owner of app_id %s", address, appID)
		return fmt.Errorf("user %s not an owner of chain %s and app_id %s", address, chain, appID)
	}

	// Delete channel info
	query = fmt.Sprintf(
		`DELETE FROM %s.%s WHERE chain=? AND app_id=? IF EXISTS`,
		config.GetConfig().DB.Keyspace, tbl,
	)

	if err := repo.Db.Query(query, chain, appID).Exec(); err != nil {
		log.WithError(err).Error("Failed to delete channel info from the database")
		return fmt.Errorf("failed to delete channel info: %w", err)
	}

	query = fmt.Sprintf(
		`UPDATE %s.%s SET app_id = app_id - {'%s'} WHERE chain = ? AND name = ?`,
		config.GetConfig().DB.Keyspace, consts.ChannelName, appID,
	)

	if err = repo.Db.Query(query, chain, strings.ToLower(channelName)).Exec(); err != nil {
		log.WithError(err).Error("failed to update channel name mapping")
		return fmt.Errorf("failed to update channel name mapping: %w", err)
	}

	query = fmt.Sprintf(
		`INSERT INTO %s.%s (chain, event_date, "create", "delete", event_time) VALUES (?, ?, ?, ?, ?) USING TTL %d`,
		config.GetConfig().DB.Keyspace, consts.ChannelActivityMetrics, config.GetConfig().TTL.Metrics,
	)
	if err := repo.Db.Query(query, chain, utilities.ToDate(now), 0, 1, now).Exec(); err != nil {
		log.WithError(err).Error("Failed to delete channel activity metrics from the database")
		return fmt.Errorf("failed to delete channel activity metrics: %w", err)
	}

	query = fmt.Sprintf(
		`DELETE channels[?] FROM %s.%s WHERE chain = ? AND address = ?`,
		config.GetConfig().DB.Keyspace, consts.UserInfo,
	)

	if err := repo.Db.Query(query, appID, chain, address).Exec(); err != nil {
		log.WithError(err).Error("Failed to remove channel from user_info")
		return fmt.Errorf("failed to remove channel from user_info: %w", err)
	}

	var userAddresses []string
	query = fmt.Sprintf(
		`SELECT users FROM %s.%s WHERE chain = ? AND app_id = ?`,
		config.GetConfig().DB.Keyspace, consts.ChannelUsers,
	)
	if err = repo.Db.Query(query, chain, appID).Scan(&userAddresses); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Warn("channel doesn't have users")
		} else {
			log.WithError(err).Error("failed to get channel users list")
		}
	}

	// Delete channel users
	query = fmt.Sprintf(
		`DELETE FROM %s.%s WHERE chain=? AND app_id=? IF EXISTS`,
		config.GetConfig().DB.Keyspace, consts.ChannelUsers,
	)

	if err := repo.Db.Query(query, chain, appID).Exec(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("Failed to delete channel users from the database")
		}
	}

	batch := repo.Db.NewBatch(gocql.UnloggedBatch).WithContext(ctx)
	for _, userAddress := range userAddresses {
		query = fmt.Sprintf(
			`DELETE optins[?] FROM %s.%s WHERE chain = ? AND address = ? IF EXISTS`,
			config.GetConfig().DB.Keyspace, consts.UserInfo,
		)
		batch.Query(query, appID, chain, userAddress)
	}

	if err = repo.Db.ExecuteBatch(batch); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to delete channel from users optin")
		}
	}

	query = fmt.Sprintf(
		`DELETE FROM %s.%s WHERE chain = ? AND channel = ?`,
		config.GetConfig().DB.Keyspace, consts.NotificationChannelCounter,
	)

	if err = repo.Db.Query(query, chain, appID).Exec(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to delete channel notification counter")
		}
	}

	query = fmt.Sprintf(
		`DELETE FROM %s.%s WHERE chain = ? AND channel = ?`,
		config.GetConfig().DB.Keyspace, consts.ChannelTractionMetrics,
	)

	if err = repo.Db.Query(query, chain, appID).Exec(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to delete channel traction metrics")
		}
	}

	cache.GetChannelNameCache().Pop(chain, channelName, appID)

	return nil
}

// GetChannel retrieves a  channel for the specified chain
func (repo *ChannelRepo) GetChannel(_ context.Context, chain, channelID string, withLogo bool) (
	*entities.Response, error,
) {
	log := utilities.NewLogger("GetChannel")

	tbl := consts.UnverifiedChannelInfo
	if cache.GetChannelVerifyCache().IsVerified(chain, channelID) {
		tbl = consts.VerifiedChannelInfo
	}

	channelList, _, err := repo.getChannels([]string{channelID}, tbl, chain, withLogo, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get channels: %w", err)
	}

	if len(channelList) != 1 {
		return nil, fmt.Errorf("channel with ID %s not found", channelID)
	}

	// construct the response object
	response := &entities.Response{
		Data: channelList[0],
	}

	log.Infof("Channel %s retrieved", channelID)

	return response, nil
}

func (repo *ChannelRepo) getChannelsUnfilteredAppIDs(
	appIDs []string, chain string, withLogo bool,
) ([]entities.ChannelModel, error) {
	verifiedChannels := make([]string, 0)
	unverifiedChannels := make([]string, 0)

	for _, appID := range appIDs {
		if cache.GetChannelVerifyCache().IsVerified(chain, appID) {
			verifiedChannels = append(verifiedChannels, appID)
		} else {
			unverifiedChannels = append(unverifiedChannels, appID)
		}
	}

	channelModels := make([]entities.ChannelModel, 0)

	if len(verifiedChannels) > 0 {
		channelList, _, err := repo.getChannels(verifiedChannels, consts.VerifiedChannelInfo, chain, withLogo, 0, nil)
		if err != nil {
			return nil, fmt.Errorf("faield to get verified channels: %w", err)
		}
		channelModels = append(channelModels, channelList...)
	}

	if len(unverifiedChannels) > 0 {
		channelList, _, err := repo.getChannels(
			unverifiedChannels, consts.UnverifiedChannelInfo, chain, withLogo, 0, nil,
		)
		if err != nil {
			return nil, fmt.Errorf("faield to get unverified channels: %w", err)
		}
		channelModels = append(channelModels, channelList...)
	}

	return channelModels, nil
}

func (repo *ChannelRepo) getChannels(
	appIDs []string, tbl, chain string, withLogo bool, pageSize int, pageState []byte,
) ([]entities.ChannelModel, string, error) {
	log := utilities.NewLogger("repo.getChannels")

	var (
		appId       string
		name        string
		verified    bool
		owner       string
		description string
		logo        string
		status      string
		created     time.Time
	)

	channelModels := make([]entities.ChannelModel, 0)

	whereClause := "chain = ?"
	whereVal := []interface{}{chain}
	getClause := []string{"app_id, name, verified, owner, description, status, created"}
	args := []interface{}{&appId, &name, &verified, &owner, &description, &status, &created}
	if withLogo {
		getClause = append(getClause, "logo")
		args = append(args, &logo)
	}

	if appIDs != nil && len(appIDs) > 0 {
		whereClause = fmt.Sprintf("%s AND app_id in ?", whereClause)
		whereVal = append(whereVal, appIDs)
	}
	channelQuery := fmt.Sprintf(
		"SELECT %s FROM %s.%s where %s",
		strings.Join(getClause, ","), config.GetConfig().DB.Keyspace, tbl, whereClause,
	)
	channelIter := repo.Db.Query(channelQuery, whereVal...).PageSize(pageSize).PageState(pageState).Iter()
	currPageState := channelIter.PageState()
	currPageStateStr := base64.URLEncoding.EncodeToString(currPageState)

	for channelIter.Scan(args...) {
		channelModel := entities.ChannelModel{
			Name:             name,
			Description:      description,
			Chain:            chain,
			AppID:            appId,
			Owner:            owner,
			Verified:         verified,
			Logo:             logo,
			Status:           status,
			CreatedTimestamp: created,
		}

		channelModels = append(channelModels, channelModel)
	}

	if err := channelIter.Close(); err != nil {
		log.WithError(err).Error("failed to close channel iterator")
		return channelModels, currPageStateStr, err
	}

	return channelModels, currPageStateStr, nil
}

func (repo *ChannelRepo) ListOptedInChannels(
	_ context.Context, chain, _ string, optedInChannels []string, withLogo bool,
) (*entities.Response, error) {
	log := utilities.NewLogger("ListOptedInChannels")

	channelModels, err := repo.getChannelsUnfilteredAppIDs(optedInChannels, chain, withLogo)
	if err != nil {
		return nil, err
	}

	// construct the response object
	response := &entities.Response{
		Data: channelModels,
	}

	log.Info("Successfully retrieved list of opted in channels")

	return response, nil
}

// ListChannels retrieves a list of channels for the specified chain, with pagination support.
func (repo *ChannelRepo) ListChannels(ctx context.Context, req *entities.ListChannelRequest) (
	*entities.Response, error,
) {
	log := utilities.NewLogger("ListChannels")

	chain := req.Chain
	pageSize := req.PageSize
	pageState := req.NextToken
	withLogo := req.WithLogo
	searchName := req.NameSearch
	verifiedKind := req.Verified

	if searchName != "" {
		return repo.ListChannelsByName(ctx, chain, searchName, withLogo)
	}

	var (
		currPageStateStr string
		err              error
	)
	channelModels := make([]entities.ChannelModel, 0)

	if verifiedKind {
		channelModels, currPageStateStr, err = repo.getChannels(
			nil, consts.VerifiedChannelInfo, chain, withLogo, pageSize, pageState,
		)
		if err != nil {
			return nil, fmt.Errorf("faield to get verified channels: %w", err)
		}
	} else {
		channelModels, currPageStateStr, err = repo.getChannels(
			nil, consts.UnverifiedChannelInfo, chain, withLogo, pageSize, pageState,
		)
		if err != nil {
			return nil, fmt.Errorf("faield to get unverified channels: %w", err)
		}
	}

	// construct the response object
	response := &entities.Response{
		PaginationMetaData: &entities.PaginationMetaData{
			Size:     len(channelModels),
			PageSize: pageSize,
			Next:     currPageStateStr,
		},
		Data: channelModels,
	}

	log.Info("Successfully retrieved list of channels")

	return response, nil
}

// ListChannelsByName retrieves a list of channels for the specified chain, with pagination support.
func (repo *ChannelRepo) ListChannelsByName(
	ctx context.Context, chain, channelName string, withLogo bool,
) (*entities.Response, error) {
	log := utilities.NewLogger("ListChannelsByName")

	appIDs := cache.GetChannelNameCache().GetAppIDs(chain, channelName)

	channelModels, err := repo.getChannelsUnfilteredAppIDs(appIDs, chain, withLogo)
	if err != nil {
		return nil, err
	}

	// construct the response object
	response := &entities.Response{
		PaginationMetaData: &entities.PaginationMetaData{
			Size: len(channelModels),
		},
		Data: channelModels,
	}
	log.Info("Successfully retrieved list of channels by name")

	return response, nil
}

// ListUserOwnedChannels retrieves the channels associated with a specific user on the specified chain with pagination support.
func (repo *ChannelRepo) ListUserOwnedChannels(
	_ context.Context, _ *http.Request, chain string, address string, withLogo bool,
) (*entities.Response, error) {
	log := utilities.NewLogger("ListUserOwnedChannels")

	var channelIds []string

	// get the user's channels from the user_info table
	query := fmt.Sprintf(
		`SELECT channels FROM %s.%s  WHERE chain = ? AND address = ?`,
		config.GetConfig().DB.Keyspace, consts.UserInfo,
	)

	err := repo.Db.Query(query, chain, address).Scan(&channelIds)
	if err != nil {
		return nil, fmt.Errorf("failed to get user's channel list: %w", err)
	}

	channelModels, err := repo.getChannelsUnfilteredAppIDs(channelIds, chain, withLogo)
	if err != nil {
		return nil, err
	}

	// construct the response object
	response := &entities.Response{
		Data: channelModels,
	}
	log.Info("Successfully retrieved list of owned channels")

	return response, nil
}

// ChannelStatistics retrieves the channel activity statistics for a specific chain and time range.
func (repo *ChannelRepo) ChannelStatistics(
	_ context.Context, _ *http.Request, chain, typeStr, startDate, endDate string,
) ([]entities.ChannelActivity, error) {

	var Iter *gocql.Iter
	log := utilities.NewLogger("ChannelStatistics")
	data := entities.ChannelActivity{}
	var channelData []entities.ChannelActivity
	var date string

	baseQuery := fmt.Sprintf(
		`SELECT event_date, SUM("create") AS created_sum, SUM("delete") AS deleted_sum FROM %s.%s WHERE chain = ? `,
		config.GetConfig().DB.Keyspace, consts.ChannelActivityMetrics,
	)

	// create the query based on the typeStr parameter
	var query string

	switch typeStr {
	case "all":
		query = fmt.Sprintf("%s GROUP BY event_date  ORDER BY event_date ASC", baseQuery)
	case "range":
		query = fmt.Sprintf(
			"%s AND event_date >= '%s' AND event_date <= '%s' GROUP BY event_date  ORDER BY event_date ASC",
			baseQuery, startDate, endDate,
		)
	default:
		log.Errorf("Invalid type parameter")
		return nil, errors.New("invalid type parameter")
	}

	Iter = repo.Db.Query(query, chain).Iter()

	for Iter.Scan(&date, &data.Created, &data.Deleted) {
		data.Date = date
		channelData = append(channelData, data)
	}

	if err := Iter.Close(); err != nil {
		log.WithError(err).Error("Failed to close iterator")
		return nil, err
	}
	return channelData, nil
}

func (repo *ChannelRepo) RetrieveChannelUsers(_ context.Context, chain, appID string) ([]string, error) {
	query := fmt.Sprintf(
		`SELECT users FROM %s.%s WHERE app_id = ? AND chain = ?`,
		config.GetConfig().DB.Keyspace, consts.ChannelUsers,
	)
	iter := repo.Db.Query(query, appID, chain).Iter()

	var users []string
	for iter.Scan(&users) {
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return users, nil
}
func (repo *ChannelRepo) VerifyChannel(ctx context.Context, chain, appID string) error {
	log := utilities.NewLoggerWithFields(
		"repo.VerifyChannel", map[string]interface{}{
			"chain": chain,
			"appID": appID,
		},
	)

	address := cast.ToString(ctx.Value(consts.UserAddress))

	if !config.IsAdminUser(chain, address) {
		return fmt.Errorf("user %s is not admin", address)
	}

	if cache.GetChannelVerifyCache().IsVerified(chain, appID) {
		return fmt.Errorf("channel %s:%s is already verified", chain, appID)
	}

	var (
		name        string
		owner       string
		status      string
		description string
		logo        string
		created     time.Time
	)

	channelQuery := fmt.Sprintf(
		"SELECT name, status, owner, description, logo, created FROM %s.%s where app_id = ? AND chain = ?",
		config.GetConfig().DB.Keyspace, consts.UnverifiedChannelInfo,
	)
	err := repo.Db.Query(channelQuery, appID, chain).Scan(&name, &status, &owner, &description, &logo, &created)
	if err != nil {
		return fmt.Errorf("failed to retrieve channel details: %w", err)
	}

	query := fmt.Sprintf(
		`INSERT INTO %s.%s (chain, name, app_id, description, logo, status, owner, verified, created) VALUES %s`,
		config.GetConfig().DB.Keyspace, consts.VerifiedChannelInfo, utilities.DBMultiValuePlaceholders(9),
	)

	if err = repo.Db.Query(
		query, chain, name, appID, description, logo, status, owner, true, created,
	).Exec(); err != nil {
		log.WithError(err).Error("failed to insert verified channel record")
		return fmt.Errorf("failed to insert channel info: %w", err)
	}
	cache.GetChannelVerifyCache().AddVerified(chain, appID)

	query = fmt.Sprintf(
		`DELETE FROM %s.%s WHERE chain=? AND app_id=? IF EXISTS`,
		config.GetConfig().DB.Keyspace, consts.UnverifiedChannelInfo,
	)

	if err = repo.Db.Query(query, chain, appID).Exec(); err != nil {
		log.WithError(err).Error("Failed to delete channel info from the database")
		return fmt.Errorf("failed to delete channel info: %w", err)
	}

	cache.GetChannelVerifyCache().PopUnverified(chain, appID)

	return nil
}

func (repo *ChannelRepo) unVerifyChannel(ctx context.Context, chain, appID string) error {
	log := utilities.NewLoggerWithFields(
		"repo.unVerifyChannel", map[string]interface{}{
			"chain": chain,
			"appID": appID,
		},
	)

	if !cache.GetChannelVerifyCache().IsVerified(chain, appID) {
		return nil
	}

	var (
		name        string
		owner       string
		status      string
		description string
		logo        string
		created     time.Time
	)

	channelQuery := fmt.Sprintf(
		"SELECT name, status, owner, description, logo, created FROM %s.%s where app_id = ? AND chain = ?",
		config.GetConfig().DB.Keyspace, consts.VerifiedChannelInfo,
	)
	err := repo.Db.Query(channelQuery, appID, chain).Scan(&name, &status, &owner, &description, &logo, &created)
	if err != nil {
		return fmt.Errorf("failed to retrieve channel details: %w", err)
	}

	query := fmt.Sprintf(
		`INSERT INTO %s.%s (chain, name, app_id, description, logo, status, owner, verified, created) VALUES %s`,
		config.GetConfig().DB.Keyspace, consts.UnverifiedChannelInfo, utilities.DBMultiValuePlaceholders(9),
	)

	if err = repo.Db.Query(
		query, chain, name, appID, description, logo, status, owner, false, created,
	).Exec(); err != nil {
		log.WithError(err).Error("failed to insert verified channel record")
		return fmt.Errorf("failed to insert channel info: %w", err)
	}
	cache.GetChannelVerifyCache().AddUnverified(chain, appID)

	query = fmt.Sprintf(
		`DELETE FROM %s.%s WHERE chain=? AND app_id=? IF EXISTS`,
		config.GetConfig().DB.Keyspace, consts.VerifiedChannelInfo,
	)

	if err = repo.Db.Query(query, chain, appID).Exec(); err != nil {
		log.WithError(err).Error("failed to delete channel info from the database")
		return fmt.Errorf("failed to delete channel info: %w", err)
	}

	cache.GetChannelVerifyCache().PopVerified(chain, appID)

	return nil
}
