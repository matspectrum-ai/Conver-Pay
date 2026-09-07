package postgres_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	providerfake "github.com/matspectrum-ai/conver-pay/apps/api/internal/provider/fake"
	storepostgres "github.com/matspectrum-ai/conver-pay/apps/api/internal/store/postgres"
)

func TestPostgresOrchestrationPersistence(t *testing.T) {
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
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("database ping error = %v", err)
	}

	store := storepostgres.New(pool)

	t.Run("concurrent idempotent creates call provider once", func(t *testing.T) {
		resetDatabase(t, pool)
		seedProvider(t, pool, "conn_a", "provider_a", 1)

		adapter := providerfake.New("provider_a", []provider.CreateOutcome{{
			Kind: provider.CreateSucceeded, ProviderPaymentID: "provider-payment-1", Pix: testPix("pix-a"),
		}}, nil)
		svc := orchestration.New(orchestration.Options{
			Repository: store,
			Providers:  providerfake.Registry{"provider_a": adapter},
		})

		const workers = 16
		results := make(chan *domain.PaymentIntent, workers)
		errs := make(chan error, workers)
		var wg sync.WaitGroup
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				intent, err := svc.CreatePayment(ctx, testCreateRequest("idem-concurrent"))
				if err != nil {
					errs <- err
					return
				}
				results <- intent
			}()
		}
		wg.Wait()
		close(results)
		close(errs)

		for err := range errs {
			t.Errorf("CreatePayment() error = %v", err)
		}
		var paymentID string
		for intent := range results {
			if paymentID == "" {
				paymentID = intent.ID
			}
			if intent.ID != paymentID {
				t.Errorf("payment ID = %s, want %s", intent.ID, paymentID)
			}
		}
		if adapter.CreateCalls() != 1 {
			t.Fatalf("provider create calls = %d, want 1", adapter.CreateCalls())
		}
		stored, err := store.GetPayment(ctx, paymentID)
		if err != nil {
			t.Fatalf("GetPayment() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusAwaitingPayment {
			t.Fatalf("stored status = %s", stored.Status)
		}
	})

	t.Run("safe fallback and recovery survive reload", func(t *testing.T) {
		resetDatabase(t, pool)
		seedProvider(t, pool, "conn_a", "provider_a", 1)
		seedProvider(t, pool, "conn_b", "provider_b", 2)

		a := providerfake.New("provider_a", []provider.CreateOutcome{{Kind: provider.CreateFailedSafe, FailureCode: "provider_unavailable"}}, nil)
		b := providerfake.New("provider_b", []provider.CreateOutcome{{Kind: provider.CreateSucceeded, ProviderPaymentID: "b-1", Pix: testPix("pix-b")}}, nil)
		svc := orchestration.New(orchestration.Options{
			Repository: store,
			Providers:  providerfake.Registry{"provider_a": a, "provider_b": b},
		})

		intent, err := svc.CreatePayment(ctx, testCreateRequest("idem-fallback"))
		if err != nil {
			t.Fatalf("CreatePayment() error = %v", err)
		}
		if intent.Status != domain.PaymentStatusAwaitingPayment || intent.Pix == nil || intent.Pix.CopyPaste != "pix-b" {
			t.Fatalf("unexpected intent = %#v", intent)
		}
		attempts, err := store.ListAttempts(ctx, intent.ID)
		if err != nil {
			t.Fatalf("ListAttempts() error = %v", err)
		}
		decisions, err := store.ListRoutingDecisions(ctx, intent.ID)
		if err != nil {
			t.Fatalf("ListRoutingDecisions() error = %v", err)
		}
		if len(attempts) != 2 || len(decisions) != 2 {
			t.Fatalf("attempts=%d decisions=%d", len(attempts), len(decisions))
		}

		intent, err = svc.MarkPaid(ctx, intent.ID, intent.PresentedAttemptID)
		if err != nil {
			t.Fatalf("MarkPaid() error = %v", err)
		}
		if !intent.Recovered || intent.RecoveredAmount != 5000 {
			t.Fatalf("unexpected recovery state = %#v", intent)
		}

		reloaded, err := store.GetPayment(ctx, intent.ID)
		if err != nil {
			t.Fatalf("GetPayment() error = %v", err)
		}
		recoveries, err := store.ListRecoveryEvents(ctx, intent.ID)
		if err != nil {
			t.Fatalf("ListRecoveryEvents() error = %v", err)
		}
		if reloaded.Status != domain.PaymentStatusPaid || !reloaded.Recovered || len(recoveries) != 1 {
			t.Fatalf("reloaded=%#v recoveries=%#v", reloaded, recoveries)
		}
	})

	t.Run("unknown blocks fallback until reconciliation proves not created", func(t *testing.T) {
		resetDatabase(t, pool)
		seedProvider(t, pool, "conn_a", "provider_a", 1)
		seedProvider(t, pool, "conn_b", "provider_b", 2)

		a := providerfake.New("provider_a",
			[]provider.CreateOutcome{{Kind: provider.CreateUnknown, FailureCode: "timeout"}},
			[]provider.ReconcileOutcome{{Kind: provider.ReconcileNotCreated, FailureCode: "confirmed_not_created"}},
		)
		b := providerfake.New("provider_b", []provider.CreateOutcome{{Kind: provider.CreateSucceeded, ProviderPaymentID: "b-2", Pix: testPix("pix-b")}}, nil)
		svc := orchestration.New(orchestration.Options{
			Repository: store,
			Providers:  providerfake.Registry{"provider_a": a, "provider_b": b},
		})

		intent, err := svc.CreatePayment(ctx, testCreateRequest("idem-unknown"))
		if err != nil {
			t.Fatalf("CreatePayment() error = %v", err)
		}
		if intent.Status != domain.PaymentStatusReconciling || b.CreateCalls() != 0 {
			t.Fatalf("unsafe pre-reconciliation fallback: status=%s B=%d", intent.Status, b.CreateCalls())
		}

		intent, err = svc.ReconcilePayment(ctx, intent.ID, domain.EnvironmentTest)
		if err != nil {
			t.Fatalf("ReconcilePayment() error = %v", err)
		}
		if intent.Status != domain.PaymentStatusAwaitingPayment || b.CreateCalls() != 1 {
			t.Fatalf("unexpected reconciled state: status=%s B=%d", intent.Status, b.CreateCalls())
		}
	})
}

func resetDatabase(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		TRUNCATE TABLE
			conver_pay.recovery_events,
			conver_pay.routing_decisions,
			conver_pay.payment_attempts,
			conver_pay.payment_intents,
			conver_pay.provider_connections
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate database: %v", err)
	}
}

func seedProvider(t *testing.T, pool *pgxpool.Pool, id, providerKey string, priority int) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO conver_pay.provider_connections (
			id, workspace_id, provider_key, environment, enabled,
			credentials_valid, circuit_state, priority
		) VALUES ($1, 'ws_1', $2, 'test', true, true, 'closed', $3)`, id, providerKey, priority)
	if err != nil {
		t.Fatalf("seed provider %s: %v", id, err)
	}
}

func testCreateRequest(idempotencyKey string) orchestration.CreatePaymentRequest {
	return orchestration.CreatePaymentRequest{
		WorkspaceID: "ws_1", IdempotencyKey: idempotencyKey, MerchantOrderID: "order_1",
		Amount: 5000, Currency: "BRL", Environment: domain.EnvironmentTest,
		Metadata: map[string]string{"source": "integration-test"},
	}
}

func testPix(code string) *domain.Pix {
	return &domain.Pix{CopyPaste: code, ExpiresAt: time.Now().UTC().Add(time.Hour)}
}
