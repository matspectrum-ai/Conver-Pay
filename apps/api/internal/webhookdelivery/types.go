package webhookdelivery

import (
	"context"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type Endpoint struct {
	ID                      string
	WorkspaceID             string
	Environment             domain.Environment
	URL                     string
	SigningSecretCiphertext string
	Enabled                 bool
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type Job struct {
	DeliveryID               string
	MerchantEventID           string
	EventType                 string
	Payload                   []byte
	TargetURL                 string
	SigningSecretCiphertext   string
	AttemptCount              int
}

type Completion struct {
	DeliveryID    string
	AttemptID     string
	Sequence      int
	Status        string
	StartedAt     time.Time
	CompletedAt   time.Time
	HTTPStatus    *int
	Latency       time.Duration
	ErrorCode     string
	NextAttemptAt time.Time
	DeliveredAt   *time.Time
}

const (
	DeliveryPending   = "pending"
	DeliveryRetry     = "retry"
	DeliverySucceeded = "succeeded"
	DeliveryFailed    = "failed"
)

type Repository interface {
	UpsertWebhookEndpoint(context.Context, *Endpoint) (*Endpoint, error)
	GetWebhookEndpoint(context.Context, string, domain.Environment) (*Endpoint, error)
	DisableWebhookEndpoint(context.Context, string, domain.Environment, time.Time) error
	MaterializePendingDeliveries(context.Context, time.Time, int) (int64, error)
	ClaimDueDeliveries(context.Context, time.Time, time.Time, string, int) ([]Job, error)
	CompleteDelivery(context.Context, string, Completion) error
}

type SecretCipher interface {
	Encrypt([]byte) (string, error)
	Decrypt(string) ([]byte, error)
}

type Sender interface {
	Send(context.Context, SendRequest) SendResult
}

type SendRequest struct {
	EventID string
	URL     string
	Payload []byte
	Secret  []byte
	Now     time.Time
}

type SendResult struct {
	HTTPStatus *int
	Latency    time.Duration
	ErrorCode  string
}
