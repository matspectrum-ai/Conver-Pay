package domain

import "time"

type PaymentStatus string

const (
	PaymentStatusCreated         PaymentStatus = "created"
	PaymentStatusRouting         PaymentStatus = "routing"
	PaymentStatusReconciling     PaymentStatus = "reconciling"
	PaymentStatusAwaitingPayment PaymentStatus = "awaiting_payment"
	PaymentStatusPaid            PaymentStatus = "paid"
	PaymentStatusFailed          PaymentStatus = "failed"
	PaymentStatusExpired         PaymentStatus = "expired"
	PaymentStatusCancelled       PaymentStatus = "cancelled"
)

type Pix struct {
	CopyPaste string
	ExpiresAt time.Time
}

type PaymentIntent struct {
	ID                 string
	WorkspaceID        string
	MerchantOrderID    string
	IdempotencyKey     string
	RequestFingerprint string
	Amount             int64
	Currency           string
	Metadata           map[string]string
	Status             PaymentStatus
	ActiveAttemptID    string
	PresentedAttemptID string
	Pix                *Pix
	Recovered          bool
	RecoveredAmount    int64
	FailureCode        string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	PaidAt             *time.Time
}
