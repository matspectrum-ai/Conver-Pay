package orchestration_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	providerfake "github.com/matspectrum-ai/conver-pay/apps/api/internal/provider/fake"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/store/memory"
)

type signedWebhookAdapter struct {
	*providerfake.Adapter
	secret string
}

func (a *signedWebhookAdapter) ParseWebhook(_ context.Context, request provider.WebhookRequest) (provider.WebhookEvent, error) {
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

func TestProviderWebhookMarksPaymentPaidExactlyOnce(t *testing.T) {
	store := memory.New()
	store.AddProviderConnection(domain.ProviderConnection{
		ID: "conn_a", WorkspaceID: "ws_1", ProviderKey: "provider_a", Environment: domain.EnvironmentTest,
		Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 1,
	})
	base := providerfake.New("provider_a", []provider.CreateOutcome{{
		Kind: provider.CreateSucceeded, ProviderPaymentID: "provider_payment_1",
		Pix: &domain.Pix{CopyPaste: "pix-a", ExpiresAt: time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC)},
	}}, nil)
	adapter := &signedWebhookAdapter{Adapter: base, secret: "valid-secret"}
	svc := orchestration.New(orchestration.Options{
		Repository: store,
		Providers:  providerfake.Registry{"provider_a": adapter},
		Clock:      &fakeClock{now: time.Date(2026, 9, 7, 5, 0, 0, 0, time.UTC)},
		IDs:        &sequenceIDs{},
	})

	_, err := svc.CreatePayment(context.Background(), orchestration.CreatePaymentRequest{
		WorkspaceID: "ws_1", IdempotencyKey: "idem-1", MerchantOrderID: "order-1",
		Amount: 5000, Currency: "BRL", Environment: domain.EnvironmentTest,
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}

	request := provider.WebhookRequest{
		Headers: map[string][]string{"X-Test-Signature": {"valid-secret"}},
		Body:    []byte(`{"event_id":"event-1","type":"payment.paid","payment_id":"provider_payment_1"}`),
	}
	paid, err := svc.ProcessProviderWebhook(context.Background(), "conn_a", request)
	if err != nil {
		t.Fatalf("ProcessProviderWebhook() error = %v", err)
	}
	if paid.Status != domain.PaymentStatusPaid || paid.Recovered {
		t.Fatalf("paid intent = %#v", paid)
	}

	events, err := store.ListMerchantEvents(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("ListMerchantEvents() error = %v", err)
	}
	if len(events) != 1 || events[0].EventType != "payment.paid" {
		t.Fatalf("merchant events = %#v", events)
	}
	firstEventID := events[0].ID

	paid, err = svc.ProcessProviderWebhook(context.Background(), "conn_a", request)
	if err != nil {
		t.Fatalf("duplicate ProcessProviderWebhook() error = %v", err)
	}
	if paid.Status != domain.PaymentStatusPaid {
		t.Fatalf("duplicate paid status = %s", paid.Status)
	}
	events, _ = store.ListMerchantEvents(context.Background(), "ws_1")
	if len(events) != 1 || events[0].ID != firstEventID {
		t.Fatalf("duplicate merchant events = %#v", events)
	}
}

func TestProviderWebhookRecoveryCreatesTwoStableMerchantEvents(t *testing.T) {
	store := memory.New()
	store.AddProviderConnection(domain.ProviderConnection{
		ID: "conn_a", WorkspaceID: "ws_1", ProviderKey: "provider_a", Environment: domain.EnvironmentTest,
		Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 1,
	})
	store.AddProviderConnection(domain.ProviderConnection{
		ID: "conn_b", WorkspaceID: "ws_1", ProviderKey: "provider_b", Environment: domain.EnvironmentTest,
		Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 2,
	})

	a := providerfake.New("provider_a", []provider.CreateOutcome{{Kind: provider.CreateFailedSafe, FailureCode: "provider_unavailable"}}, nil)
	bBase := providerfake.New("provider_b", []provider.CreateOutcome{{
		Kind: provider.CreateSucceeded, ProviderPaymentID: "provider_payment_b",
		Pix: &domain.Pix{CopyPaste: "pix-b", ExpiresAt: time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC)},
	}}, nil)
	b := &signedWebhookAdapter{Adapter: bBase, secret: "b-secret"}
	svc := orchestration.New(orchestration.Options{
		Repository: store,
		Providers: providerfake.Registry{
			"provider_a": a,
			"provider_b": b,
		},
		Clock: &fakeClock{now: time.Date(2026, 9, 7, 5, 0, 0, 0, time.UTC)},
		IDs:   &sequenceIDs{},
	})

	intent, err := svc.CreatePayment(context.Background(), orchestration.CreatePaymentRequest{
		WorkspaceID: "ws_1", IdempotencyKey: "idem-recovery", MerchantOrderID: "order-recovery",
		Amount: 5000, Currency: "BRL", Environment: domain.EnvironmentTest,
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if intent.Status != domain.PaymentStatusAwaitingPayment {
		t.Fatalf("status = %s", intent.Status)
	}

	request := provider.WebhookRequest{
		Headers: map[string][]string{"X-Test-Signature": {"b-secret"}},
		Body:    []byte(`{"event_id":"event-b-paid","type":"payment.paid","payment_id":"provider_payment_b"}`),
	}
	paid, err := svc.ProcessProviderWebhook(context.Background(), "conn_b", request)
	if err != nil {
		t.Fatalf("ProcessProviderWebhook() error = %v", err)
	}
	if !paid.Recovered || paid.RecoveredAmount != 5000 {
		t.Fatalf("recovery state = %#v", paid)
	}

	recoveries, _ := store.ListRecoveryEvents(context.Background(), paid.ID)
	if len(recoveries) != 1 {
		t.Fatalf("recoveries = %#v", recoveries)
	}
	events, _ := store.ListMerchantEvents(context.Background(), "ws_1")
	if len(events) != 2 || events[0].EventType != "payment.paid" || events[1].EventType != "payment.recovered" {
		t.Fatalf("merchant events = %#v", events)
	}

	if _, err := svc.ProcessProviderWebhook(context.Background(), "conn_b", request); err != nil {
		t.Fatalf("duplicate ProcessProviderWebhook() error = %v", err)
	}
	recoveries, _ = store.ListRecoveryEvents(context.Background(), paid.ID)
	events, _ = store.ListMerchantEvents(context.Background(), "ws_1")
	if len(recoveries) != 1 || len(events) != 2 {
		t.Fatalf("duplicate changed state: recoveries=%d events=%d", len(recoveries), len(events))
	}
}

func TestProviderWebhookRejectsInvalidSignatureBeforeStateChange(t *testing.T) {
	store := memory.New()
	store.AddProviderConnection(domain.ProviderConnection{
		ID: "conn_a", WorkspaceID: "ws_1", ProviderKey: "provider_a", Environment: domain.EnvironmentTest,
		Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 1,
	})
	base := providerfake.New("provider_a", []provider.CreateOutcome{{
		Kind: provider.CreateSucceeded, ProviderPaymentID: "provider_payment_1",
		Pix: &domain.Pix{CopyPaste: "pix-a", ExpiresAt: time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC)},
	}}, nil)
	adapter := &signedWebhookAdapter{Adapter: base, secret: "valid-secret"}
	svc := orchestration.New(orchestration.Options{
		Repository: store,
		Providers:  providerfake.Registry{"provider_a": adapter},
	})
	intent, err := svc.CreatePayment(context.Background(), orchestration.CreatePaymentRequest{
		WorkspaceID: "ws_1", IdempotencyKey: "idem-signature", MerchantOrderID: "order-signature",
		Amount: 5000, Currency: "BRL", Environment: domain.EnvironmentTest,
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}

	_, err = svc.ProcessProviderWebhook(context.Background(), "conn_a", provider.WebhookRequest{
		Headers: map[string][]string{"X-Test-Signature": {"wrong"}},
		Body:    []byte(`{"event_id":"event-1","type":"payment.paid","payment_id":"provider_payment_1"}`),
	})
	if !errors.Is(err, provider.ErrInvalidWebhookSignature) {
		t.Fatalf("error = %v, want invalid signature", err)
	}

	stored, _ := store.GetPayment(context.Background(), intent.ID)
	if stored.Status != domain.PaymentStatusAwaitingPayment {
		t.Fatalf("status changed to %s", stored.Status)
	}
	events, _ := store.ListMerchantEvents(context.Background(), "ws_1")
	if len(events) != 0 {
		t.Fatalf("merchant events = %#v", events)
	}
}
