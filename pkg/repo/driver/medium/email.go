package medium

import (
	"context"
	"fmt"
	"net/url"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go/aws"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/ui/templates"
	_ "notiboy/ui/templates"
	"notiboy/utilities"
)

var emailClient *EmailClient

type EmailClient struct {
	client *sesv2.Client
	queue  chan *entities.Notification
}

func GetEmailClient() *EmailClient {
	return emailClient
}

func NewEmailClient() (*EmailClient, error) {
	log := utilities.NewLogger("NewEmailClient")

	emailConfig := config.GetConfig().Email

	region := emailConfig.Region
	username := emailConfig.Username
	password := emailConfig.Password

	amazonConfiguration, err :=
		awsConfig.LoadDefaultConfig(
			context.Background(),
			awsConfig.WithRegion(region),
			awsConfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					username, password, "",
				),
			),
		)
	if err != nil {
		log.WithError(err).Error("failed to create amazon config")
		return nil, err
	}

	mailClient := sesv2.NewFromConfig(amazonConfiguration)

	emailClient = new(EmailClient)
	emailClient.client = mailClient
	emailClient.queue = make(chan *entities.Notification, 100)

	return emailClient, nil
}

func (ec *EmailClient) Close() {
	return
}

func (ec *EmailClient) Enqueue(notification *entities.Notification) {
	ec.queue <- notification
}

func (ec *EmailClient) SpawnSender(ctx context.Context) {
	log := utilities.NewLogger("Email.SpawnSender")

	for {
		select {
		case notification := <-ec.queue:
			if !notification.MediumPublished[consts.Email].Allowed {
				continue
			}
			emailMeta := notification.ReceiverInfo.MediumMetadata.Email
			if !emailMeta.Verified || emailMeta.ID == "" {
				continue
			}

			subject := fmt.Sprintf("Announcement from %s", notification.ChannelName)
			if notification.Type == "private" {
				subject = fmt.Sprintf("You have a notification from %s", notification.ChannelName)
			}

			msg := notification.Message
			link, err := url.JoinPath(config.GetConfig().Server.RedirectPrefix, "notifications")
			if err != nil {
				log.WithError(err).Error("url joining failed")
				continue
			}
			if notification.Link != "" {
				link = notification.Link
			}

			body, err := utilities.TemplateRendering(templates.NotificationTemplate, map[string]string{
				"Heading": subject,
				"Message": msg,
				"Link":    link,
			})
			if err != nil {
				log.WithError(err).Error("failed to render email template")
				continue
			}

			err = ec.SendMail(
				ctx, config.GetConfig().Email.Notification.From, emailMeta.ID, subject, body.String(),
			)
			if err != nil {
				log.WithError(err).Errorf("failed to email %s", notification.Receiver)
				continue
			}

		case <-ctx.Done():
			log.Infof("Shutting down")
			return
		}
	}
}
func (ec *EmailClient) SendMail(ctx context.Context, from, to, subject, body string) error {
	log := utilities.NewLoggerWithFields("SendMail", map[string]interface{}{
		"to":   to,
		"from": from,
	})

	client := ec.client
	charset := aws.String("UTF-8")

	input := &sesv2.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Charset: charset,
					Data:    aws.String(subject),
				},
				Body: &types.Body{
					Html: &types.Content{
						Charset: charset,
						Data:    aws.String(body),
					},
				},
			},
		},
		FromEmailAddress: aws.String(from),
	}

	// Attempt to send the email.
	_, err := client.SendEmail(ctx, input)
	if err != nil {
		log.WithError(err).Error("failed to send email")
		return err
	}
	log.Debug("Email sent!")

	return nil
}
