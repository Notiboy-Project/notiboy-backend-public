package entities

import "time"

type BillingRequest struct {
	Chain         string   `json:"chain,omitempty"`
	Address       string   `json:"address,omitempty"`
	SignedTxn     string   `json:"signed_txn,omitempty"`
	TxnID         string   `json:"txn_id"`
	Membership    string   `json:"membership,omitempty"`
	TotalSent     int      `json:"total_sent"`
	Days          int      `json:"days"`
	OwnedChannels []string `json:"owned_channels"`
}

type BillingRecords struct {
	PaidAmount float64   `json:"paid_amount"`
	PaidTime   time.Time `json:"paid_time"`
	TxnID      string    `json:"txn_id"`
}
type BillingInfo struct {
	Address                string           `json:"address"`
	Chain                  string           `json:"chain"`
	Expiry                 time.Time        `json:"expiry"`
	LastUpdated            time.Time        `json:"last_updated"`
	Membership             string           `json:"membership"`
	Balance                string           `json:"balance"`
	RemainingNotifications int              `json:"remaining_notifications"`
	BillingRecords         []BillingRecords `json:"billing_records"`
	BlockedChannels        []string         `json:"blocked_channels"`
}
