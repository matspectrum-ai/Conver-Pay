package orchestration

import (
	"context"
	"encoding/json"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

func (s *Service) MarkPaidWithOutbox(ctx context.Context, paymentID, attemptID string) (*domain.PaymentIntent, error) {
	intent, err := s.repo.GetPayment(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if intent.Status == domain.PaymentStatusPaid {
		return intent, nil
	}
	if intent.Status != domain.PaymentStatusAwaitingPayment || intent.PresentedAttemptID != attemptID {
		return nil, domain.ErrInvalidTransition
	}

	attempt, err := s.repo.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.Status != domain.AttemptStatusSucceeded {
		return nil, domain.ErrInvalidTransition
	}

	now := s.clock.Now()
	intent.Status = domain.PaymentStatusPaid
	intent.PaidAt = &now
	intent.UpdatedAt = now

	attempts, err := s.repo.ListAttempts(ctx, intent.ID)
	if err != nil {
		return nil, err
	}

	var recovery *domain.RecoveryEvent
	for i := range attempts {
		if attempts[i].Sequence < attempt.Sequence && attempts[i].Status == domain.AttemptStatusFailedSafe {
			failed := attempts[i]
			recovery = &domain.RecoveryEvent{
				ID:                  s.ids.New("re"),
				PaymentIntentID:     intent.ID,
				FailedAttemptID:     failed.ID,
				SuccessfulAttemptID: attempt.ID,
				FailureReason:       failed.FailureCode,
				Amount:              intent.Amount,
				CreatedAt:           now,
			}
			intent.Recovered = true
			intent.RecoveredAmount = intent.Amount
			break
		}
	}

	events, err := s.paymentMerchantEvents(intent, now)
	if err != nil {
		return nil, err
	}
	if err := s.repo.MarkPaymentPaidWithEvents(ctx, intent, recovery, events); err != nil {
		return nil, err
	}
	return intent, nil
}

func (s *Service) paymentMerchantEvents(intent *domain.PaymentIntent, now interface{ MarshalJSON() ([]byte, error) }) ([]domain.MerchantEvent, error) {
	createdAt, ok := now.(interface{ String() string })
	_ = createdAt
	_ = ok
	return nil, nil
}

func (s *Service) newMerchantEvent(intent *domain.PaymentIntent, eventType, eventKey string, createdAt interface{}) (domain.MerchantEvent, error) {
	payload, err := json.Marshal(map[string]any{
		"id":   eventKey,
		"type": eventType,
		"data": map[string]any{
			"payment_id":       intent.ID,
			"merchant_order_id": intent.MerchantOrderID,
			"amount":           intent.Amount,
			"currency":         intent.Currency,
			"status":           intent.Status,
			"recovered":        intent.Recovered,
			"recovered_amount": intent.RecoveredAmount,
		},
	})
	if err != nil {
		return domain.MerchantEvent{}, err
	}
	return domain.MerchantEvent{
		ID:              s.ids.New("evt"),
		WorkspaceID:     intent.WorkspaceID,
		EventKey:        eventKey,
		EventType:       eventType,
		PaymentIntentID: intent.ID,
		Payload:         payload,
	}, nil
}
