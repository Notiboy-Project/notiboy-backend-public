package entities

import (
	"encoding/json"
	"fmt"
)

type VerifyMedium struct {
	MediumAddress string `json:"medium_address"`
}

type TplRenderData struct {
	CallbackUrl       string `default:"#"`
	ButtonDescription string `default:"View More"`
	Message           string `default:"Welcome to Notiboy"`
}

type MediumMetadata struct {
	Email   *EmailMedium
	Discord *DiscordMedium
}

func (mm *MediumMetadata) Marshal() (string, error) {
	byteData, err := json.Marshal(mm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal medium metadata: %w", err)
	}

	return string(byteData), err
}

func (mm *MediumMetadata) Unmarshal(data string) error {
	err := json.Unmarshal([]byte(data), mm)
	if err != nil {
		return fmt.Errorf("unmarshal failed for medium metadata: %w", err)
	}

	return nil
}

type DiscordMedium struct {
	ID          string
	DMChannelID string
	Verified    bool
}
type EmailMedium struct {
	ID       string
	Verified bool
}
