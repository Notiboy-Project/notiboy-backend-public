package medium

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/utilities"
	"notiboy/utilities/http_client"

	"github.com/bwmarrin/discordgo"
)

var discordMessenger *DiscordMessenger

type DiscordMessenger struct {
	Token  string
	Client *discordgo.Session
	queue  chan *entities.Notification
}

func GetDiscordMessenger() *DiscordMessenger {
	return discordMessenger
}

func NewDiscordMessenger(token string) (*DiscordMessenger, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	err = dg.Open()
	if err != nil {
		return nil, err
	}

	discordMessenger = &DiscordMessenger{
		Token:  token,
		Client: dg,
		queue:  make(chan *entities.Notification, 100),
	}

	return discordMessenger, err
}

func GetChannelID(receiverID string) (string, error) {
	payload := map[string]string{
		"recipient_id": receiverID,
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", consts.GetChannelIdUrl, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s", config.GetConfig().Discord.BotToken))
	client := http_client.GetClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var channel entities.DiscordChannel
	json.NewDecoder(resp.Body).Decode(&channel)
	return channel.ID, nil

}
func (d *DiscordMessenger) Close() {
	defer d.Client.Close()
}

func (d *DiscordMessenger) SpawnSender(ctx context.Context) {
	log := utilities.NewLogger("Discord.SpawnSender")
	for {
		select {
		case notification := <-d.queue:
			message := notification.Message
			if !notification.MediumPublished[consts.Discord].Allowed {
				continue
			}
			discordMeta := notification.ReceiverInfo.MediumMetadata.Discord
			if !discordMeta.Verified || discordMeta.DMChannelID == "" {
				continue
			}

			msg := fmt.Sprintf("*Announcement from* **%s**\n", notification.ChannelName)
			if notification.Type == "private" {
				msg = fmt.Sprintf("*You have a notification from* **%s**\n", notification.ChannelName)
			}

			msg = fmt.Sprintf("%s ```%s```\n", msg, message)

			if notification.Link != "" {
				msg = fmt.Sprintf("%s\nLink: %s", msg, notification.Link)
			}

			id, err := d.Client.ChannelMessageSendComplex(
				discordMeta.DMChannelID, &discordgo.MessageSend{
					Content: msg,
				},
			)
			if err != nil {
				log.WithError(err).Errorf("failed to send discord message to %s", notification.Receiver)
				continue
			}

			log.Debugf("Discord notification %s sent to channel %s of user %s", id.ID, discordMeta.DMChannelID, notification.Receiver)
		case <-ctx.Done():
			log.Infof("Shutting down")
			return
		}
	}
}

func (d *DiscordMessenger) Enqueue(notification *entities.Notification) {
	d.queue <- notification
}

func GetToken(discordtoken string) (string, error) {
	dc := config.GetConfig().Discord

	redirectURI, err := url.JoinPath(config.GetConfig().Server.RedirectPrefix, dc.RedirectURI)
	if err != nil {
		return "", fmt.Errorf("failed to create redirect_uri: %v", err)
	}

	payloadStr := fmt.Sprintf(
		"client_id=%s&client_secret=%s&grant_type=authorization_code&code=%s&redirect_uri=%s",
		dc.ClientID, dc.ClientSecret, discordtoken, redirectURI,
	)
	payload := strings.NewReader(payloadStr)

	req, err := http.NewRequest("POST", consts.DiscordGetToken, payload)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	defer res.Body.Close()

	var tokenResp struct {
		TokenType   string `json:"token_type"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}
	return tokenResp.AccessToken, nil
}

func GetCurrentUserID(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", consts.DiscordGetCurrentUser, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	var user struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	return user.ID, nil
}

func AddServerMember(accessToken string, guildID string, userID string) error {
	type MemberAddRequest struct {
		AccessToken string `json:"access_token"`
	}

	memberAddReq := &MemberAddRequest{
		AccessToken: accessToken,
	}

	body, err := json.Marshal(memberAddReq)
	if err != nil {
		return fmt.Errorf("failed to encode request: %v", err)
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf(consts.DiscordAddMember+"/%s/members/%s", guildID, userID), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+config.GetConfig().Discord.BotToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	return nil
}
