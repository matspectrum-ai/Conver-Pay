package webhookdelivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	New(prefix string) string
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type randomIDs struct{}

func (randomIDs) New(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

type Options struct {
	Repository                Repository
	Cipher                    SecretCipher
	Sender                    Sender
	Clock                     Clock
	IDs                       IDGenerator
	WorkerID                  string
	BatchSize                 int
	LeaseDuration             time.Duration
	MaxAttempts               int
	AllowInsecureLocalTargets bool
}

type Service struct {
	repo                      Repository
	cipher                    SecretCipher
	sender                    Sender
	clock                     Clock
	ids                       IDGenerator
	workerID                  string
	batchSize                 int
	leaseDuration             time.Duration
	maxAttempts               int
	allowInsecureLocalTargets bool
}

func New(opts Options) (*Service, error) {
	if opts.Repository == nil {
		return nil, fmt.Errorf("webhook repository is required")
	}
	if opts.Cipher == nil {
		return nil, fmt.Errorf("webhook secret cipher is required")
	}
	clock := opts.Clock
	if clock == nil {
		clock = realClock{}
	}
	ids := opts.IDs
	if ids == nil {
		ids = randomIDs{}
	}
	workerID := strings.TrimSpace(opts.WorkerID)
	if workerID == "" {
		workerID = ids.New("worker")
	}
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 25
	}
	lease := opts.LeaseDuration
	if lease <= 0 {
		lease = 30 * time.Second
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 12
	}
	return &Service{
		repo: opts.Repository, cipher: opts.Cipher, sender: opts.Sender,
		clock: clock, ids: ids, workerID: workerID, batchSize: batchSize,
		leaseDuration: lease, maxAttempts: maxAttempts,
		allowInsecureLocalTargets: opts.AllowInsecureLocalTargets,
	}, nil
}

func (s *Service) SetEndpoint(ctx context.Context, workspaceID string, environment domain.Environment, rawURL, signingSecret string) (*Endpoint, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	if environment != domain.EnvironmentTest && environment != domain.EnvironmentLive {
		return nil, fmt.Errorf("invalid environment")
	}
	url, err := validateEndpointURL(rawURL, s.allowInsecureLocalTargets)
	if err != nil {
		return nil, err
	}
	if len([]byte(signingSecret)) < 32 {
		return nil, fmt.Errorf("signing secret must be at least 32 bytes")
	}
	ciphertext, err := s.cipher.Encrypt([]byte(signingSecret))
	if err != nil {
		return nil, fmt.Errorf("encrypt webhook signing secret: %w", err)
	}

	now := s.clock.Now()
	endpoint := &Endpoint{
		ID: s.ids.New("whe"), WorkspaceID: workspaceID, Environment: environment,
		URL: url, SigningSecretCiphertext: ciphertext, Enabled: true,
		ActiveSince: now, CreatedAt: now, UpdatedAt: now,
	}
	if existing, err := s.repo.GetWebhookEndpoint(ctx, workspaceID, environment); err == nil {
		endpoint.ID = existing.ID
		endpoint.CreatedAt = existing.CreatedAt
		if existing.Enabled {
			endpoint.ActiveSince = existing.ActiveSince
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	return s.repo.UpsertWebhookEndpoint(ctx, endpoint)
}

func (s *Service) GetEndpoint(ctx context.Context, workspaceID string, environment domain.Environment) (*Endpoint, error) {
	return s.repo.GetWebhookEndpoint(ctx, workspaceID, environment)
}

func (s *Service) DisableEndpoint(ctx context.Context, workspaceID string, environment domain.Environment) error {
	return s.repo.DisableWebhookEndpoint(ctx, workspaceID, environment, s.clock.Now())
}

func (s *Service) RunOnce(ctx context.Context) (int, error) {
	now := s.clock.Now()
	if _, err := s.repo.MaterializePendingDeliveries(ctx, now, s.batchSize*4); err != nil {
		return 0, err
	}
	jobs, err := s.repo.ClaimDueDeliveries(ctx, now, now.Add(s.leaseDuration), s.workerID, s.batchSize)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, job := range jobs {
		if err := s.processJob(ctx, job); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (s *Service) processJob(ctx context.Context, job Job) error {
	startedAt := s.clock.Now()
	sequence := job.AttemptCount + 1
	secret, decryptErr := s.cipher.Decrypt(job.SigningSecretCiphertext)

	result := SendResult{}
	if decryptErr != nil {
		result.ErrorCode = "secret_decryption_failed"
	} else if s.sender == nil {
		result.ErrorCode = "sender_not_configured"
	} else {
		result = s.sender.Send(ctx, SendRequest{
			EventID: job.MerchantEventID,
			URL: job.TargetURL,
			Payload: append([]byte(nil), job.Payload...),
			Secret: secret,
			Now: startedAt,
		})
	}
	completedAt := s.clock.Now()
	if result.Latency <= 0 {
		result.Latency = completedAt.Sub(startedAt)
	}

	status := classifyDelivery(result)
	var deliveredAt *time.Time
	nextAttemptAt := completedAt
	if status == DeliverySucceeded {
		t := completedAt
		deliveredAt = &t
	} else if status == DeliveryRetry {
		if sequence >= s.maxAttempts {
			status = DeliveryFailed
		} else {
			nextAttemptAt = completedAt.Add(retryDelay(job.DeliveryID, sequence))
		}
	}

	completion := Completion{
		DeliveryID: job.DeliveryID,
		AttemptID: s.ids.New("wha"),
		Sequence: sequence,
		Status: status,
		StartedAt: startedAt,
		CompletedAt: completedAt,
		HTTPStatus: result.HTTPStatus,
		Latency: result.Latency,
		ErrorCode: result.ErrorCode,
		NextAttemptAt: nextAttemptAt,
		DeliveredAt: deliveredAt,
	}
	return s.repo.CompleteDelivery(ctx, s.workerID, completion)
}

func classifyDelivery(result SendResult) string {
	if result.HTTPStatus != nil {
		status := *result.HTTPStatus
		if status >= 200 && status < 300 {
			return DeliverySucceeded
		}
		if status == 408 || status == 425 || status == 429 || status >= 500 {
			return DeliveryRetry
		}
		return DeliveryFailed
	}
	return DeliveryRetry
}
