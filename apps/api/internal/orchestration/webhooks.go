package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

func (s *Service) ProcessProviderWebhook(ctx context.Context, providerConnectionID string, request provider.WebhookRequest) (*domain.PaymentIntent, error) {
	connection, err := s.repo.GetProviderConnection(ctx, providerConnectionID)
	if err != nil {
		return nil, err
	}

	adapter, ok := s.providers.Get(connection.ProviderKey)
	if !ok {
		return nil, fmt.Errorf("provider adapter %q not registered", connection.ProviderKey)
	}
	webhookAdapter, ok := adapter.(provider.WebhookAdapter)
	if !ok {
		return nil, provider.ErrUnsupportedWebhook
	}

	event, err := webhookAdapter.ParseWebhook(ctx, request)
	if err != nil {
		return nil, err
	}
	if event.ExternalEventID == "" || event.ProviderPaymentID == "" || event.Type == "" {
		return nil, provider.ErrUnsupportedWebhook
	}
	if event.Type != "payment.paid" {
		return nil, provider.ErrUnsupportedWebhook
	}

	hash := sha256.Sum256(request.Body)
	now := s.clock.Now()
	_, err = s.repo.RecordProviderEvent(ctx, &domain.ProviderEvent{
		ID:                   s.ids.New("pev"),
		ProviderConnectionID: providerConnectionID,
		ExternalEventID:      event.ExternalEventID,
		EventType:            event.Type,
		ProviderPaymentID:    event.ProviderPaymentID,
		PayloadHash:          hex.EncodeToString(hash[:]),
		ReceivedAt:           now,
	})
	if err != nil {
		return nil, err
	}

	attempt, err := s.repo.GetAttemptByProviderPaymentID(ctx, providerConnectionID, event.ProviderPaymentID)
	if err != nil {
		return nil, err
	}
	intent, err := s.MarkPaidWithOutbox(ctx, attempt.PaymentIntentID, attempt.ID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.MarkProviderEventProcessed(ctx, providerConnectionID, event.ExternalEventID, s.clock.Now()); err != nil {
		return nil, err
	}
	return intent, nil
}
