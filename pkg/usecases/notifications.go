package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"firebase.google.com/go/v4/messaging"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/repo"
	"notiboy/pkg/repo/driver/db"
	"notiboy/pkg/repo/driver/medium"
	"notiboy/utilities"

	uuidLib "github.com/google/uuid"
)

var nuc *NotificationUsecases

type NotificationUsecases struct {
	repo     repo.NotificationRepoImply
	userRepo repo.UserRepoImply
	verify   repo.VerifyRepoImply
	channel  repo.ChannelRepoImpl
	optin    repo.OptinRepoImply
	ws       *medium.Socket
}

func (usecase *NotificationUsecases) DeleteScheduledNotificationInfo(
	ctx context.Context, chain string, sender string, schedule time.Time,
) error {
	err := usecase.repo.DeleteScheduledNotificationInfo(ctx, chain, sender, schedule)
	if err != nil {
		return fmt.Errorf("failed to delete scheduled notification: %w", err)
	}

	return nil
}

func (usecase *NotificationUsecases) UpdateScheduledNotificationInfo(
	ctx context.Context, request *entities.ScheduleNotificationRequest,
) error {
	err := usecase.repo.UpdateScheduledNotificationInfo(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to update scheduled notification: %w", err)
	}

	return nil
}

type NotificationUsecaseImply interface {
	SendNotifications(context.Context, entities.NotificationRequest) error
	GetNotifications(context.Context, entities.RequestNotification, int, []byte) (
		[]entities.ReadNotification, []byte, error,
	)
	GetScheduledNotificationsBySender(context.Context, string, string) ([]entities.NotificationRequest, error)
	DeleteScheduledNotificationInfo(context.Context, string, string, time.Time) error
	UpdateScheduledNotificationInfo(context.Context, *entities.ScheduleNotificationRequest) error
	NotificationReachCount(context.Context, string, int, []byte) ([]entities.NotificationReach, []byte, error)
}

// NewNotificationUsecases creates a new instance of the NotificationUsecases struct
func NewNotificationUsecases(
	notificationRepo repo.NotificationRepoImply, userRepo repo.UserRepoImply, verify repo.VerifyRepoImply,
	channel repo.ChannelRepoImpl, optin repo.OptinRepoImply, ws *medium.Socket,
) NotificationUsecaseImply {
	nuc = &NotificationUsecases{
		repo:     notificationRepo,
		userRepo: userRepo,
		verify:   verify,
		channel:  channel,
		optin:    optin,
		ws:       ws,
	}

	return nuc
}

func GetNotificationUsecases() *NotificationUsecases {
	return nuc
}

func NotificationSchedulerStub(ctx context.Context, usecase *NotificationUsecases) {
	log := utilities.NewLogger("NotificationSchedulerStub")

	ticker := time.NewTicker(time.Second * 30)

	go func() {
		runOnce := make(chan struct{}, 1)

		for {
			select {
			case <-ctx.Done():
				log.Info("Terminating...")
				ticker.Stop()
				return
			case <-ticker.C:
				select {
				// at any point of time, no more than one go routine should run
				case runOnce <- struct{}{}:
					go notificationScheduler(ctx, usecase, runOnce)
				default:
				}
			}
		}
	}()
}

func notificationScheduler(ctx context.Context, usecase *NotificationUsecases, runOnce chan struct{}) {
	log := utilities.NewLogger("notificationScheduler")

	defer func() {
		<-runOnce
	}()

	var requests []entities.NotificationRequest
	for _, chain := range config.GetConfig().Chain.Supported {
		// fetching notifications scheduled 30 seconds in advance as well
		reqs, err := usecase.repo.GetScheduledNotificationInfo(ctx, chain, utilities.TimeNow().Add(30*time.Second))
		if err != nil {
			log.WithError(err).Error("failed to get scheduled notifications")
			continue
		}

		requests = append(requests, reqs...)
	}

	throttler := make(chan struct{}, 100)
	wg := new(sync.WaitGroup)

	for _, request := range requests {
		throttler <- struct{}{}

		go func(request entities.NotificationRequest) {
			wg.Add(1)
			defer func() {
				wg.Done()
				<-throttler
			}()

			chain := request.Chain
			schedule := request.Schedule
			// so that request won't be put in scheduled queue again
			request.Schedule = time.Time{}
			err := usecase.SendNotifications(ctx, request)
			if err != nil {
				log.WithError(err).Error("failed to send notifications")
				return
			}

			tblNotificationInfo := fmt.Sprintf(
				`%s.%s`, config.GetConfig().DB.Keyspace, consts.ScheduledNotificationInfo,
			)
			query := fmt.Sprintf(`DELETE FROM %s WHERE chain = ? AND schedule = ?`, tblNotificationInfo)
			if err = db.GetCassandraSession().Query(query, chain, schedule).Exec(); err != nil {
				log.WithError(err).Error("failed to execute query for deleting schedule notification")
			}
		}(request)
	}
	wg.Wait()
}

func (usecase *NotificationUsecases) GetScheduledNotificationsBySender(
	ctx context.Context, chain string, sender string,
) ([]entities.NotificationRequest, error) {
	reqs, err := usecase.repo.GetScheduledNotificationInfoBySender(ctx, chain, sender)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled notifications: %w", err)
	}

	return reqs, nil
}

// SendNotifications sends private notifications to the specified user.
func (usecase *NotificationUsecases) SendNotifications(
	ctx context.Context, request entities.NotificationRequest,
) error {
	log := utilities.NewLogger("SendNotifications")

	chain := request.Chain
	sender := request.Sender
	channel := request.Channel
	kind := request.Type
	link := request.Link
	message := request.Message
	schedule := request.Schedule

	userModel, err := db.GetUserModel(ctx, chain, sender)
	if err != nil {
		return fmt.Errorf("getting user model failed: %w", err)
	}
	membership := consts.MembershipStringToEnum(userModel.Membership)
	ttl := consts.NotificationRetentionSecs[membership]

	if !schedule.IsZero() {
		maxFutureTime := utilities.TimeNow().Add(time.Second * time.Duration(consts.NotificationMaxSchedule[membership]))
		if schedule.After(maxFutureTime) {
			return fmt.Errorf(
				"cannot schedule a message later than %s (max message retention period) in your current membership tier: %s",
				maxFutureTime, membership,
			)
		}

		err = usecase.repo.InsertScheduledNotificationInfo(
			ctx, &entities.ScheduleNotificationRequest{
				Chain:     chain,
				Sender:    sender,
				Receivers: request.Receivers,
				Message:   message,
				Link:      link,
				Channel:   channel,
				Type:      kind,
				Schedule:  schedule,
				TTL:       ttl,
			},
		)
		if err != nil {
			log.WithError(err).Error("error inserting schedule notification info")
			return fmt.Errorf("error inserting schedule notification info: %w", err)
		}

		return nil
	}

	hash := utilities.Encrypt(message)
	request.Hash = hash

	uuid := uuidLib.NewString()
	now := utilities.TimeNow()

	permittedCharCount := consts.NotificationCharacterCount[membership]
	curMsgCharCount := len(message)
	curLinkCharCount := len(link)

	if curMsgCharCount > permittedCharCount {
		return fmt.Errorf(
			"cannot send notification as your membership allows only %d notification text character count",
			permittedCharCount,
		)
	}

	if curLinkCharCount > permittedCharCount {
		return fmt.Errorf(
			"cannot send notification as your membership allows only %d notification link character count",
			permittedCharCount,
		)
	}

	channelInfo, err := usecase.channel.GetChannel(ctx, chain, channel, true)
	if err != nil {
		log.WithError(err).Error("failed to get channel")
		return err
	}

	channelData := channelInfo.Data.(entities.ChannelModel)

	if channelData.Status == consts.STATUS_CHANNEL_LIMIT_EXCEEDED {
		msg := fmt.Sprintf("cannot send notification - limit exceeded. Delete a channel or upgrade membership to continue")
		log.Warnf(msg)
		return fmt.Errorf(msg)
	}

	if channelData.Status != consts.STATUS_ACTIVE {
		msg := fmt.Sprintf("channel is marked %s, cannot send notification", channelData.Status)
		log.Warnf(msg)
		return fmt.Errorf(msg)
	}

	if sender != channelData.Owner {
		log.Warnf("Sender %s is not the owner %s of the channel", sender, channelData.Owner)
		return fmt.Errorf("sender is not the owner of the channel")
	}

	if kind == "public" {
		request.Receivers, err = usecase.channel.RetrieveChannelUsers(ctx, chain, channel)
		if err != nil {
			return fmt.Errorf("failed to fetch users list for channel %s: %w", channel, err)
		}
	}

	toSend := len(request.Receivers)
	if toSend == 0 {
		log.Warn("nothing to send, receivers not specified")
		return nil
	}

	totalSent, err := usecase.userRepo.GetUserSendMetricsForMonth(ctx, chain, sender)
	if err != nil {
		log.WithError(err).Errorf("failed to get user send metrics")
		return err
	}
	remainingNotifications := consts.NotificationCount[membership] - totalSent
	if remainingNotifications < 0 {
		remainingNotifications = 0
	}
	if toSend > remainingNotifications {
		return fmt.Errorf(
			"cannot send %d notifications as your membership only allows %d more notifications",
			toSend, remainingNotifications,
		)
	}

	sent := 0
	for _, receiver := range request.Receivers {
		receiverInfo, err := usecase.userRepo.GetUser(
			ctx, entities.UserIdentifier{
				Chain:   chain,
				Address: receiver,
			},
		)
		if err != nil {
			log.WithError(err).Warnf("failed to get user info for %s", receiver)
			continue
		}

		data, ok := receiverInfo.Data.(*entities.UserModel)
		if !ok {
			log.Warnf("incorrect get user info for %s", receiver)
			continue
		}

		allowedMediumsMap := utilities.SliceToMap(data.AllowedMediums)

		if !utilities.ContainsString(data.Optins, channel) {
			log.Warnf("user %s has not opted in to channel %s", receiver, channel)
			continue
		}

		notification := &entities.Notification{
			Chain:        chain,
			Receiver:     receiver,
			ReceiverInfo: *data,
			UUID:         uuid,
			Channel:      channel,
			ChannelName:  channelData.Name,
			Logo:         channelData.Logo,
			CreatedTime:  now,
			Hash:         hash,
			Link:         link,
			MediumPublished: map[string]entities.MediumPublishedMeta{
				consts.Discord: {
					Published: false,
					Allowed:   allowedMediumsMap[consts.Discord],
				},
				consts.Email: {
					Published: false,
					Allowed:   allowedMediumsMap[consts.Email],
				},
			},
			Message:     message,
			Seen:        false,
			Type:        kind,
			UpdatedTime: now,
			TTL:         ttl,
			Verified:    channelData.Verified,
		}

		err = usecase.repo.InsertNotificationInfo(ctx, notification)
		if err != nil {
			log.Error("error inserting notification info:", err)
			continue
		}

		go func(notification *entities.Notification) {
			identifier := medium.FormatIdentifier(chain, notification.Receiver)

			n := entities.ReadNotification{
				Message:     notification.Message,
				Seen:        false,
				Link:        notification.Link,
				CreatedTime: notification.CreatedTime,
				AppID:       notification.Channel,
				ChannelName: notification.ChannelName,
				Hash:        notification.Hash,
				Uuid:        notification.UUID,
				Kind:        notification.Type,
				Logo:        notification.Logo,
			}
			data, err := json.Marshal(n)
			if err != nil {
				log.WithError(err).Errorf("ffailed to marshal notification data %+v", n)
				return
			} else {
				if err = usecase.ws.PushMessage(identifier, data, false); err != nil {
					log.WithError(err).Error("failed to push websocket notification")
				}
			}

			msg := messaging.Message{
				Notification: &messaging.Notification{
					Title: fmt.Sprintf("Notification from %s", notification.ChannelName),
					Body:  notification.Message,
				},
				Data: map[string]string{
					"click_action": "FLUTTER_NOTIFICATION_CLICK",
					"seen":         "false",
					"link":         notification.Link,
					"created_time": notification.CreatedTime.Format("2006-01-02T15:04:05Z"),
					"app_id":       notification.Channel,
					"channel_name": notification.ChannelName,
					"hash":         notification.Hash,
					"uuid":         notification.UUID,
					"kind":         notification.Type,
				},
				APNS: &messaging.APNSConfig{Payload: &messaging.APNSPayload{Aps: &messaging.Aps{Sound: "default"}}},
			}
			tokens, err := usecase.userRepo.GetFCMTokens(
				ctx, entities.UserIdentifier{
					Chain:   chain,
					Address: receiver,
				},
			)
			if err != nil {
				log.WithError(err).Error("failed to get fcm tokens")
			}
			if len(tokens) > 0 {
				if err = medium.GetFirebaseClient().PushMessageToClient(ctx, notification.Chain, notification.Receiver, msg, tokens); err != nil {
					log.WithError(err).Errorf("failed to push notification")
				}
			}

			if config.GetConfig().Mode != "local" {
				medium.GetDiscordMessenger().Enqueue(notification)
				medium.GetEmailClient().Enqueue(notification)
			}
		}(notification)

		sent++
	}

	err = usecase.repo.InsertGlobalStats(ctx, request, sent)
	if err != nil {
		log.Errorf("failed to insert global stats: %v", err)
	}

	err = usecase.repo.InsertNotificationChannelCounter(ctx, request, sent)
	if err != nil {
		log.Errorf("failed to insert update channel counter: %v", err)
	}

	err = usecase.repo.InsertChannelSendMetrics(ctx, request, now, sent)
	if err != nil {
		log.Errorf("failed to insert notification channel send metrics: %v", err)
	}

	err = usecase.repo.InsertNotificationSentCountForReach(ctx, request, sent)
	if err != nil {
		log.Errorf("failed to insert notification sent count for reach: %v", err)
	}

	err = usecase.repo.InsertUserSendMetrics(ctx, request, now, sent)
	if err != nil {
		log.Errorf("failed to insert total user sent count: %v", err)
	}

	if sent == 0 {
		return fmt.Errorf("failed to send notification")
	}

	if sent != toSend {
		log.Infof("Notification sent only to %d instead of %d", sent, toSend)
	}

	return nil
}

// GetNotifications retrieves the notification information based on the provided criteria.
func (usecase *NotificationUsecases) GetNotifications(
	ctx context.Context, request entities.RequestNotification, pageSize int, pageState []byte,
) ([]entities.ReadNotification, []byte, error) {
	return usecase.repo.GetNotificationInfo(ctx, request, pageSize, pageState)
}

// NotificationReachCount retrieves the reach count information for a notification with the given UUID.
func (usecase *NotificationUsecases) NotificationReachCount(
	ctx context.Context, uuid string, pageSize int, pageState []byte,
) ([]entities.NotificationReach, []byte, error) {
	return usecase.repo.NotificationReachCount(ctx, uuid, pageSize, pageState)
}
