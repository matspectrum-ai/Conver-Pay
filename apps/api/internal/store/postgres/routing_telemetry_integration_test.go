package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	providerfake "github.com/matspectrum-ai/conver-pay/apps/api/internal/provider/fake"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/routingtelemetry"
	storepostgres "github.com/matspectrum-ai/conver-pay/apps/api/internal/store/postgres"
)

func TestPostgresHealthScoreDrivesAndPersistsRoutingDecision(t *testing.T) {
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
	seedProvider(t, pool, "conn_a", "provider_a", 1)
	seedProvider(t, pool, "conn_b", "provider_b", 2)

	now := time.Now().UTC().Truncate(time.Millisecond)
	seedHealthHistory(t, pool, "conn_a", now, 20, 10, 8, 2, 900*time.Millisecond)
	seedHealthHistory(t, pool, "conn_b", now, 20, 19, 1, 0, 180*time.Millisecond)

	store := storepostgres.New(pool)
	telemetry := routingtelemetry.New(store, routingtelemetry.Options{Window: 15 * time.Minute})
	a := providerfake.New("provider_a", []provider.CreateOutcome{{Kind: provider.CreateSucceeded, ProviderPaymentID: "a-new", Pix: testPix("pix-a")}}, nil)
	b := providerfake.New("provider_b", []provider.CreateOutcome{{Kind: provider.CreateSucceeded, ProviderPaymentID: "b-new", Pix: testPix("pix-b")}}, nil)
	svc := orchestration.New(orchestration.Options{
		Repository:       store,
		Providers:        providerfake.Registry{"provider_a": a, "provider_b": b},
		RoutingTelemetry: telemetry,
	})

	intent, err := svc.CreatePayment(ctx, testCreateRequest("idem-health-routing"))
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if intent.Pix == nil || intent.Pix.CopyPaste != "pix-b" || a.CreateCalls() != 0 || b.CreateCalls() != 1 {
		t.Fatalf("routing result pix=%#v A=%d B=%d", intent.Pix, a.CreateCalls(), b.CreateCalls())
	}

	decisions, err := store.ListRoutingDecisions(ctx, intent.ID)
	if err != nil {
		t.Fatalf("ListRoutingDecisions() error = %v", err)
	}
	if len(decisions) != 1 || decisions[0].SelectedProviderConnectionID != "conn_b" || decisions[0].ScoreVersion != domain.RoutingScoreVersion {
		t.Fatalf("decisions = %#v", decisions)
	}
	if len(decisions[0].Candidates) != 2 {
		t.Fatalf("candidates = %#v", decisions[0].Candidates)
	}
	var aScore, bScore float64
	for _, candidate := range decisions[0].Candidates {
		switch candidate.ProviderConnectionID {
		case "conn_a":
			aScore = candidate.Score
		case "conn_b":
			bScore = candidate.Score
		}
	}
	if bScore <= aScore || bScore < 0.85 {
		t.Fatalf("scores A=%f B=%f", aScore, bScore)
	}

	snapshots, err := store.ListRecentProviderHealthSnapshots(ctx, "conn_b", 1)
	if err != nil || len(snapshots) != 1 || snapshots[0].SampleCount != 20 || snapshots[0].QRSuccessCount != 19 {
		t.Fatalf("snapshots=%#v err=%v", snapshots, err)
	}
}

func seedHealthHistory(t *testing.T, pool *pgxpool.Pool, connectionID string, now time.Time, total, succeeded, failed, unknown int, latency time.Duration) {
	t.Helper()
	if succeeded+failed+unknown != total {
		t.Fatal("invalid health history counts")
	}
	for i := 0; i < total; i++ {
		paymentID := fmt.Sprintf("pi_hist_%s_%02d", connectionID, i)
		attemptID := fmt.Sprintf("pa_hist_%s_%02d", connectionID, i)
		started := now.Add(-10*time.Minute + time.Duration(i)*time.Second)
		status := domain.AttemptStatusSucceeded
		failureCode := ""
		switch {
		case i >= succeeded+failed:
			status = domain.AttemptStatusUnknown
			failureCode = "timeout"
		case i >= succeeded:
			status = domain.AttemptStatusFailedSafe
			failureCode = "provider_error"
		}
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO conver_pay.payment_intents (
				id, workspace_id, merchant_order_id, idempotency_key, request_fingerprint,
				amount, currency, status, created_at, updated_at
			) VALUES ($1,'ws_1',$2,$3,$3,1000,'BRL','failed',$4,$4)
		`, paymentID, "order-"+paymentID, "idem-"+paymentID, started); err != nil {
			t.Fatalf("insert historical payment: %v", err)
		}
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO conver_pay.payment_attempts (
				id, payment_intent_id, provider_connection_id, sequence, status,
				failure_code, request_started_at, response_received_at, created_at, updated_at
			) VALUES ($1,$2,$3,1,$4,$5,$6,$7,$6,$7)
		`, attemptID, paymentID, connectionID, status, failureCode, started, started.Add(latency)); err != nil {
			t.Fatalf("insert historical attempt: %v", err)
		}
	}
}
