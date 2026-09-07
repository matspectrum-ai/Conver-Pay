package orchestration_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	providerfake "github.com/matspectrum-ai/conver-pay/apps/api/internal/provider/fake"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/store/memory"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time {
	current := c.now
	c.now = c.now.Add(time.Millisecond)
	return current
}

type sequenceIDs struct{ n int }

func (g *sequenceIDs) New(prefix string) string {
	g.n++
	return fmt.Sprintf("%s_%03d", prefix, g.n)
}

type fixture struct {
	store *memory.Store
	svc   *orchestration.Service
	a     *providerfake.Adapter
	b     *providerfake.Adapter
}

func newFixture(aCreates []provider.CreateOutcome, aReconcile []provider.ReconcileOutcome, bCreates []provider.CreateOutcome) fixture {
	store := memory.New()
	store.AddProviderConnection(domain.ProviderConnection{
		ID: "conn_a", WorkspaceID: "ws_1", ProviderKey: "provider_a", Environment: domain.EnvironmentTest,
		Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 1,
	})
	store.AddProviderConnection(domain.ProviderConnection{
		ID: "conn_b", WorkspaceID: "ws_1", ProviderKey: "provider_b", Environment: domain.EnvironmentTest,
		Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 2,
	})
	a := providerfake.New("provider_a", aCreates, aReconcile)
	b := providerfake.New("provider_b", bCreates, nil)
	svc := orchestration.New(orchestration.Options{
		Repository: store,
		Providers: providerfake.Registry{
			"provider_a": a,
			"provider_b": b,
		},
		Clock: &fakeClock{now: time.Date(2026, 9, 7, 5, 0, 0, 0, time.UTC)},
		IDs:   &sequenceIDs{},
	})
	return fixture{store: store, svc: svc, a: a, b: b}
}

func createRequest() orchestration.CreatePaymentRequest {
	return orchestration.CreatePaymentRequest{
		WorkspaceID: "ws_1", IdempotencyKey: "idem_1", MerchantOrderID: "order_1",
		Amount: 5000, Currency: "BRL", Environment: domain.EnvironmentTest,
		Metadata: map[string]string{"source": "checkout"},
	}
}

func pix(code string) *domain.Pix {
	return &domain.Pix{CopyPaste: code, ExpiresAt: time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC)}
}

func TestAC001NormalPaymentSucceeds(t *testing.T) {
	fx := newFixture([]provider.CreateOutcome{{Kind: provider.CreateSucceeded, ProviderPaymentID: "a_1", Pix: pix("pix-a")}}, nil, nil)

	intent, err := fx.svc.CreatePayment(context.Background(), createRequest())
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if intent.Status != domain.PaymentStatusAwaitingPayment {
		t.Fatalf("status = %s", intent.Status)
	}
	if intent.Pix == nil || intent.Pix.CopyPaste != "pix-a" {
		t.Fatalf("unexpected pix: %#v", intent.Pix)
	}
	if fx.a.CreateCalls() != 1 || fx.b.CreateCalls() != 0 {
		t.Fatalf("create calls A=%d B=%d", fx.a.CreateCalls(), fx.b.CreateCalls())
	}
	attempts, _ := fx.store.ListAttempts(context.Background(), intent.ID)
	decisions, _ := fx.store.ListRoutingDecisions(context.Background(), intent.ID)
	if len(attempts) != 1 || len(decisions) != 1 {
		t.Fatalf("attempts=%d decisions=%d", len(attempts), len(decisions))
	}
}

func TestAC002SafeFailureFallsBack(t *testing.T) {
	fx := newFixture(
		[]provider.CreateOutcome{{Kind: provider.CreateFailedSafe, FailureCode: "provider_unavailable"}},
		nil,
		[]provider.CreateOutcome{{Kind: provider.CreateSucceeded, ProviderPaymentID: "b_1", Pix: pix("pix-b")}},
	)

	intent, err := fx.svc.CreatePayment(context.Background(), createRequest())
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if intent.Status != domain.PaymentStatusAwaitingPayment || intent.Pix.CopyPaste != "pix-b" {
		t.Fatalf("unexpected intent: %#v", intent)
	}
	attempts, _ := fx.store.ListAttempts(context.Background(), intent.ID)
	if len(attempts) != 2 || attempts[0].Status != domain.AttemptStatusFailedSafe || attempts[1].Status != domain.AttemptStatusSucceeded {
		t.Fatalf("unexpected attempts: %#v", attempts)
	}
}

func TestAC003TimeoutDoesNotImmediatelyFallback(t *testing.T) {
	fx := newFixture([]provider.CreateOutcome{{Kind: provider.CreateUnknown, FailureCode: "timeout"}}, nil,
		[]provider.CreateOutcome{{Kind: provider.CreateSucceeded, Pix: pix("pix-b")}})

	intent, err := fx.svc.CreatePayment(context.Background(), createRequest())
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if intent.Status != domain.PaymentStatusReconciling {
		t.Fatalf("status = %s", intent.Status)
	}
	if fx.a.CreateCalls() != 1 || fx.b.CreateCalls() != 0 {
		t.Fatalf("unsafe fallback calls A=%d B=%d", fx.a.CreateCalls(), fx.b.CreateCalls())
	}
}

func TestAC004UnknownReconcilesNotCreatedThenFallsBack(t *testing.T) {
	fx := newFixture(
		[]provider.CreateOutcome{{Kind: provider.CreateUnknown, FailureCode: "timeout"}},
		[]provider.ReconcileOutcome{{Kind: provider.ReconcileNotCreated, FailureCode: "confirmed_not_created"}},
		[]provider.CreateOutcome{{Kind: provider.CreateSucceeded, ProviderPaymentID: "b_1", Pix: pix("pix-b")}},
	)

	intent, err := fx.svc.CreatePayment(context.Background(), createRequest())
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	intent, err = fx.svc.ReconcilePayment(context.Background(), intent.ID, domain.EnvironmentTest)
	if err != nil {
		t.Fatalf("ReconcilePayment() error = %v", err)
	}
	if intent.Status != domain.PaymentStatusAwaitingPayment || intent.Pix.CopyPaste != "pix-b" {
		t.Fatalf("unexpected intent: %#v", intent)
	}
	if fx.b.CreateCalls() != 1 {
		t.Fatalf("B calls = %d, want 1", fx.b.CreateCalls())
	}
}

func TestAC005UnknownReconcilesCreatedAndDoesNotCallB(t *testing.T) {
	fx := newFixture(
		[]provider.CreateOutcome{{Kind: provider.CreateUnknown, FailureCode: "timeout"}},
		[]provider.ReconcileOutcome{{Kind: provider.ReconcileCreated, ProviderPaymentID: "a_1", Pix: pix("pix-a")}},
		[]provider.CreateOutcome{{Kind: provider.CreateSucceeded, Pix: pix("pix-b")}},
	)

	intent, _ := fx.svc.CreatePayment(context.Background(), createRequest())
	intent, err := fx.svc.ReconcilePayment(context.Background(), intent.ID, domain.EnvironmentTest)
	if err != nil {
		t.Fatalf("ReconcilePayment() error = %v", err)
	}
	if intent.Status != domain.PaymentStatusAwaitingPayment || intent.Pix.CopyPaste != "pix-a" {
		t.Fatalf("unexpected intent: %#v", intent)
	}
	if fx.b.CreateCalls() != 0 {
		t.Fatalf("B calls = %d, want 0", fx.b.CreateCalls())
	}
}

func TestAC006IdempotentRetryDoesNotCallProviderAgain(t *testing.T) {
	fx := newFixture([]provider.CreateOutcome{{Kind: provider.CreateSucceeded, Pix: pix("pix-a")}}, nil, nil)
	req := createRequest()

	first, err := fx.svc.CreatePayment(context.Background(), req)
	if err != nil {
		t.Fatalf("first create error = %v", err)
	}
	second, err := fx.svc.CreatePayment(context.Background(), req)
	if err != nil {
		t.Fatalf("second create error = %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("ids differ: %s vs %s", first.ID, second.ID)
	}
	if fx.a.CreateCalls() != 1 {
		t.Fatalf("A calls = %d, want 1", fx.a.CreateCalls())
	}
}

func TestAC007IdempotencyConflict(t *testing.T) {
	fx := newFixture([]provider.CreateOutcome{{Kind: provider.CreateSucceeded, Pix: pix("pix-a")}}, nil, nil)
	req := createRequest()
	if _, err := fx.svc.CreatePayment(context.Background(), req); err != nil {
		t.Fatalf("first create error = %v", err)
	}
	req.Amount = 9000
	_, err := fx.svc.CreatePayment(context.Background(), req)
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("error = %v, want idempotency conflict", err)
	}
	if fx.a.CreateCalls() != 1 {
		t.Fatalf("A calls = %d, want 1", fx.a.CreateCalls())
	}
}

func TestAC010RecoveryCreatedOnlyAfterFallbackIsPaid(t *testing.T) {
	fx := newFixture(
		[]provider.CreateOutcome{{Kind: provider.CreateFailedSafe, FailureCode: "provider_unavailable"}}, nil,
		[]provider.CreateOutcome{{Kind: provider.CreateSucceeded, ProviderPaymentID: "b_1", Pix: pix("pix-b")}},
	)
	intent, err := fx.svc.CreatePayment(context.Background(), createRequest())
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	before, _ := fx.store.ListRecoveryEvents(context.Background(), intent.ID)
	if len(before) != 0 {
		t.Fatalf("recovery exists before payment: %#v", before)
	}

	intent, err = fx.svc.MarkPaid(context.Background(), intent.ID, intent.PresentedAttemptID)
	if err != nil {
		t.Fatalf("MarkPaid() error = %v", err)
	}
	if !intent.Recovered || intent.RecoveredAmount != 5000 {
		t.Fatalf("unexpected recovered state: %#v", intent)
	}
	after, _ := fx.store.ListRecoveryEvents(context.Background(), intent.ID)
	if len(after) != 1 {
		t.Fatalf("recovery count = %d, want 1", len(after))
	}

	if _, err := fx.svc.MarkPaid(context.Background(), intent.ID, intent.PresentedAttemptID); err != nil {
		t.Fatalf("duplicate MarkPaid() error = %v", err)
	}
	afterDuplicate, _ := fx.store.ListRecoveryEvents(context.Background(), intent.ID)
	if len(afterDuplicate) != 1 {
		t.Fatalf("duplicate recovery count = %d, want 1", len(afterDuplicate))
	}
}

func TestAC012CircuitOpenProviderSkipped(t *testing.T) {
	store := memory.New()
	store.AddProviderConnection(domain.ProviderConnection{ID: "conn_a", WorkspaceID: "ws_1", ProviderKey: "provider_a", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitOpen, Priority: 1})
	store.AddProviderConnection(domain.ProviderConnection{ID: "conn_b", WorkspaceID: "ws_1", ProviderKey: "provider_b", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 2})
	a := providerfake.New("provider_a", []provider.CreateOutcome{{Kind: provider.CreateSucceeded, Pix: pix("pix-a")}}, nil)
	b := providerfake.New("provider_b", []provider.CreateOutcome{{Kind: provider.CreateSucceeded, Pix: pix("pix-b")}}, nil)
	svc := orchestration.New(orchestration.Options{Repository: store, Providers: providerfake.Registry{"provider_a": a, "provider_b": b}, Clock: &fakeClock{now: time.Now().UTC()}, IDs: &sequenceIDs{}})

	intent, err := svc.CreatePayment(context.Background(), createRequest())
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if intent.Pix.CopyPaste != "pix-b" || a.CreateCalls() != 0 || b.CreateCalls() != 1 {
		t.Fatalf("unexpected routing: pix=%v A=%d B=%d", intent.Pix, a.CreateCalls(), b.CreateCalls())
	}
	decisions, _ := store.ListRoutingDecisions(context.Background(), intent.ID)
	if len(decisions) != 1 || len(decisions[0].Candidates) != 2 || decisions[0].Candidates[0].ExclusionReason != "circuit_open" {
		t.Fatalf("unexpected decision: %#v", decisions)
	}
}
