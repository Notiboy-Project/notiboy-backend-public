package entities

import "time"

type ChannelUsers struct {
	AppId string `json:"id"`
	Chain string `json:"chain"`
	Users string `json:"title"`
}

type ChannelModel struct {
	Name             string    `json:"name,omitempty" `
	Description      string    `json:"description,omitempty"`
	Logo             string    `json:"logo,omitempty" `
	Chain            string    `json:"chain"`
	AppID            string    `json:"app_id"`
	Owner            string    `json:"address"`
	Verified         bool      `json:"verified"`
	Status           string    `json:"status"`
	CreatedTimestamp time.Time `json:"created_timestamp"`
}

type ChannelInfo struct {
	Name        string `json:"name,omitempty" `
	Description string `json:"description,omitempty"`
	Logo        string `json:"logo,omitempty" `
	Chain       string `json:"chain"`
	AppID       string `json:"app_id"`
	Address     string `json:"address"`
}

type ChannelStatsResponse struct {
	Data       []ChannelActivity `json:"data"`
	StatusCode int               `json:"status_code"`
	Message    string            `json:"message"`
}

type ChannelActivity struct {
	Created int    `json:"created"`
	Deleted int    `json:"deleted"`
	Date    string `json:"date"`
}

type ChannelReadSentResponse struct {
	EventDate string `json:"event_date"`
	Medium    string `json:"medium,omitempty"`
	Read      int    `json:"read,omitempty"`
	Sent      int    `json:"sent,omitempty"`
}

type ListChannelRequest struct {
	Pagination `json:"pagination"`
	Chain      string `json:"chain,omitempty"`
	WithLogo   bool   `json:"withLogo,omitempty"`
	Verified   bool   `json:"verified,omitempty"`
	NameSearch string `json:"nameSearch,omitempty"`
}

type ListChannelUsersRequest struct {
	Pagination
	AppId       string `json:"id"`
	Chain       string `json:"chain"`
	Address     string `json:"address"`
	WithLogo    bool
	AddressOnly bool
}
