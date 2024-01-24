package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cast"

	"notiboy/config"
	"notiboy/pkg/cache"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	dbDriver "notiboy/pkg/repo/driver/db"
	"notiboy/utilities"
	"notiboy/utilities/jwt"

	"github.com/gocql/gocql"
	"github.com/sirupsen/logrus"
)

type UserRepo struct {
	db   *gocql.Session
	conf *config.NotiboyConfModel
}

// UserRepoImply represents the interface for the repository that handles user-related operations.
type UserRepoImply interface {
	ProfileUpdate(context.Context, entities.UserInfo) error
	Onboarding(context.Context, entities.OnboardingRequest) error
	Offboarding(context.Context, string, string) error
	GetAllowedMedium(ctx context.Context, address string, chain string) ([]string, error)
	GlobalStatistics(ctx context.Context) ([]entities.GlobalStatistics, error)
	UserStatistics(ctx context.Context, chain, statType, startDate, endDate string) ([]entities.UserActivity, error)
	GetMediumAddress(context.Context, string, string, string) (string, error)
	UpdateMediumMetadata(context.Context, []string, string, string) error
	GetUser(context.Context, entities.UserIdentifier) (*entities.Response, error)
	Login(context.Context, entities.UserIdentifier) (string, error)
	Logout(context.Context, entities.UserIdentifier) error
	GeneratePAT(context.Context, string, string, string) (string, error)
	GetPAT(context.Context, string) ([]entities.PATTokens, error)
	RevokePAT(context.Context, string, string) error
	GetUserSendMetricsForMonth(context.Context, string, string) (int, error)
	StoreFCMToken(ctx context.Context, fcm entities.FCM) error
	GetFCMTokens(ctx context.Context, userIdentifier entities.UserIdentifier) ([]string, error)
}

// NewUserRepo
func NewUserRepo(db *gocql.Session, conf *config.NotiboyConfModel) UserRepoImply {
	return &UserRepo{db: db, conf: conf}
}

func getUserLimit(_ context.Context, membership string) map[string]interface{} {
	tier := consts.MembershipStringToEnum(membership)

	tierInfo := map[string]interface{}{
		"notification_char_count":   consts.NotificationCharacterCount[tier],
		"notification_count":        consts.NotificationCount[tier],
		"channel_count":             consts.ChannelCount[tier],
		"charge":                    consts.MembershipCharge[tier],
		"notification_retention":    consts.NotificationRetentionSecs[tier] / 60 / 60 / 24,
		"notification_max_schedule": consts.NotificationMaxSchedule[tier] / 60 / 60 / 24,
	}

	for statsName, val := range consts.Analytics {
		tierInfo[statsName] = val[tier]
	}

	return tierInfo
}

// GetUser retrieves user information based on the given UserIdentifier.
func (user *UserRepo) GetUser(ctx context.Context, data entities.UserIdentifier) (*entities.Response, error) {
	keyspace := user.conf.DB.Keyspace
	tblUserInfo := fmt.Sprintf("%s.%s", keyspace, consts.UserTable)

	var allowedMediums, supportedMediums, channels, optins []string
	var status, membership string
	var mediumMetadataStr, logo string

	infoQuery := "SELECT channels, optins, membership, logo, status, allowed_mediums, supported_mediums, medium_metadata FROM " + tblUserInfo + " WHERE chain = ? AND address = ?"
	if err := user.db.Query(infoQuery, data.Chain, data.Address).Scan(
		&channels, &optins, &membership, &logo, &status, &allowedMediums, &supportedMediums, &mediumMetadataStr,
	); err != nil {
		return nil, fmt.Errorf("failed to query db: %w", err)
	}

	mediumMetadata := new(entities.MediumMetadata)
	if strings.TrimSpace(mediumMetadataStr) != "" {
		if err := mediumMetadata.Unmarshal(mediumMetadataStr); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", mediumMetadataStr, err)
		}
	}

	userInfo := &entities.UserModel{
		UserIdentifier: entities.UserIdentifier{
			Chain:   data.Chain,
			Address: data.Address,
		},
		SupportedMediums: supportedMediums,
		AllowedMediums:   allowedMediums,
		Membership:       membership,
		Logo:             logo,
		MediumMetadata:   *mediumMetadata,
		Status:           status,
		Channels:         channels,
		Optins:           optins,
		Privileges:       getUserLimit(ctx, membership),
	}

	// construct the response object
	response := entities.Response{
		Data: userInfo,
	}

	return &response, nil
}

// ProfileUpdate updates the user profile information with the provided data.
func (user *UserRepo) ProfileUpdate(_ context.Context, data entities.UserInfo) error {

	var setClause []string
	var args []interface{}
	log := utilities.NewLogger("ProfileUpdate")

	setClause = append(setClause, "allowed_mediums = ?")
	args = append(args, data.AllowedMediums)

	if data.Logo != "" {
		logoConfig := config.GetConfig().Logo
		err := utilities.ValidateImage(
			data.Logo, logoConfig.MaxX, logoConfig.MaxY,
			logoConfig.MaxSize, logoConfig.SupportedTypes,
		)
		if err != nil {
			log.WithError(err).Error("Logo validation failed")
			return fmt.Errorf("logo validation failed: %w", err)
		}
		setClause = append(setClause, "logo = ?")
		args = append(args, data.Logo)
	}

	if len(setClause) == 0 {
		return nil
	}

	whereClause := "address = ? AND chain = ?"
	args = append(args, data.Address, data.Chain)

	query := fmt.Sprintf(
		"UPDATE %s.%s SET %s WHERE %s",
		user.conf.DB.Keyspace, consts.UserInfo, strings.Join(setClause, ", "), whereClause,
	)
	if err := user.db.Query(query, args...).Exec(); err != nil {
		log.WithError(err).Error("Failed to update user info")
		return err
	}

	return nil
}

// Onboarding performs the onboarding process for a user based on the provided OnboardingRequest data.
func (user *UserRepo) Onboarding(_ context.Context, data entities.OnboardingRequest) error {

	now := utilities.TimeNow()
	var status string
	log := utilities.NewLogger("Onboarding")

	var channels []string

	queryProfile := fmt.Sprintf(
		"SELECT status, channels FROM %s.%s WHERE address = ? AND chain = ?",
		user.conf.DB.Keyspace, consts.UserInfo,
	)

	if err := user.db.Query(queryProfile, data.Address, data.Chain).Scan(&status, &channels); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("Failed to check user profile status")
			return fmt.Errorf("failed to check user profile status: %w", err)
		}
	}
	if status == "ACTIVE" {
		log.Warn("User is already onboarded")
		return fmt.Errorf("user is already onboarded")
	}

	mediumMetadata := &entities.MediumMetadata{
		Email:   &entities.EmailMedium{},
		Discord: &entities.DiscordMedium{},
	}
	mediumMetadataStr, err := mediumMetadata.Marshal()
	if err != nil {
		return err
	}

	//user_info insert/update
	query := fmt.Sprintf(
		"INSERT INTO %s.%s (address, chain, allowed_mediums, medium_metadata, membership, created, modified, status) VALUES (?, ?, ?, ?, ?, ? ,?, ?)",
		user.conf.DB.Keyspace, consts.UserInfo,
	)

	if err := user.db.Query(
		query, data.Address, data.Chain, []string{}, mediumMetadataStr, "", now, now, "ACTIVE",
	).Exec(); err != nil {
		return fmt.Errorf("failed to insert user profile: %w", err)
	}

	// user_activity_metrics insert
	insertQuery := fmt.Sprintf(
		"INSERT INTO %s.%s (chain, event_date, event_time, onboard) VALUES (?, ?, ?, ?) USING TTL %d",
		user.conf.DB.Keyspace, consts.UserActivityMetrics, config.GetConfig().TTL.Metrics,
	)

	if err := user.db.Query(insertQuery, data.Chain, utilities.ToDate(now), now, 1).Exec(); err != nil {
		log.WithError(err).Error("Failed to insert user activity metrics")
		return fmt.Errorf("failed to insert user activity metrics: %w", err)
	}

	updateQuery := fmt.Sprintf(
		"UPDATE %s.%s SET users_onboarded = users_onboarded + 1 WHERE chain = ?",
		user.conf.DB.Keyspace, consts.GlobalStatistics,
	)

	if err := user.db.Query(updateQuery, data.Chain).Exec(); err != nil {
		log.WithError(err).Error("Failed to increment users onboarded count")
		return fmt.Errorf("failed to increment users onboarded count: %w", err)
	}

	if len(channels) > 0 {
		verifiedChannels := make([]string, 0)
		unverifiedChannels := make([]string, 0)
		for _, channel := range channels {
			if cache.GetChannelVerifyCache().IsVerified(data.Chain, channel) {
				verifiedChannels = append(verifiedChannels, channel)
			} else {
				unverifiedChannels = append(unverifiedChannels, channel)
			}
		}

		if len(verifiedChannels) > 0 {
			// user's channel status changing to active
			updateChannelQuery := fmt.Sprintf(
				`UPDATE %s.%s SET status = ? WHERE chain = ? AND app_id IN ?`,
				user.conf.DB.Keyspace, consts.VerifiedChannelInfo,
			)

			if err = user.db.Query(updateChannelQuery, "ACTIVE", data.Chain, channels).Exec(); err != nil {
				log.WithError(err).Error("Failed to update channel statuses")
				return fmt.Errorf("failed to update channel statuses: %w", err)
			}
		}

		if len(unverifiedChannels) > 0 {
			// user's channel status changing to active
			updateChannelQuery := fmt.Sprintf(
				`UPDATE %s.%s SET status = ? WHERE chain = ? AND app_id IN ?`,
				user.conf.DB.Keyspace, consts.UnverifiedChannelInfo,
			)

			if err = user.db.Query(updateChannelQuery, "ACTIVE", data.Chain, channels).Exec(); err != nil {
				log.WithError(err).Error("Failed to update channel statuses")
				return fmt.Errorf("failed to update channel statuses: %w", err)
			}
		}

	}

	return nil
}

// Offboarding performs the offboarding process for a user identified by the given address and chain.
func (user *UserRepo) Offboarding(_ context.Context, address string, chain string) error {

	now := utilities.TimeNow()
	log := utilities.NewLogger("Offboarding")

	//checking profile status
	queryProfile := fmt.Sprintf(
		"SELECT status FROM %s.%s WHERE address = ? AND chain = ?",
		user.conf.DB.Keyspace, consts.UserInfo,
	)

	var status string
	if err := user.db.Query(queryProfile, address, chain).Scan(&status); err != nil {
		log.WithError(err).Error("Failed to check user profile status")
		return fmt.Errorf("failed to check user profile status: %w", err)
	}
	// user_activity_metrics insert
	insertQuery := fmt.Sprintf(
		"INSERT INTO %s.%s (chain, event_date, event_time, offboard) VALUES (?, ?, ?, ?) USING TTL %d",
		user.conf.DB.Keyspace, consts.UserActivityMetrics, config.GetConfig().TTL.Metrics,
	)

	if err := user.db.Query(insertQuery, chain, utilities.ToDate(now), now, 1).Exec(); err != nil {
		log.WithError(err).Error("Failed to insert user activity metrics")
		return fmt.Errorf("failed to insert user activity metrics: %w", err)
	}

	//updating profile status
	queryUp := fmt.Sprintf(
		"UPDATE %s.%s SET status = ?, modified = ? WHERE address = ? AND chain = ?",
		user.conf.DB.Keyspace, consts.UserInfo,
	)

	if err := user.db.Query(queryUp, "INACTIVE", time.Now(), address, chain).Exec(); err != nil {
		log.WithError(err).Error("Failed to update user profile status")
		return fmt.Errorf("failed to update user profile status: %w", err)
	}
	// collecting channels of user
	var channels []string
	selectQuery := fmt.Sprintf(
		`SELECT channels FROM %s.%s WHERE address = ? AND chain = ?`,
		user.conf.DB.Keyspace, consts.UserInfo,
	)

	if err := user.db.Query(selectQuery, address, chain).Scan(&channels); err != nil {
		log.WithError(err).Error("Failed to retreive channels")
		return fmt.Errorf("failed to retrieve channels: %w", err)
	}

	if len(channels) > 0 {
		verifiedChannels := make([]string, 0)
		unverifiedChannels := make([]string, 0)

		for _, appID := range channels {
			if cache.GetChannelVerifyCache().IsVerified(chain, appID) {
				verifiedChannels = append(verifiedChannels, appID)
			} else {
				unverifiedChannels = append(unverifiedChannels, appID)
			}
		}

		fn := func(tbl string) error {
			// user's channel status changing to active
			updateChannelQuery := fmt.Sprintf(
				`UPDATE %s.%s SET status = ? WHERE chain = ? AND app_id IN ?`,
				user.conf.DB.Keyspace, tbl,
			)

			if err := user.db.Query(
				updateChannelQuery, consts.STATUS_CHANNEL_ORPHANED, chain, channels,
			).Exec(); err != nil {
				log.WithError(err).Error("Failed to update channel statuses")
				return fmt.Errorf("failed to update channel statuses: %w", err)
			}

			return nil
		}

		if len(verifiedChannels) > 0 {
			if err := fn(consts.VerifiedChannelInfo); err != nil {
				return err
			}
		}
		if len(unverifiedChannels) > 0 {
			if err := fn(consts.UnverifiedChannelInfo); err != nil {
				return err
			}
		}

		log.Infof("Channel statuses updated for user: %s", address)
	}

	query := fmt.Sprintf(
		`DELETE FROM %s.%s WHERE address = ? AND chain = ? IF EXISTS`,
		user.conf.DB.Keyspace, consts.NotificationReadStatus,
	)

	if err := user.db.Query(query, address, chain).Exec(); err != nil {
		log.WithError(err).Error("failed to delete NotificationReadStatus")
		return fmt.Errorf("failed to delete NotificationReadStatus: %w", err)
	}

	return nil
}

// GetAllowedMedium retrieves the list of allowed mediums for a specific user and chain from the user repository.
func (user *UserRepo) GetAllowedMedium(_ context.Context, address string, chain string) ([]string, error) {

	log := utilities.NewLogger("GetAllowedMedium")
	query := fmt.Sprintf(
		`SELECT allowed_mediums FROM %s.%s WHERE address = ? AND chain = ?`,
		user.conf.DB.Keyspace, consts.UserInfo,
	)

	var allowedMediums []string
	if err := user.db.Query(query, address, chain).Scan(&allowedMediums); err != nil {
		log.Error("Failed to retrieve allowed mediums", err)
		return nil, err
	}
	log.Info("Allowed mediums retrieved successfully", "AllowedMediums")
	return allowedMediums, nil
}

// GlobalStatistics retrieves the global statistics from the user repository.
func (user *UserRepo) GlobalStatistics(_ context.Context) ([]entities.GlobalStatistics, error) {

	var stats entities.GlobalStatistics
	log := utilities.NewLogger("GlobalStatistics")
	queryUser := fmt.Sprintf(
		`SELECT users_onboarded, notifications_sent FROM %s.%s`,
		user.conf.DB.Keyspace, consts.GlobalStatistics,
	)

	var totUserCount, userCount, totNotifCount, notifCount int64

	iter := user.db.Query(queryUser).Iter()
	for iter.Scan(&userCount, &notifCount) {
		totUserCount += userCount
		totNotifCount += notifCount
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}

	log.Infof("Retrieved user count: %d, notification count: %d", userCount, notifCount)

	stats.Users = int(totUserCount)
	stats.NotificationsSent = int(totNotifCount)

	queryChannel := fmt.Sprintf(
		`SELECT SUM("create"-"delete") FROM %s.%s`,
		user.conf.DB.Keyspace, consts.ChannelActivityMetrics,
	)

	var channelsCount int64
	if err := user.db.Query(queryChannel).Scan(&channelsCount); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			channelsCount = 0
		} else {
			log.Errorf("Failed to retrieve channel count: %v", err)
			return nil, err
		}
	}
	log.Infof("Retrieved channel count: %d", channelsCount)

	stats.Channels = int(channelsCount)

	globalStatics := []entities.GlobalStatistics{stats}

	return globalStatics, nil

}

// UserStatistics retrieves user activity statistics for the given chain.
func (user *UserRepo) UserStatistics(
	_ context.Context, chain string, statType string, startDate string, endDate string,
) ([]entities.UserActivity, error) {
	data := entities.UserActivity{}
	var userData []entities.UserActivity
	var date string
	log := utilities.NewLogger("UserStatistics")

	query := fmt.Sprintf(
		`SELECT event_date, SUM("onboard") AS onboard, SUM("offboard") AS offboard FROM %s.%s WHERE chain = ? `,
		user.conf.DB.Keyspace, consts.UserActivityMetrics,
	)
	if statType == "range" {
		query = fmt.Sprintf(
			"%s AND event_date >= '%s' AND event_date <= '%s' GROUP BY event_date ORDER BY event_date ASC", query,
			startDate, endDate,
		)
	} else {
		// for fetching all data
		query = fmt.Sprintf("%s GROUP BY event_date ORDER BY event_date ASC", query)
	}

	iter := user.db.Query(query, chain).Iter()
	for iter.Scan(&date, &data.Onboard, &data.Offboard) {
		// removing an outlier that makes graph ugly
		// 297 auto-onboards on platform launch day
		if date == "2023-05-29" {
			continue
		}
		data.Date = date
		userData = append(userData, data)
	}

	if err := iter.Close(); err != nil {
		log.WithError(err).Error("Failed to retrieve user activity data:")
		return nil, err
	}
	return userData, nil
}

// GetMediumAddress retrieves the medium address for a specific user, medium, and chain from the user repository.
func (user *UserRepo) GetMediumAddress(_ context.Context, address, medium, chain string) (string, error) {

	userInfo := fmt.Sprintf("%s.%s", user.conf.DB.Keyspace, consts.UserInfo)

	query := fmt.Sprintf(
		`SELECT medium_metadata
		FROM %s WHERE address = ? AND chain = ?`,
		userInfo,
	)

	var mediumMetadataStr string
	if err := user.db.Query(query, address, chain).Scan(&mediumMetadataStr); err != nil {
		if err.Error() == "not found" {
		} else {
			return "", fmt.Errorf("failed to check medium metadata: %w", err)
		}
	}

	mediumMetadata := new(entities.MediumMetadata)
	err := mediumMetadata.Unmarshal(mediumMetadataStr)
	if err != nil {
		return "", err
	}

	switch medium {
	case "email":
		if mediumMetadata.Email.Verified {
			return mediumMetadata.Email.ID, nil
		}
		return "", errors.New("medium address not verified")
	case "discord":
		if mediumMetadata.Discord.Verified {
			return mediumMetadata.Discord.ID, nil
		}
		return "", errors.New("medium address not verified")

	default:
		return "", errors.New("please choose valid medium")
	}
}

// UpdateMediumMetadata updates the medium metadata for multiple users in the user repository.
func (user *UserRepo) UpdateMediumMetadata(ctx context.Context, users []string, chain, medium string) error {
	tblNotificationChanMetrics := user.conf.DB.Keyspace + ".user_info"

	batch := user.db.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	for _, user := range users {
		batch.Query(
			fmt.Sprintf(
				`
		UPDATE %s
		SET 
		allowed_mediums = allowed_mediums - {'%s'},
		medium_metadata = medium_metadata - {'%s'}
		WHERE address = ? AND chain=?`, tblNotificationChanMetrics, medium, medium,
			), user, chain,
		)
	}

	err := user.db.ExecuteBatch(batch)
	if err != nil {
		return err
	}
	return nil
}

// Login performs user login and generates a JWT token for the specified address and chain.
// If auto-onboarding is enabled, it automatically onboards the user if they are not already onboarded.
// The JWT token is inserted into the login table with an expiration time.
// The generated JWT token is returned.
func (user *UserRepo) Login(ctx context.Context, request entities.UserIdentifier) (string, error) {
	log := utilities.NewLogger("Login").WithFields(
		logrus.Fields{
			"chain":   request.Chain,
			"address": request.Address,
		},
	)

	if config.GetConfig().AutoOnboardUsers {
		onboarded, err := dbDriver.IsUserOnboarded(ctx, request.Chain, request.Address)
		if err != nil {
			log.WithError(err).Error("user onboard check failed")
			return "", fmt.Errorf("user onboard check failed: %w", err)
		}

		if !onboarded {
			log.Infof("Auto-onboarding user %s", request.Address)
			err = user.Onboarding(
				ctx, entities.OnboardingRequest{
					UserIdentifier: entities.UserIdentifier{
						Chain:   request.Chain,
						Address: request.Address,
					},
				},
			)
			if err != nil {
				log.WithError(err).Error("user auto-onboarding failed")
				return "", fmt.Errorf("user auto-onboarding failed for %s: %w", request.Address, err)
			}
		}
	}

	loginInfoTable := fmt.Sprintf("%s.%s", user.conf.DB.Keyspace, consts.LoginTable)

	ttl := cast.ToDuration(user.conf.LoginTokenExpiry)
	jwtToken, tokenExpiry, err := jwt.GenerateJWT(request.Address, request.Chain, "", "", ttl)
	if err != nil {
		return "", fmt.Errorf("token generation failed: %w", err)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (address, chain, jwt, created) VALUES (?, ?, ?, ?) USING TTL ?",
		loginInfoTable,
	)

	if err = user.db.Query(
		query, request.Address, request.Chain, jwtToken, time.Now(), tokenExpiry,
	).Exec(); err != nil {
		log.WithError(err).Error("Failed to insert login token")
		return "", fmt.Errorf("failed to insert login token: %w", err)
	}

	log.Info("Login succeeded")

	return jwtToken, nil
}

// Logout removes the specified JWT token from the login table, effectively logging out the user.
// The JWT token is matched against the address and chain in the login table, and if found, it is deleted.
func (user *UserRepo) Logout(ctx context.Context, req entities.UserIdentifier) error {

	log := utilities.NewLogger("Logout")
	loginInfoTable := fmt.Sprintf("%s.%s", user.conf.DB.Keyspace, consts.LoginTable)

	queryProfile := fmt.Sprintf("DELETE FROM %s WHERE address = ? AND chain = ? AND jwt = ?", loginInfoTable)

	if err := user.db.Query(queryProfile, req.Address, req.Chain, ctx.Value(consts.UserToken)).Exec(); err != nil {
		log.WithError(err).Error("failed to logout")
		return fmt.Errorf("failed to log user out: %w", err)
	}
	return nil
}

func (user *UserRepo) GeneratePAT(ctx context.Context, name, kind, description string) (string, error) {
	chain := cast.ToString(ctx.Value(consts.UserChain))
	address := cast.ToString(ctx.Value(consts.UserAddress))

	log := utilities.NewLogger("GeneratePAT").WithFields(
		logrus.Fields{
			"chain":   chain,
			"address": address,
		},
	)

	patTable := fmt.Sprintf("%s.%s", user.conf.DB.Keyspace, consts.PATInfo)

	if kind == "mobile" {
		var jwtToken string
		query := fmt.Sprintf(
			"SELECT jwt FROM %s WHERE address = ? AND chain = ? AND kind = ?",
			patTable,
		)
		err := user.db.Query(query, address, chain, kind).Scan(&jwtToken)
		if err != nil {
			if !errors.Is(err, gocql.ErrNotFound) {
				return "", fmt.Errorf("failed to get token: %w", err)
			}
		} else {
			return jwtToken, nil
		}
	}

	ttl := cast.ToDuration(user.conf.TTL.PAToken)
	id := uuid.New().String()

	jwtToken, tokenExpiry, err := jwt.GenerateJWT(address, chain, kind, id, ttl)
	if err != nil {
		return "", fmt.Errorf("token generation failed: %w", err)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (address, chain, uuid, name, jwt, created, description, kind) VALUES %s USING TTL ?",
		patTable, utilities.DBMultiValuePlaceholders(8),
	)

	if err = user.db.Query(
		query, address, chain, id, name, jwtToken, time.Now(), description, kind, tokenExpiry,
	).Exec(); err != nil {
		log.WithError(err).Error("failed to insert pa token")
		return "", fmt.Errorf("failed to insert pa token: %w", err)
	}

	log.Info("Personal access token generated")

	return jwtToken, nil
}

func (user *UserRepo) GetPAT(ctx context.Context, kind string) ([]entities.PATTokens, error) {
	chain := cast.ToString(ctx.Value(consts.UserChain))
	address := cast.ToString(ctx.Value(consts.UserAddress))

	log := utilities.NewLogger("GetPAT").WithFields(
		logrus.Fields{
			"chain":   chain,
			"address": address,
		},
	)

	var (
		name        string
		id          string
		description string
		created     time.Time
	)

	if kind != "normal" && kind != "mobile" {
		return nil, fmt.Errorf("invalid PAT kind %s received", kind)
	}

	data := make([]entities.PATTokens, 0)

	patTable := fmt.Sprintf("%s.%s", user.conf.DB.Keyspace, consts.PATInfo)
	query := fmt.Sprintf(
		"SELECT name, uuid, created, description FROM %s WHERE address = ? AND chain = ? AND kind = ?",
		patTable,
	)

	iter := user.db.Query(query, address, chain, kind).Iter()
	for iter.Scan(&name, &id, &created, &description) {
		data = append(
			data, entities.PATTokens{
				Name:        name,
				UUID:        id,
				Created:     created,
				Kind:        kind,
				Description: description,
			},
		)
	}
	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to read pa tokens")
			return data, fmt.Errorf("failed to read pa tokens: %w", err)
		}
	}

	log.Info("Personal access tokens retrieved")

	return data, nil
}

func (user *UserRepo) RevokePAT(ctx context.Context, id, kind string) error {
	chain := cast.ToString(ctx.Value(consts.UserChain))
	address := cast.ToString(ctx.Value(consts.UserAddress))

	log := utilities.NewLogger("RevokePAT").WithFields(
		logrus.Fields{
			"chain":   chain,
			"address": address,
		},
	)

	patTable := fmt.Sprintf("%s.%s", user.conf.DB.Keyspace, consts.PATInfo)

	queryProfile := fmt.Sprintf("DELETE FROM %s WHERE address = ? AND chain = ? AND kind = ? AND uuid = ?", patTable)
	if err := user.db.Query(queryProfile, address, chain, kind, id).Exec(); err != nil {
		log.WithError(err).Error("failed to revoke")
		return fmt.Errorf("failed to revoke: %w", err)
	}
	return nil
}

func (user *UserRepo) GetUserSendMetricsForMonth(_ context.Context, chain, address string) (int, error) {
	log := utilities.NewLogger("GetUserSendMetricsForMonth").WithFields(
		logrus.Fields{
			"chain":   chain,
			"address": address,
		},
	)

	var totalSent int

	tbl := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationTotalSendPerUserMetrics)
	query := fmt.Sprintf(
		`SELECT SUM("sent") AS sent FROM %s WHERE chain = ? AND address = ? AND event_date >= '%s' AND event_date <= '%s' GROUP BY event_date`,
		tbl, utilities.ToDate(utilities.BeginningOfMonth(time.Now())),
		utilities.ToDate(utilities.EndOfMonth(time.Now())),
	)

	err := user.db.Query(query, chain, address).Scan(&totalSent)
	if err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to get total sent")
			return totalSent, fmt.Errorf("failed to get total sent")
		}
	}

	return totalSent, nil
}

func (user *UserRepo) StoreFCMToken(_ context.Context, fcm entities.FCM) error {
	chain := fcm.Chain
	address := fcm.Address

	log := utilities.NewLogger("StoreFCMToken").WithFields(
		logrus.Fields{
			"chain":   chain,
			"address": address,
		},
	)

	fcmTable := fmt.Sprintf("%s.%s", user.conf.DB.Keyspace, consts.FcmTable)
	ttl := user.conf.TTL.FCM

	query := fmt.Sprintf(
		"INSERT INTO %s (chain, address, device_id, updated) VALUES (?, ?, ?, ?) USING TTL %d",
		fcmTable, ttl,
	)

	if err := user.db.Query(query, chain, address, fcm.DeviceID, utilities.TimeNow()).Exec(); err != nil {
		log.WithError(err).Error("failed to insert fcm device id")
		return fmt.Errorf("failed to insert fcm device id: %w", err)
	}

	return nil
}

func (user *UserRepo) GetFCMTokens(_ context.Context, userIdentifier entities.UserIdentifier) ([]string, error) {
	chain := userIdentifier.Chain
	address := userIdentifier.Address

	log := utilities.NewLogger("GetFCMTokens").WithFields(
		logrus.Fields{
			"chain":   chain,
			"address": address,
		},
	)

	fcmTable := fmt.Sprintf("%s.%s", user.conf.DB.Keyspace, consts.FcmTable)
	query := fmt.Sprintf(`SELECT device_id FROM %s WHERE chain = ? AND address = ?`, fcmTable)
	iter := user.db.Query(query, chain, address).Iter()

	var deviceID string
	deviceIDs := make([]string, 0)

	for iter.Scan(&deviceID) {
		deviceIDs = append(deviceIDs, deviceID)
	}

	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to read fcm device ids")
			return deviceIDs, fmt.Errorf("failed to read fcm device ids: %w", err)
		}
	}

	return deviceIDs, nil
}
