package entities

import "time"

type EmailMetadata struct {
	EmailID string
}

type UserModel struct {
	UserIdentifier
	SupportedMediums []string               `json:"supported_mediums,omitempty"`
	AllowedMediums   []string               ` json:"allowed_mediums,omitempty"`
	Membership       string                 ` json:"membership,omitempty"`
	Logo             string                 ` json:"logo,omitempty"`
	MediumMetadata   MediumMetadata         `json:"medium_metadata"`
	Status           string                 `json:"status,omitempty"`
	Channels         []string               `json:"channels,omitempty"`
	Optins           []string               `json:"optins,omitempty"`
	Privileges       map[string]interface{} `json:"privileges,omitempty"`
}

type UserInfo struct {
	UserIdentifier
	SupportedMediums []string `json:"supported_mediums,omitempty"`
	AllowedMediums   []string `json:"allowed_mediums,omitempty"`
	Membership       string   `json:"membership,omitempty"`
	Logo             string   `json:"logo,omitempty"`
	MediumMetadata   map[string]struct{}
}
type OnboardingRequest struct {
	UserIdentifier
	AllowedMediums []string `json:"allowed_mediums,omitempty"`
	Membership     string   `json:"membership" binding:"required"`
}

type UserActivity struct {
	Onboard  int    `json:"onboard"`
	Offboard int    `json:"offboard"`
	Date     string `json:"date"`
}

type UserIdentifier struct {
	Chain   string `json:"chain,omitempty"`
	Address string `json:"address,omitempty"`
}

type PATTokens struct {
	Name        string    `json:"name"`
	UUID        string    `json:"uuid"`
	Created     time.Time `json:"created"`
	Kind        string    `json:"kind"`
	Description string    `json:"description"`
}

type FCM struct {
	UserIdentifier
	DeviceID string `json:"device_id"`
}
