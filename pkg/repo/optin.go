package repo

import (
	"context"
	"errors"
	"fmt"

	"notiboy/config"
	"notiboy/pkg/cache"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/utilities"

	"github.com/gocql/gocql"
)

type OptinRepo struct {
	db   *gocql.Session
	conf *config.NotiboyConfModel
	repo ChannelRepoImpl
}

// OptinRepoImply represents the interface for the repository that handles opt-in and opt-out operations.
type OptinRepoImply interface {
	Optin(context.Context, string, string, string) error
	Optout(context.Context, string, string, string) error
	OptinoutStatistics(
		ctx context.Context, chain, appId, statType, startDate, endDate string,
	) (*entities.ChannelOptInOutStats, error)
	VerifyUserOptin(ctx context.Context, chain, appId, userId string) (string, error)
	OptinUsers(ctx context.Context, chain, appId string) ([]string, error)
}

// NewUserRepo
func NewOptinRepo(db *gocql.Session, conf *config.NotiboyConfModel, repo ChannelRepoImpl) OptinRepoImply {
	return &OptinRepo{db: db, conf: conf, repo: repo}
}

// Optin adds a user to a channel by updating the channel user and user info in the database.
func (user *OptinRepo) Optin(_ context.Context, chain, appId, userAddr string) error {
	now := utilities.TimeNow()

	// Get the status of the channel
	var channelStat string
	log := utilities.NewLogger("Optin")

	tbl := consts.UnverifiedChannelInfo
	if cache.GetChannelVerifyCache().IsVerified(chain, appId) {
		tbl = consts.VerifiedChannelInfo
	}

	query := fmt.Sprintf(
		`SELECT status FROM %s.%s WHERE chain = ? AND app_id = ?`,
		user.conf.DB.Keyspace, tbl,
	)

	if err := user.db.Query(query, chain, appId).Scan(&channelStat); err != nil {
		log.WithError(err).Error("Failed to execute channel status query")
		return fmt.Errorf("channel existing status:%w", err)

	}

	// Check if the channel status is active
	if channelStat != "ACTIVE" {
		log.Errorf("Invalid channel status: %s", channelStat)
		return errors.New("invalid channel")
	}

	var appIDFromDB string
	query = fmt.Sprintf(
		"SELECT optins[?] FROM %s.%s WHERE address = ? AND chain = ?",
		user.conf.DB.Keyspace, consts.UserTable,
	)

	if err := user.db.Query(query, appId, userAddr, chain).Scan(&appIDFromDB); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("Failed to check user's opt-in status for the channel")
			return fmt.Errorf("failed to query if user already opted in to channel: %w", err)
		}
	}

	if appId == appIDFromDB {
		return errors.New("you have already opted in to the channel")
	}

	// Update the channel user
	query = fmt.Sprintf(
		`UPDATE %s.%s SET users = users + ? WHERE app_id = ? and chain = ?`,
		user.conf.DB.Keyspace, consts.ChannelUsers,
	)

	if err := user.db.Query(query, []string{userAddr}, appId, chain).Exec(); err != nil {
		log.WithError(err).Error("Failed to update channel users")
		return err
	}

	// Update the user info
	query = fmt.Sprintf(
		`UPDATE %s.%s SET optins = optins + ? WHERE address = ? AND chain = ?`,
		user.conf.DB.Keyspace, consts.UserInfo,
	)
	if err := user.db.Query(query, []string{appId}, userAddr, chain).Exec(); err != nil {
		log.WithError(err).Error("Failed to update user opt-ins")
		return err
	}

	// Insert a row into channel_traction_metrics with 1 for optin
	query = fmt.Sprintf(
		`INSERT INTO %s.%s (chain, channel, optin, optout, event_date, event_time)
	VALUES (?, ?, 1, 0, ?, ?) USING TTL %d`,
		user.conf.DB.Keyspace, consts.ChannelTractionMetrics, config.GetConfig().TTL.Metrics,
	)

	if err := user.db.Query(query, chain, appId, utilities.ToDate(now), now).Exec(); err != nil {
		log.WithError(err).Error("Failed to record channel opt-in traction metrics")
		return err
	}

	return nil
}

// Optout removes a user from a channel by updating the channel user and user info in the database.
func (user *OptinRepo) Optout(_ context.Context, chain, appId, userAddr string) error {
	log := utilities.NewLoggerWithFields(
		"optout",
		map[string]interface{}{
			"chain":   chain,
			"appID":   appId,
			"address": userAddr,
		},
	)

	now := utilities.TimeNow()

	tbl := consts.UnverifiedChannelInfo
	if cache.GetChannelVerifyCache().IsVerified(chain, appId) {
		tbl = consts.VerifiedChannelInfo
	}

	// Get the status of the channel
	var channelStat string
	query := fmt.Sprintf(
		`SELECT status FROM %s.%s WHERE chain = ? AND app_id = ?`,
		user.conf.DB.Keyspace, tbl,
	)
	if err := user.db.Query(query, chain, appId).Scan(&channelStat); err != nil {
		log.WithError(err).Error("failed to query channel status")
		return fmt.Errorf("channel opt-out authorisation:%w", err)
	}

	// Check if the channel status is active
	if channelStat != "ACTIVE" {
		log.Error("inactive channel")
		return errors.New("inactive channel")
	}

	var appIDFromDB string
	query = fmt.Sprintf(
		"SELECT optins[?] FROM %s.%s WHERE address = ? AND chain = ?",
		user.conf.DB.Keyspace, consts.UserTable,
	)
	if err := user.db.Query(query, appId, userAddr, chain).Scan(&appIDFromDB); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to query if user already opted out of channel")
			return fmt.Errorf("failed to query if user already opted out of channel: %w", err)
		}
	}

	if appId != appIDFromDB {
		log.Debug("Not opted into channel")
		return errors.New("you are not opted in to the channel")
	}

	// Update the channel user
	query = fmt.Sprintf(
		`UPDATE %s.%s SET users = users - ? WHERE app_id = ? and chain = ?`,
		user.conf.DB.Keyspace, consts.ChannelUsers,
	)
	if err := user.db.Query(query, []string{userAddr}, appId, chain).Exec(); err != nil {
		log.WithError(err).Error("failed to update channel users list")
		return err
	}

	// Update the user info
	query = fmt.Sprintf(
		`UPDATE %s.%s SET optins = optins - ? WHERE address = ? AND chain = ?`,
		user.conf.DB.Keyspace, consts.UserInfo,
	)
	if err := user.db.Query(query, []string{appId}, userAddr, chain).Exec(); err != nil {
		log.WithError(err).Error("failed to update user optins list")
		return err
	}

	// Insert a row into channel_traction_metrics with 0 for optout
	query = fmt.Sprintf(
		`INSERT INTO %s.%s (chain, channel, optin, optout, event_date, event_time)
	VALUES (?, ?, 0, 1, ?, ?) USING TTL %d`,
		user.conf.DB.Keyspace, consts.ChannelTractionMetrics, config.GetConfig().TTL.Metrics,
	)
	if err := user.db.Query(query, chain, appId, utilities.ToDate(now), now).Exec(); err != nil {
		log.WithError(err).Error("failed to insert metrics")
		return err
	}

	return nil
}

// OptinoutStatistics retrieves opt-in and opt-out statistics for a specific channel within a given time range or overall.
func (user *OptinRepo) OptinoutStatistics(
	_ context.Context, chain, appId, statType, startDate, endDate string,
) (*entities.ChannelOptInOutStats, error) {

	data := entities.OptInOut{}
	var userData = make([]entities.OptInOut, 0)
	var date string
	log := utilities.NewLogger("OptinoutStatistics")
	// Get the status of the channel
	tbl := consts.UnverifiedChannelInfo
	if cache.GetChannelVerifyCache().IsVerified(chain, appId) {
		tbl = consts.VerifiedChannelInfo
	}

	var appExist int
	query := fmt.Sprintf(
		`SELECT COUNT(app_id) FROM %s.%s WHERE chain = ? AND app_id = ?`,
		user.conf.DB.Keyspace, tbl,
	)
	if err := user.db.Query(query, chain, appId).Scan(&appExist); err != nil {
		log.WithError(err).Error("failed to query channel status")
		return nil, fmt.Errorf("channel opt-out authorisation:%w", err)
	}
	if appExist == 0 {
		log.WithError(errors.New("invalid channel"))
		return nil, errors.New("invalid channel")
	}

	query = fmt.Sprintf(
		`SELECT event_date, SUM("optin") AS optin, SUM("optout") AS optout FROM %s.%s WHERE chain = ? AND channel = ? `,
		user.conf.DB.Keyspace, consts.ChannelTractionMetrics,
	)
	if statType == "range" {
		query = fmt.Sprintf(
			"%s AND event_date >= '%s' AND event_date <= '%s' GROUP BY event_date ORDER BY event_date ASC", query,
			startDate, endDate,
		)
	} else {
		query = fmt.Sprintf("%s GROUP BY event_date ORDER BY event_date ASC", query)
	}
	iter := user.db.Query(query, chain, appId).Iter()
	for iter.Scan(&date, &data.Optin, &data.Optout) {
		data.Date = date
		userData = append(userData, data)
	}
	if err := iter.Close(); err != nil {
		log.WithError(err).Error("Failed to close iterator for channel traction metrics")
		return nil, err
	}

	userList := fmt.Sprintf(
		`SELECT users FROM %s.%s WHERE chain = ? AND app_id = ?`,
		config.GetConfig().DB.Keyspace, consts.ChannelUsers,
	)
	var users []string

	err := user.db.Query(userList, chain, appId).Scan(&users)
	if err != nil {
		return nil, fmt.Errorf("failed to get user list for channel")
	}

	return &entities.ChannelOptInOutStats{
		OptInOut:   userData,
		TotalUsers: int64(len(users)),
	}, nil
}

// VerifyUserOptin verifies if a user is opted in to a specific channel.
func (user *OptinRepo) VerifyUserOptin(_ context.Context, chain, appId, userId string) (string, error) {

	var appIDFromDB string
	log := utilities.NewLogger("VerifyUserOptin")
	query := fmt.Sprintf(
		"SELECT optins[?] FROM %s.%s WHERE address = ? AND chain = ?",
		user.conf.DB.Keyspace, consts.UserTable,
	)
	if err := user.db.Query(query, appId, userId, chain).Scan(&appIDFromDB); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to query if user is opted in to channel")
			return "", fmt.Errorf("failed to query if user is opted in to channel: %w", err)
		}
	}

	if appId != appIDFromDB {
		log.Debug("Not opted into channel")
		return "", errors.New("you are not opted in to the channel")
	}

	return appId, nil
}

// OptinUsers retrieves the list of users who have opted in to a specific channel.
func (user *OptinRepo) OptinUsers(_ context.Context, chain, appId string) ([]string, error) {
	var users []string
	log := utilities.NewLogger("OptinUsers")
	// retreving the optin users list
	optInUsersQuery := fmt.Sprintf(
		`SELECT users FROM %s.%s WHERE chain = ? AND app_id = ?`,
		user.conf.DB.Keyspace, consts.ChannelUsers,
	)

	if err := user.db.Query(optInUsersQuery, chain, appId).Scan(&users); err != nil {
		log.WithError(err).Error("Failed to query opt-in users for the channel")
		return nil, err
	}

	return users, nil
}
