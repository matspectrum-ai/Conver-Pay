package domain

import "time"

type RecoveryEvent struct {
	ID                  string
	PaymentIntentID     string
	FailedAttemptID     string
	SuccessfulAttemptID string
	FailureReason       string
	Amount              int64
	CreatedAt           time.Time
}
