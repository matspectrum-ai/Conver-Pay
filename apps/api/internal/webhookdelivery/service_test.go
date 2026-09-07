package webhookdelivery

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time {
	now := c.now
	c.now = c.now.Add(time.Millisecond)
	return now
}

type testIDs struct{ n int }

func (g *testIDs) New(prefix string) string {
	g.n++
	return fmt.Sprintf("%s_%03d", prefix, g.n)
}

type testCipher struct{}

func (testCipher) Encrypt(plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", errors.New("empty")
	}
	return "encrypted-secret", nil
}

func (testCipher) Decrypt(ciphertext string) ([]byte, error) {
	if ciphertext != "encrypted-secret" {
		return nil, errors.New("bad ciphertext")
	}
	return []byte("01234567890123456789012345678901"), nil
}

type testRepo struct {
	endpoint     *Endpoint
	jobs         []Job
	completion   *Completion
	materialized int64
}

func (r *testRepo) UpsertWebhookEndpoint(_ context.Context, endpoint *Endpoint) (*Endpoint, error) {
	copy := *endpoint
	r.endpoint = &copy
	return &copy, nil
}

func (r *testRepo) GetWebhookEndpoint(_ context.Context, workspaceID string, environment domain.Environment) (*Endpoint, error) {
	if r.endpoint == nil || r.endpoint.WorkspaceID != workspaceID || r.endpoint.Environment != environment {
		return nil, domain.ErrNotFound
	}
	copy := *r.endpoint
	return &copy, nil
}

func (r *testRepo) DisableWebhookEndpoint(_ context.Context, workspaceID string, environment domain.Environment, updatedAt time.Time) error {
	if r.endpoint == nil || r.endpoint.WorkspaceID != workspaceID || r.endpoint.Environment != environment {
		return domain.ErrNotFound
	}
	r.endpoint.Enabled = false
	r.endpoint.UpdatedAt = updatedAt
	return nil
}

func (r *testRepo) MaterializePendingDeliveries(_ context.Context, _ time.Time, _ int) (int64, error) {
	return r.materialized, nil
}

func (r *testRepo) ClaimDueDeliveries(_ context.Context, _, _ time.Time, _ string, _ int) ([]Job, error) {
	jobs := append([]Job(nil), r.jobs...)
	r.jobs = nil
	return jobs, nil
}

func (r *testRepo) CompleteDelivery(_ context.Context, _ string, completion Completion) error {
	copy := completion
	r.completion = &copy
	return nil
}

type testSender struct {
	result  SendResult
	request *SendRequest
}

func (s *testSender) Send(_ context.Context, request SendRequest) SendResult {
	copy := request
	copy.Payload = append([]byte(nil), request.Payload...)
	copy.Secret = append([]byte(nil), request.Secret...)
	s.request = &copy
	return s.result
}

func newTestService(t *testing.T, repo *testRepo, sender Sender, maxAttempts int) *Service {
	t.Helper()
	service, err := New(Options{
		Repository:                repo,
		Cipher:                    testCipher{},
		Sender:                    sender,
		Clock:                     &testClock{now: time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)},
		IDs:                       &testIDs{},
		WorkerID:                  "worker_test",
		MaxAttempts:               maxAttempts,
		AllowInsecureLocalTargets: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return service
}

func TestSetEndpointEncryptsSecretAndScopesEnvironment(t *testing.T) {
	repo := &testRepo{}
	service := newTestService(t, repo, nil, 0)
	endpoint, err := service.SetEndpoint(context.Background(), "ws_1", domain.EnvironmentTest,
		"http://127.0.0.1:8081/hook", "01234567890123456789012345678901")
	if err != nil {
		t.Fatalf("SetEndpoint() error = %v", err)
	}
	if endpoint.SigningSecretCiphertext != "encrypted-secret" || repo.endpoint.SigningSecretCiphertext != "encrypted-secret" {
		t.Fatalf("ciphertext not persisted: %#v", repo.endpoint)
	}
	if endpoint.Environment != domain.EnvironmentTest || !endpoint.Enabled {
		t.Fatalf("unexpected endpoint = %#v", endpoint)
	}
	if _, err := service.GetEndpoint(context.Background(), "ws_1", domain.EnvironmentLive); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("live GetEndpoint() error = %v", err)
	}
}

func TestRunOnceCompletesSuccessfulDelivery(t *testing.T) {
	status := 204
	repo := &testRepo{jobs: []Job{{
		DeliveryID: "whd_1", MerchantEventID: "evt_1", EventType: "payment.paid",
		Payload: []byte(`{"type":"payment.paid"}`), TargetURL: "https://merchant.example/hook",
		SigningSecretCiphertext: "encrypted-secret", AttemptCount: 0,
	}}}
	sender := &testSender{result: SendResult{HTTPStatus: &status, Latency: 25 * time.Millisecond}}
	service := newTestService(t, repo, sender, 12)
	processed, err := service.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 1 || repo.completion == nil {
		t.Fatalf("processed=%d completion=%#v", processed, repo.completion)
	}
	if repo.completion.Status != DeliverySucceeded || repo.completion.Sequence != 1 || repo.completion.DeliveredAt == nil {
		t.Fatalf("completion = %#v", repo.completion)
	}
	if sender.request == nil || sender.request.EventID != "evt_1" || string(sender.request.Secret) != "01234567890123456789012345678901" {
		t.Fatalf("sender request = %#v", sender.request)
	}
}

func TestRunOnceRetryAndTerminalClassification(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		attemptCount int
		wantStatus   string
		wantFuture   bool
	}{
		{name: "429 retries", status: 429, attemptCount: 0, wantStatus: DeliveryRetry, wantFuture: true},
		{name: "500 retries", status: 500, attemptCount: 2, wantStatus: DeliveryRetry, wantFuture: true},
		{name: "400 is terminal", status: 400, attemptCount: 0, wantStatus: DeliveryFailed},
		{name: "max attempts stops", status: 503, attemptCount: 11, wantStatus: DeliveryFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &testRepo{jobs: []Job{{
				DeliveryID: "whd_retry", MerchantEventID: "evt_retry", Payload: []byte(`{}`),
				TargetURL: "https://merchant.example/hook", SigningSecretCiphertext: "encrypted-secret",
				AttemptCount: tc.attemptCount,
			}}}
			sender := &testSender{result: SendResult{HTTPStatus: &tc.status, Latency: time.Millisecond}}
			service := newTestService(t, repo, sender, 12)
			if _, err := service.RunOnce(context.Background()); err != nil {
				t.Fatalf("RunOnce() error = %v", err)
			}
			if repo.completion == nil || repo.completion.Status != tc.wantStatus {
				t.Fatalf("completion = %#v, want status %s", repo.completion, tc.wantStatus)
			}
			if tc.wantFuture && !repo.completion.NextAttemptAt.After(repo.completion.CompletedAt) {
				t.Fatalf("next attempt = %s completed = %s", repo.completion.NextAttemptAt, repo.completion.CompletedAt)
			}
		})
	}
}
