package postgres_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	providerfake "github.com/matspectrum-ai/conver-pay/apps/api/internal/provider/fake"
	storepostgres "github.com/matspectrum-ai/conver-pay/apps/api/internal/store/postgres"
)

type integrationWebhookAdapter struct {
	*providerfake.Adapter
	secret string
}

func (a *integrationWebhookAdapter) ParseWebhook(_ context.Context, request provider.WebhookRequest) (provider.WebhookEvent, error) {
	values := request.Headers["X-Test-Signature"]
	if len(values) != 1 || values[0] != a.secret {
		return provider.WebhookEvent{}, provider.ErrInvalidWebhookSignature
	}
	var payload struct {
		EventID   string `json:"event_id"`
		Type      string `json:"type"`
		PaymentID string `json:"payment_id"`
	}
	if err := json.Unmarshal(request.Body, &payload); err != nil {
		return provider.WebhookEvent{}, provider.ErrUnsupportedWebhook
	}
	return provider.WebhookEvent{
		ExternalEventID:   payload.EventID,
		Type:              payload.Type,
		ProviderPaymentID: payload.PaymentID,
	}, nil
}

func TestPostgresProviderWebhookDedupeAndAtomicOutbox(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()

	resetDatabase(t, pool)
	seedProvider(t, pool, "conn_a", "provider_a", 1)
	seedProvider(t, pool, "conn_b", "provider_b", 2)

	a := providerfake.New("provider_a", []provider.CreateOutcome{{
		Kind: provider.CreateFailedSafe, FailureCode: "provider_unavailable",
	}}, nil)
	bBase := providerfake.New("provider_b", []provider.CreateOutcome{{
		Kind: provider.CreateSucceeded, ProviderPaymentID: "provider-payment-b", Pix: testPix("pix-b"),
	}}, nil)
	b := &integrationWebhookAdapter{Adapter: bBase, secret: "b-secret"}
	store := storepostgres.New(pool)
	svc := orchestration.New(orchestration.Options{
		Repository: store,
		Providers: providerfake.Registry{
			"provider_a": a,
			"provider_b": b,
		},
	})

	intent, err := svc.CreatePayment(ctx, testCreateRequest("idem-webhook-outbox"))
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if intent.Status != domain.PaymentStatusAwaitingPayment {
		t.Fatalf("pre-webhook status = %s", intent.Status)
	}

	request := provider.WebhookRequest{
		Headers: map[string][]string{"X-Test-Signature": {"b-secret"}},
		Body:    []byte(`{"event_id":"event-paid-b","type":"payment.paid","payment_id":"provider-payment-b"}`),
	}
	paid, err := svc.ProcessProviderWebhook(ctx, "conn_b", request)
	if err != nil {
		t.Fatalf("ProcessProviderWebhook() error = %v", err)
	}
	if paid.Status != domain.PaymentStatusPaid || !paid.Recovered || paid.RecoveredAmount != 5000 {
		t.Fatalf("paid state = %#v", paid)
	}

	if _, err := svc.ProcessProviderWebhook(ctx, "conn_b", request); err != nil {
		t.Fatalf("duplicate ProcessProviderWebhook() error = %v", err)
	}

	reloaded, err := store.GetPayment(ctx, paid.ID)
	if err != nil {
		t.Fatalf("GetPayment() error = %v", err)
	}
	recoveries, err := store.ListRecoveryEvents(ctx, paid.ID)
	if err != nil {
		t.Fatalf("ListRecoveryEvents() error = %v", err)
	}
	merchantEvents, err := store.ListMerchantEvents(ctx, "ws_1")
	if err != nil {
		t.Fatalf("ListMerchantEvents() error = %v", err)
	}
	if reloaded.Status != domain.PaymentStatusPaid || !reloaded.Recovered {
		t.Fatalf("reloaded payment = %#v", reloaded)
	}
	if len(recoveries) != 1 {
		t.Fatalf("recoveries = %#v", recoveries)
	}
	if len(merchantEvents) != 2 {
		t.Fatalf("merchant events = %#v", merchantEvents)
	}
	types := map[string]bool{}
	for _, event := range merchantEvents {
		types[event.EventType] = true
	}
	if !types["payment.paid"] || !types["payment.recovered"] {
		t.Fatalf("merchant event types = %#v", types)
	}

	var providerEventCount, processedCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE processed_at IS NOT NULL)
		FROM conver_pay.provider_events
		WHERE provider_connection_id='conn_b' AND external_event_id='event-paid-b'
	`).Scan(&providerEventCount, &processedCount); err != nil {
		t.Fatalf("query provider event ledger: %v", err)
	}
	if providerEventCount != 1 || processedCount != 1 {
		t.Fatalf("provider events count=%d processed=%d", providerEventCount, processedCount)
	}
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not configured")
	}
	return url
}
