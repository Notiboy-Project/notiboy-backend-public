package entities

import "time"

type Database struct {
	Host     string
	Keyspace string
	Username string
	Password string
}

type TimeToLive struct {
	UserActivityMetrics    string `default:"31536000"`
	ChannelActivityMetrics string `default:"31536000"`
	NotificationInfo       string `default:"31536000"`
	VerifyInfo             string `default:"14400"`
}

type Email struct {
	FromEmail    string
	FromUserName string
}

type TimeDelay struct {
	Delay time.Duration `default:"1m"`
}
type ChainAPI struct {
	Address string `json:"address"`
	Token   string `json:"token"`
}

type BlockModel struct {
	Block float64
	Time  time.Time
}
