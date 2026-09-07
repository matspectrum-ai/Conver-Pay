package domain

import "time"

type ProviderEvent struct {
	ID                   string
	ProviderConnectionID string
	ExternalEventID      string
	EventType            string
	ProviderPaymentID    string
	PayloadHash          string
	ReceivedAt           time.Time
	ProcessedAt          *time.Time
}

type MerchantEvent struct {
	ID              string
	WorkspaceID     string
	EventKey        string
	EventType       string
	PaymentIntentID string
	Payload         []byte
	CreatedAt       time.Time
}
