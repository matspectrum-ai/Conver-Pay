package domain

import "time"

type AttemptStatus string

const (
	AttemptStatusCreated        AttemptStatus = "created"
	AttemptStatusRequesting     AttemptStatus = "requesting"
	AttemptStatusSucceeded      AttemptStatus = "succeeded"
	AttemptStatusFailedSafe     AttemptStatus = "failed_safe"
	AttemptStatusFailedTerminal AttemptStatus = "failed_terminal"
	AttemptStatusUnknown        AttemptStatus = "unknown"
)

type PaymentAttempt struct {
	ID                   string
	PaymentIntentID      string
	ProviderConnectionID string
	Sequence             int
	Status               AttemptStatus
	ProviderPaymentID    string
	FailureCode          string
	ReconciliationStatus string
	Pix                  *Pix
	RequestStartedAt     time.Time
	ResponseReceivedAt   *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
