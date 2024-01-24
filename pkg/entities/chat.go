package entities

import "time"

const (
	MsgDelivered = "delivered"
	MsgSubmitted = "submitted"
	MsgBlocked   = "blocked"

	UserChatMsgKind    = "chat"
	UserChatStatusKind = "status"
	UserChatAckKind    = "ack"
	UserChatSeenKind   = "seen"
)

type UserChat struct {
	Chain    string `json:"chain,omitempty"`
	UserA    string `json:"user_a,omitempty"`
	UserB    string `json:"user_b,omitempty"`
	Sender   string `json:"sender,omitempty"`
	Message  string `json:"message,omitempty"`
	Uuid     string `json:"uuid,omitempty"`
	Status   string `json:"status,omitempty"`
	SentTime int64  `json:"sent_time,omitempty"`
}

type UserChatResponse struct {
	Uuid   string `json:"uuid,omitempty"`
	Status string `json:"status,omitempty"`
	Sender string `json:"sender,omitempty"`
	Time   int64  `json:"time,omitempty"`
}

type UserStatus struct {
	Chain  string `json:"chain"`
	User   string `json:"user"`
	Online bool   `json:"online"`
}

type GroupChatInfo struct {
	Chain        string    `json:"chain,omitempty"`
	GID          string    `json:"gid,omitempty"`
	Name         string    `json:"name,omitempty"`
	Description  string    `json:"description,omitempty"`
	Owner        string    `json:"owner,omitempty"`
	Admins       []string  `json:"admins,omitempty"`
	Users        []string  `json:"users,omitempty"`
	BlockedUsers []string  `json:"blocked_users,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type GroupChat struct {
	Chain    string `json:"chain,omitempty"`
	GID      string `json:"gid,omitempty"`
	Sender   string `json:"sender,omitempty"`
	Message  string `json:"message,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	Status   string `json:"status,omitempty"`
	SentTime int64  `json:"sent_time,omitempty"`
}
