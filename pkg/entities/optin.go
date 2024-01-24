package entities

type OptInOut struct {
	Optin  int    `json:"optin"`
	Optout int    `json:"optout"`
	Date   string `json:"date"`
}

type ChannelOptInOutStats struct {
	OptInOut   []OptInOut `json:"optInOut"`
	TotalUsers int64      `json:"totalUsers"`
}
