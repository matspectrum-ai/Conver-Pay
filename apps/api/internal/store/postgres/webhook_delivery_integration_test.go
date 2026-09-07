package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	providerfake "github.com/matspectrum-ai/conver-pay/apps/api/internal/provider/fake"
	storepostgres "github.com/matspectrum-ai/conver-pay/apps/api/internal/store/postgres"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/webhookdelivery"
)

func TestPostgresWebhookDeliveryLeaseRetryAndEnvironment(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()

	resetDatabase(t, pool)
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE conver_pay.merchant_webhook_endpoints RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate webhook endpoints: %v", err)
	}
	seedProvider(t, pool, "conn_test", "provider_test", 1)

	store := storepostgres.New(pool)
	endpointCreated := time.Now().UTC().Add(-time.Minute)
	for _, endpoint := range []*webhookdelivery.Endpoint{
		{
			ID: "whe_test", WorkspaceID: "ws_1", Environment: domain.EnvironmentTest,
			URL: "https://test.example/hook", SigningSecretCiphertext: "cipher-test",
			Enabled: true, ActiveSince: endpointCreated, CreatedAt: endpointCreated, UpdatedAt: endpointCreated,
		},
		{
			ID: "whe_live", WorkspaceID: "ws_1", Environment: domain.EnvironmentLive,
			URL: "https://live.example/hook", SigningSecretCiphertext: "cipher-live",
			Enabled: true, ActiveSince: endpointCreated, CreatedAt: endpointCreated, UpdatedAt: endpointCreated,
		},
	} {
		if _, err := store.UpsertWebhookEndpoint(ctx, endpoint); err != nil {
			t.Fatalf("UpsertWebhookEndpoint() error = %v", err)
		}
	}

	adapter := providerfake.New("provider_test", []provider.CreateOutcome{{
		Kind: provider.CreateSucceeded, ProviderPaymentID: "provider-delivery-1", Pix: testPix("pix-delivery"),
	}}, nil)
	svc := orchestration.New(orchestration.Options{
		Repository: store,
		Providers: providerfake.Registry{"provider_test": adapter},
	})
	intent, err := svc.CreatePayment(ctx, testCreateRequest("idem-delivery"))
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	intent, err = svc.MarkPaidWithOutbox(ctx, intent.ID, intent.PresentedAttemptID)
	if err != nil {
		t.Fatalf("MarkPaidWithOutbox() error = %v", err)
	}
	if intent.Status != domain.PaymentStatusPaid {
		t.Fatalf("payment status = %s", intent.Status)
	}

	now := time.Now().UTC()
	materialized, err := store.MaterializePendingDeliveries(ctx, now, 10)
	if err != nil {
		t.Fatalf("MaterializePendingDeliveries() error = %v", err)
	}
	if materialized != 1 {
		t.Fatalf("materialized = %d, want 1", materialized)
	}

	jobs, err := store.ClaimDueDeliveries(ctx, now, now.Add(30*time.Second), "worker_a", 10)
	if err != nil {
		t.Fatalf("ClaimDueDeliveries(worker_a) error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].TargetURL != "https://test.example/hook" || jobs[0].SigningSecretCiphertext != "cipher-test" {
		t.Fatalf("jobs = %#v", jobs)
	}
	otherJobs, err := store.ClaimDueDeliveries(ctx, now.Add(time.Second), now.Add(31*time.Second), "worker_b", 10)
	if err != nil {
		t.Fatalf("ClaimDueDeliveries(worker_b) error = %v", err)
	}
	if len(otherJobs) != 0 {
		t.Fatalf("worker_b claimed leased jobs = %#v", otherJobs)
	}

	status500 := 500
	retryAt := now.Add(time.Minute)
	if err := store.CompleteDelivery(ctx, "worker_a", webhookdelivery.Completion{
		DeliveryID: jobs[0].DeliveryID, AttemptID: "wha_1", Sequence: 1,
		Status: webhookdelivery.DeliveryRetry, StartedAt: now, CompletedAt: now.Add(20 * time.Millisecond),
		HTTPStatus: &status500, Latency: 20 * time.Millisecond, NextAttemptAt: retryAt,
	}); err != nil {
		t.Fatalf("CompleteDelivery(retry) error = %v", err)
	}

	jobs, err = store.ClaimDueDeliveries(ctx, retryAt.Add(time.Millisecond), retryAt.Add(30*time.Second), "worker_b", 10)
	if err != nil {
		t.Fatalf("ClaimDueDeliveries(retry) error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].AttemptCount != 1 {
		t.Fatalf("retry jobs = %#v", jobs)
	}
	status204 := 204
	deliveredAt := retryAt.Add(10 * time.Millisecond)
	if err := store.CompleteDelivery(ctx, "worker_b", webhookdelivery.Completion{
		DeliveryID: jobs[0].DeliveryID, AttemptID: "wha_2", Sequence: 2,
		Status: webhookdelivery.DeliverySucceeded, StartedAt: retryAt,
		CompletedAt: deliveredAt, HTTPStatus: &status204, Latency: 10 * time.Millisecond,
		NextAttemptAt: deliveredAt, DeliveredAt: &deliveredAt,
	}); err != nil {
		t.Fatalf("CompleteDelivery(success) error = %v", err)
	}

	var deliveryStatus string
	var attemptCount, historyCount int
	if err := pool.QueryRow(ctx, `
		SELECT status, attempt_count FROM conver_pay.webhook_deliveries WHERE id=$1
	`, jobs[0].DeliveryID).Scan(&deliveryStatus, &attemptCount); err != nil {
		t.Fatalf("query delivery: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM conver_pay.webhook_delivery_attempts WHERE delivery_id=$1
	`, jobs[0].DeliveryID).Scan(&historyCount); err != nil {
		t.Fatalf("query attempt history: %v", err)
	}
	if deliveryStatus != webhookdelivery.DeliverySucceeded || attemptCount != 2 || historyCount != 2 {
		t.Fatalf("status=%s attempt_count=%d history=%d", deliveryStatus, attemptCount, historyCount)
	}
}
