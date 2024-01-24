package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/utilities"

	"github.com/gocql/gocql"
)

type NotificationRepo struct {
	db   *gocql.Session
	conf *config.NotiboyConfModel
}

// NotificationRepoImply is an interface that defines the contract for a notification repository implementation.
type NotificationRepoImply interface {
	InsertNotificationInfo(context.Context, *entities.Notification) error
	InsertScheduledNotificationInfo(context.Context, *entities.ScheduleNotificationRequest) error
	GetScheduledNotificationInfo(context.Context, string, time.Time) ([]entities.NotificationRequest, error)
	DeleteScheduledNotificationInfo(context.Context, string, string, time.Time) error
	UpdateScheduledNotificationInfo(context.Context, *entities.ScheduleNotificationRequest) error
	GetScheduledNotificationInfoBySender(context.Context, string, string) ([]entities.NotificationRequest, error)
	InsertChannelSendMetrics(context.Context, entities.NotificationRequest, time.Time, int) error
	InsertUserSendMetrics(context.Context, entities.NotificationRequest, time.Time, int) error
	InsertNotificationChannelCounter(context.Context, entities.NotificationRequest, int) error
	ListOfReadUsers(context.Context, entities.NotificationRequest) ([]string, error)
	GetNotificationInfo(context.Context, entities.RequestNotification, int, []byte) (
		[]entities.ReadNotification, []byte, error,
	)
	InsertGlobalStats(context.Context, entities.NotificationRequest, int) error
	NotificationReachCount(context.Context, string, int, []byte) ([]entities.NotificationReach, []byte, error)
	InsertNotificationSentCountForReach(context.Context, entities.NotificationRequest, int) error
}

func NewNotificationRepo(db *gocql.Session, conf *config.NotiboyConfModel) NotificationRepoImply {
	return &NotificationRepo{db: db, conf: conf}
}

func (repo *NotificationRepo) DeleteScheduledNotificationInfo(
	_ context.Context, chain string, sender string, schedule time.Time,
) error {
	log := utilities.NewLoggerWithFields(
		"DeleteScheduledNotificationInfo", map[string]interface{}{
			"sender":   sender,
			"chain":    chain,
			"schedule": schedule,
		},
	)

	tblNotificationInfo := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.ScheduledNotificationInfo)
	query := fmt.Sprintf(`DELETE FROM %s WHERE chain = ? AND schedule = ? AND sender = ?`, tblNotificationInfo)

	if err := repo.db.Query(query, chain, schedule, sender).Exec(); err != nil {
		return fmt.Errorf("failed to delete scheduled notification: %w", err)
	}

	log.Info("Deleted scheduled notification")

	return nil
}

func (repo *NotificationRepo) UpdateScheduledNotificationInfo(
	_ context.Context, request *entities.ScheduleNotificationRequest,
) error {
	log := utilities.NewLoggerWithFields(
		"InsertScheduledNotificationInfo", map[string]interface{}{
			"channel": request.Channel,
			"chain":   request.Chain,
			"type":    request.Type,
		},
	)

	var (
		set    []string
		setVal []interface{}
	)

	if len(request.Receivers) != 0 {
		set = append(set, "receivers = ?")
		setVal = append(setVal, request.Receivers)
	}
	if request.Message != "" {
		set = append(set, "message = ?")
		setVal = append(setVal, request.Message)
	}
	if request.Link != "" {
		set = append(set, "link = ?")
		setVal = append(setVal, request.Link)
	}
	if request.Type != "" {
		set = append(set, "type = ?")
		setVal = append(setVal, request.Type)
	}

	setVal = append(setVal, request.Chain, request.Schedule, request.Sender)

	tblNotificationInfo := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.ScheduledNotificationInfo)
	query := fmt.Sprintf(
		`UPDATE %s SET %s WHERE chain = ? AND schedule = ? AND sender = ? IF EXISTS`,
		tblNotificationInfo, strings.Join(set, ","),
	)

	if err := repo.db.Query(query, setVal...).Exec(); err != nil {
		log.WithError(err).Error("failed to execute query for updating schedule notification")
		return err
	}

	log.Debug("Schedule notification set")

	return nil
}

func (repo *NotificationRepo) InsertScheduledNotificationInfo(
	_ context.Context, request *entities.ScheduleNotificationRequest,
) error {
	log := utilities.NewLoggerWithFields(
		"InsertScheduledNotificationInfo", map[string]interface{}{
			"channel": request.Channel,
			"chain":   request.Chain,
			"type":    request.Type,
		},
	)

	//increase ttl by 12 hours so that there won't be a race condition between periodic  daemon picking this up and the entry
	//getting evicted from table
	ttl := request.TTL + 43200

	tblNotificationInfo := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.ScheduledNotificationInfo)
	query := fmt.Sprintf(
		`INSERT INTO %s
	(chain, sender, receivers, message, link, app_id, type, schedule)
	 VALUES %s USING TTL %d`, tblNotificationInfo, utilities.DBMultiValuePlaceholders(8), ttl,
	)

	params := []interface{}{
		request.Chain,
		request.Sender,
		request.Receivers,
		request.Message,
		request.Link,
		request.Channel,
		request.Type,
		request.Schedule,
	}

	if err := repo.db.Query(query, params...).Exec(); err != nil {
		log.WithError(err).Error("failed to execute query for inserting schedule notification")
		return err
	}

	log.Debug("Schedule notification inserted")

	return nil
}

func (repo *NotificationRepo) GetScheduledNotificationInfo(
	_ context.Context, chain string, timeUntil time.Time,
) ([]entities.NotificationRequest, error) {
	log := utilities.NewLoggerWithFields(
		"GetScheduledNotificationInfo", map[string]interface{}{
			"time":  timeUntil,
			"chain": chain,
		},
	)

	tblNotificationInfo := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.ScheduledNotificationInfo)
	query := fmt.Sprintf(
		`SELECT sender, receivers, message, link, app_id, type, schedule FROM %s WHERE chain = ? AND schedule <= ?`,
		tblNotificationInfo,
	)

	iter := repo.db.Query(query, chain, timeUntil).Iter()

	var (
		sender    string
		receivers []string
		message   string
		link      string
		channel   string
		kind      string
		schedule  time.Time

		requests []entities.NotificationRequest
	)

	for iter.Scan(&sender, &receivers, &message, &link, &channel, &kind, &schedule) {
		requests = append(
			requests, entities.NotificationRequest{
				Chain:     chain,
				Sender:    sender,
				Receivers: receivers,
				Message:   message,
				Link:      link,
				Channel:   channel,
				Type:      kind,
				Schedule:  schedule,
			},
		)
	}

	if err := iter.Close(); err != nil {
		log.WithError(err).Error("failed to retrieve scheduled notifications")
		return nil, err
	}

	if len(requests) > 0 {
		log.Debugf("Schedule notification retrieved, count: %d", len(requests))
	}

	return requests, nil
}

func (repo *NotificationRepo) GetScheduledNotificationInfoBySender(
	_ context.Context, chain, sender string,
) ([]entities.NotificationRequest, error) {
	log := utilities.NewLoggerWithFields(
		"GetScheduledNotificationInfoBySender", map[string]interface{}{
			"sender": sender,
			"chain":  chain,
		},
	)

	tblNotificationInfo := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.ScheduledNotificationInfo)
	query := fmt.Sprintf(
		`SELECT sender, receivers, message, link, app_id, type, schedule FROM %s WHERE chain = ?`,
		tblNotificationInfo,
	)

	iter := repo.db.Query(query, chain).Iter()

	var (
		receivers []string
		dbSender  string
		message   string
		link      string
		channel   string
		kind      string
		schedule  time.Time

		requests []entities.NotificationRequest
	)

	for iter.Scan(&dbSender, &receivers, &message, &link, &channel, &kind, &schedule) {
		if sender != dbSender {
			continue
		}

		requests = append(
			requests, entities.NotificationRequest{
				Sender:    sender,
				Receivers: receivers,
				Message:   message,
				Link:      link,
				Channel:   channel,
				Type:      kind,
				Schedule:  schedule,
			},
		)
	}

	if err := iter.Close(); err != nil {
		log.WithError(err).Error("failed to retrieve scheduled notifications for sender")
		return nil, err
	}

	log.Debugf("Schedule notification retrieved for sender, count: %d", len(requests))

	return requests, nil
}

// InsertNotificationInfo inserts the notification information into the notification repository.
func (repo *NotificationRepo) InsertNotificationInfo(_ context.Context, request *entities.Notification) error {

	log := utilities.NewLoggerWithFields(
		"InsertNotificationInfo", map[string]interface{}{
			"channel":  request.Channel,
			"chain":    request.Chain,
			"receiver": request.Receiver,
		},
	)

	mediumPublished, err := json.Marshal(request.MediumPublished)
	if err != nil {
		log.WithError(err).Error("failed to marshal medium published object")
		return err
	}

	tblNotificationInfo := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationInfo)
	query := fmt.Sprintf(
		`INSERT INTO %s
	(chain, receiver, uuid, app_id, channel_name, created_time, hash, link, medium_published, message, seen, type, updated_time, logo, verified)
	 VALUES %s USING TTL %d`, tblNotificationInfo, utilities.DBMultiValuePlaceholders(15), request.TTL,
	)

	params := []interface{}{
		request.Chain,
		request.Receiver,
		request.UUID,
		request.Channel,
		request.ChannelName,
		request.CreatedTime,
		request.Hash,
		request.Link,
		string(mediumPublished),
		request.Message,
		request.Seen,
		request.Type,
		request.UpdatedTime,
		request.Logo,
		request.Verified,
	}

	if err = repo.db.Query(query, params...).Exec(); err != nil {
		log.WithError(err).Error("failed to execute query for inserting notification")
		return err
	}

	log.Debug("Notification sent")

	return nil
}

// InsertNotificationSentCountForReach inserts the notification reach information into the notification repository.
func (repo *NotificationRepo) InsertNotificationSentCountForReach(
	_ context.Context, request entities.NotificationRequest, count int,
) error {

	log := utilities.NewLogger("InsertNotificationSentCountForReach")

	tblNotificationSent := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationTotalSent)
	query := fmt.Sprintf(
		`INSERT INTO %s (hash, event_time, sent) VALUES (?, ?, ?) USING TTL %d`,
		tblNotificationSent, config.GetConfig().TTL.Notifications,
	)
	if err := repo.db.Query(query, request.Hash, utilities.TimeNow(), count).Exec(); err != nil {
		log.WithError(err).Error("failed to update notification sent count for reach")
		return err
	}

	return nil

}

// InsertChannelSendMetrics inserts the notification channel metrics into the notification repository.
func (repo *NotificationRepo) InsertChannelSendMetrics(
	_ context.Context, request entities.NotificationRequest, now time.Time, sent int,
) error {
	log := utilities.NewLogger("InsertChannelSendMetrics")

	tblNotificationChanMetrics := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.ChannelSentReadMetrics)

	query := fmt.Sprintf(
		`INSERT INTO %s (chain, channel, event_time, event_date, sent, read) VALUES %s USING TTL %d`,
		tblNotificationChanMetrics, utilities.DBMultiValuePlaceholders(6), config.GetConfig().TTL.Metrics,
	)
	err := repo.db.Query(
		query,
		request.Chain, request.Channel, now, utilities.ToDate(now), sent, 0,
	).Exec()
	if err != nil {
		log.WithError(err).Error("failed to insert")
		return err
	}
	return nil
}

func (repo *NotificationRepo) InsertUserSendMetrics(
	_ context.Context, request entities.NotificationRequest, now time.Time, sent int,
) error {
	log := utilities.NewLogger("InsertUserSendMetrics")

	tbl := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationTotalSendPerUserMetrics)

	query := fmt.Sprintf(
		`INSERT INTO %s (chain, address, event_time, event_date, sent) VALUES %s USING TTL %d`,
		tbl, utilities.DBMultiValuePlaceholders(5), repo.conf.TTL.UserTotalSend,
	)
	err := repo.db.Query(
		query,
		request.Chain, request.Sender, now, utilities.ToDate(now), sent,
	).Exec()
	if err != nil {
		log.WithError(err).Error("failed to insert")
		return err
	}
	return nil
}

// InsertNotificationChannelCounter inserts the notification channel counter information into the notification repository.
func (repo *NotificationRepo) InsertNotificationChannelCounter(
	_ context.Context, request entities.NotificationRequest, count int,
) error {

	log := utilities.NewLogger("InsertNotificationChannelCounter")

	tblNotificationChanCounter := fmt.Sprintf(
		`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationChannelCounter,
	)
	var totalSent int
	query := fmt.Sprintf(`SELECT total_sent from %s WHERE channel = ? AND chain = ?`, tblNotificationChanCounter)
	if err := repo.db.Query(query, request.Channel, request.Chain).Scan(&totalSent); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to read notification channel counter")
			return err
		}
	}

	query = fmt.Sprintf(`UPDATE %s SET total_sent = ? WHERE channel = ? AND chain = ?`, tblNotificationChanCounter)
	if err := repo.db.Query(query, totalSent+count, request.Channel, request.Chain).Exec(); err != nil {
		log.WithError(err).Error("failed to update notification channel counter")
		return err
	}

	return nil

}

// InsertGlobalStats inserts the notification statistics into the global statistics table.
func (repo *NotificationRepo) InsertGlobalStats(
	_ context.Context, request entities.NotificationRequest, count int,
) error {

	log := utilities.NewLogger("InsertGlobalStats")

	tblGlobalStat := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.GlobalStatistics)

	query := `UPDATE ` + tblGlobalStat + ` SET notifications_sent = notifications_sent + ? WHERE chain = ?`
	if err := repo.db.Query(query, count, request.Chain).Exec(); err != nil {
		log.WithError(err).Error("failed to update notification global counter")
		return err
	}

	return nil
}

// ListOfReadUsers retrieves the list of users who have seen a notification.
func (repo *NotificationRepo) ListOfReadUsers(_ context.Context, request entities.NotificationRequest) (
	[]string, error,
) {

	log := utilities.NewLogger("ListOfReadUsers")

	tblNotificationInfo := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationInfo)

	query := "SELECT seen FROM " + tblNotificationInfo + " WHERE uuid = ? AND chain = ? AND app_id = ?"

	var seen []string
	if err := repo.db.Query(query, request.UUID, request.Chain, request.Channel).Scan(&seen); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return []string{}, nil
		}
		log.WithError(err).Error("failed to retrieve list of read users")
		return nil, err
	}

	return seen, nil
}

// GetNotificationInfo retrieves the information of notifications for a specific user, chain, channel, and type.
func (repo *NotificationRepo) GetNotificationInfo(
	ctx context.Context, request entities.RequestNotification, pageSize int, pageState []byte,
) ([]entities.ReadNotification, []byte, error) {
	var notifications = make([]entities.ReadNotification, 0)

	log := utilities.NewLogger("GetNotificationInfo")

	tblNotificationInfo := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationInfo)
	tblNotificationReadStatus := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationReadStatus)

	var lastRead time.Time
	query := fmt.Sprintf(`SELECT last_read FROM %s WHERE address = ? AND chain = ? `, tblNotificationReadStatus)
	err := repo.db.Query(query, request.User, request.Chain).Scan(&lastRead)
	if err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to get read status")
			return notifications, nil, err
		}
	}

	query = fmt.Sprintf(
		`SELECT uuid, app_id, channel_name, logo, created_time, hash, link, message, type, verified FROM %s WHERE chain = ? AND receiver = ?`,
		tblNotificationInfo,
	)

	iter := repo.db.Query(query, request.Chain, request.User).PageSize(pageSize).PageState(pageState).Iter()
	currPageState := iter.PageState()

	var (
		uuid        string
		appID       string
		channelName string
		createdTime time.Time
		hash        string
		link        string
		message     string
		kind        string
		logo        string
		verified    bool
	)

	for iter.Scan(&uuid, &appID, &channelName, &logo, &createdTime, &hash, &link, &message, &kind, &verified) {
		seen := false
		if createdTime.Before(lastRead) || createdTime == lastRead {
			seen = true
		}

		notification := entities.ReadNotification{
			Message:     message,
			Link:        link,
			CreatedTime: createdTime,
			AppID:       appID,
			ChannelName: channelName,
			Logo:        logo,
			Hash:        hash,
			Uuid:        uuid,
			Kind:        kind,
			Seen:        seen,
			Verified:    verified,
		}
		notifications = append(notifications, notification)
	}

	if err = iter.Close(); err != nil {
		log.WithError(err).Error("failed to retrieve notifications")
		return notifications, []byte{}, err
	}

	if len(notifications) > 0 {
		data := &entities.UpdateReadStatusRequest{
			Uuid:    notifications[0].Uuid,
			Time:    notifications[0].CreatedTime,
			Medium:  consts.Inapp,
			Chain:   request.Chain,
			Address: request.User,
		}

		go func() {
			err = repo.updateReadStatus(ctx, data, notifications)
			if err != nil {
				log.WithError(err).Error("failed to update read status")
			}
		}()
	}

	return notifications, currPageState, nil
}

// updateReadStatus updates the read status of a notification for the specified UUID, chain, channel, and time.
func (repo *NotificationRepo) updateReadStatus(
	ctx context.Context, data *entities.UpdateReadStatusRequest, notifications []entities.ReadNotification,
) error {
	log := utilities.NewLogger("updateReadStatus")

	tblNotificationReadStatus := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationReadStatus)
	tblNotificationEmailReach := fmt.Sprintf(
		`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationEmailMediumReach,
	)
	tblNotificationDiscordReach := fmt.Sprintf(
		`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationDiscordMediumReach,
	)
	tblNotificationAppReach := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationAppMediumReach)
	tblNotificationChanMetrics := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.ChannelSentReadMetrics)

	var prevLastRead time.Time
	query := fmt.Sprintf(`SELECT last_read FROM %s WHERE address = ? AND chain = ? `, tblNotificationReadStatus)
	err := repo.db.Query(query, data.Address, data.Chain).Scan(&prevLastRead)
	if err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to get read status")
			return err
		}
	}

	query = fmt.Sprintf(`INSERT INTO %s (address, chain, last_read) VALUES (?, ?, ?) `, tblNotificationReadStatus)
	err = repo.db.Query(query, data.Address, data.Chain, data.Time).Exec()
	if err != nil {
		log.WithError(err).Error("failed to set read status")
		return err
	}

	channelHashMap := make(map[string]string)
	for _, notification := range notifications {
		if notification.Seen {
			continue
		}
		channelHashMap[notification.Hash] = notification.AppID
	}

	log.Debugf("Updating metrics for %d notifications", len(channelHashMap))

	for hash, appID := range channelHashMap {
		query = `INSERT INTO %s (hash, event_time, read) VALUES (?, ?, ?) USING TTL ?`

		switch data.Medium {
		case consts.Discord:
			query = fmt.Sprintf(query, tblNotificationDiscordReach)
		case consts.Email:
			query = fmt.Sprintf(query, tblNotificationEmailReach)
		default:
			query = fmt.Sprintf(query, tblNotificationAppReach)
		}

		err = repo.db.Query(query, hash, utilities.TimeNow(), 1, config.GetConfig().TTL.Metrics).Exec()
		if err != nil {
			log.WithError(err).Error("failed to update notification reach")
			continue
		}

		query = fmt.Sprintf(
			`INSERT INTO %s (chain, channel, event_date, event_time, medium, sent, read) VALUES %s USING TTL %d`,
			tblNotificationChanMetrics, utilities.DBMultiValuePlaceholders(7), config.GetConfig().TTL.Metrics,
		)

		err = repo.db.Query(
			query, data.Chain, appID, utilities.ToDate(utilities.TimeNow()), utilities.TimeNow(), data.Medium, 0, 1,
		).Exec()
		if err != nil {
			log.WithError(err).Errorf("failed to update %s", tblNotificationChanMetrics)
			continue
		}
	}

	return nil
}

// NotificationReachCount retrieves the notification reach count for a specific UUID.
func (repo *NotificationRepo) NotificationReachCount(
	_ context.Context, uuid string, pageSize int, pageState []byte,
) ([]entities.NotificationReach, []byte, error) {

	var totalSent int
	var mediumReadCount map[string]int
	log := utilities.NewLogger("NotificationReachCount")

	tblNotificationMediumReach := fmt.Sprintf(
		`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationEmailMediumReach,
	)
	tblNotificationReach := fmt.Sprintf(`%s.%s`, config.GetConfig().DB.Keyspace, consts.NotificationTotalSent)

	query := fmt.Sprintf(
		`
	SELECT
	medium_read_count
	FROM
	%s
	WHERE 
	uuid = ?`, tblNotificationMediumReach,
	)

	iter1 := repo.db.Query(query, uuid).PageSize(pageSize).PageState([]byte{}).Iter()
	if !iter1.Scan(&mediumReadCount) {
		log.Errorf("no data found in notification_medium_reach for UUID: %s", uuid)
		return nil, []byte{}, fmt.Errorf("no data found in notification_medium_reach for uuid provided")
	}
	if err := iter1.Close(); err != nil {
		log.WithError(err).Error("failed to close iterator")
		return nil, []byte{}, err
	}

	query = fmt.Sprintf(
		`
			SELECT
			COUNT(hash)
			FROM 
			%s
			WHERE
			hash = ?
			`, tblNotificationReach,
	)

	iter2 := repo.db.Query(query, uuid).PageSize(pageSize).PageState(pageState).Iter()
	currPageState := iter2.PageState()
	if !iter2.Scan(&totalSent) {
		log.Errorf("no data found in notification_total_sent for UUID: %s", uuid)
		return nil, []byte{}, fmt.Errorf("no data found in notification_total_sent for uuid provided")
	}
	if err := iter2.Close(); err != nil {
		log.WithError(err).Error("failed to close iterator")
		return nil, []byte{}, err
	}

	notificationReach := entities.NotificationReach{
		MediumReadCount: mediumReadCount,
		TotalSent:       totalSent,
	}

	result := []entities.NotificationReach{notificationReach}
	return result, currPageState, nil
}
