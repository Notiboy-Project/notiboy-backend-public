package medium

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"notiboy/config"
	"notiboy/utilities"
)

type FirebaseModel struct {
	fcmClient *messaging.Client
}

var firebaseObj *FirebaseModel

func GetFirebaseClient() *FirebaseModel {
	return firebaseObj
}

func InitFirebase(ctx context.Context, conf *config.NotiboyConfModel) error {
	// Use the path to your service account credential json file
	opt := option.WithCredentialsFile(conf.Firebase.Path)
	// Create a new firebase app
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return fmt.Errorf("failed to created new app with config path %s: %w", conf.Firebase.Path, err)
	}
	// Get the FCM object
	fcmClient, err := app.Messaging(ctx)
	if err != nil {
		return fmt.Errorf("failed to created new messaging client: %w", err)
	}

	firebaseObj = &FirebaseModel{fcmClient: fcmClient}

	return nil
}

func (fb *FirebaseModel) PushMessageToClient(ctx context.Context, chain, receiver string, msg messaging.Message, deviceIDs []string) error {
	log := utilities.NewLoggerWithFields(
		"firebase.PushMessageToClient", map[string]interface{}{
			"chain":   chain,
			"address": receiver,
		},
	)

	var messages []*messaging.Message
	for _, deviceID := range deviceIDs {
		newMsg := msg
		newMsg.Token = deviceID
		messages = append(messages, &newMsg)
	}

	resp, err := fb.fcmClient.SendEach(ctx, messages)
	if err != nil {
		return err
	}

	if resp.FailureCount > 0 {
		for _, errResp := range resp.Responses {
			if errResp != nil && errResp.Error != nil {
				log.WithError(errResp.Error).Errorf(
					"failed to push firebase notification to %s:%s",
					chain, receiver,
				)
			}
		}
	}

	log.Debugf(
		"firebase notification pushed to %s:%s for %d device IDs",
		chain, receiver, len(deviceIDs),
	)

	return nil
}
