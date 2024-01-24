package entities

import "time"

type ScheduleNotificationRequest struct {
	Chain     string
	Sender    string
	Receivers []string `json:"receivers"`
	Message   string   `json:"message" validate:"required"`
	Link      string   `json:"link" validate:"required"`
	Channel   string   `json:"app_id"`
	Type      string
	Schedule  time.Time
	TTL       int
}

type NotificationRequest struct {
	User                   string    `json:"user,omitempty"`
	Message                string    `json:"message" validate:"required"`
	Link                   string    `json:"link" validate:"required"`
	Schedule               time.Time `json:"schedule,omitempty"`
	Sender                 string
	MediumPublished        map[string]bool
	Chain                  string
	Channel                string
	Type                   string
	Sent                   int
	Read                   int
	Hash                   string
	UUID                   string
	Status                 string
	MediumReadCount        map[string]int
	Seen                   []string
	Time                   time.Time
	Medium                 string
	Receivers              []string `json:"receivers"`
	ReceiverMails          []string
	UpdatedTime            time.Time
	DiscordReceiverIds     map[string]string
	UnverifiedDiscordIDs   []string
	SystemSupportedMediums []string
}

type MediumPublishedMeta struct {
	Published bool
	Allowed   bool
}
type Notification struct {
	Chain           string                         `json:"chain,omitempty"`
	Receiver        string                         `json:"receiver"`
	ReceiverInfo    UserModel                      `json:"receiverInfo"`
	UUID            string                         `json:"UUID,omitempty"`
	Channel         string                         `json:"channel,omitempty"`
	ChannelName     string                         `json:"channel_name,omitempty"`
	CreatedTime     time.Time                      `json:"createdTime"`
	Hash            string                         `json:"hash,omitempty"`
	Link            string                         `json:"link,omitempty"`
	MediumPublished map[string]MediumPublishedMeta `json:"mediumPublished,omitempty"`
	Message         string                         `json:"message,omitempty"`
	Seen            bool                           `json:"seen,omitempty"`
	Type            string                         `json:"type,omitempty"`
	UpdatedTime     time.Time                      `json:"updatedTime,omitempty"`
	TTL             int                            `json:"ttl"`
	Logo            string                         `json:"logo"`
	Verified        bool                           `json:"verified"`
}

type RequestNotification struct {
	Chain   string
	Channel string
	Type    string
	User    string
}
type ReadNotification struct {
	Message     string    `json:"message,omitempty"`
	Seen        bool      `json:"seen,omitempty"`
	Link        string    `json:"link,omitempty"`
	CreatedTime time.Time `json:"created_time"`
	AppID       string    `json:"app_id,omitempty"`
	ChannelName string    `json:"channel_name"`
	Hash        string    `json:"hash,omitempty"`
	Uuid        string    `json:"uuid,omitempty"`
	Kind        string    `json:"kind,omitempty"`
	Logo        string    `json:"logo,omitempty"`
	Verified    bool      `json:"verified"`
}

type UpdateReadStatusRequest struct {
	Uuid    string    `json:"uuid" validate:"required"`
	Time    time.Time `json:"timestamp" validate:"required"`
	Medium  string    `json:"medium" validate:"required"`
	Chain   string
	Address string
}

type NotificationReach struct {
	MediumReadCount map[string]int `json:"medium_read_count"`
	TotalSent       int            `json:"total_sent"`
}

type NotificationReachResponse struct {
	Status  string              `json:"status"`
	Message string              `json:"message"`
	Data    []NotificationReach `json:"data"`
}
