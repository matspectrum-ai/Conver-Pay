package orchestration

import (
	"context"
	"encoding/json"
	"time"

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

func (s *Service) paymentMerchantEvents(intent *domain.PaymentIntent, createdAt time.Time) ([]domain.MerchantEvent, error) {
	paid, err := s.newMerchantEvent(intent, "payment.paid", "payment.paid:"+intent.ID, createdAt)
	if err != nil {
		return nil, err
	}
	events := []domain.MerchantEvent{paid}
	if intent.Recovered {
		recovered, err := s.newMerchantEvent(intent, "payment.recovered", "payment.recovered:"+intent.ID, createdAt)
		if err != nil {
			return nil, err
		}
		events = append(events, recovered)
	}
	return events, nil
}

func (s *Service) newMerchantEvent(intent *domain.PaymentIntent, eventType, eventKey string, createdAt time.Time) (domain.MerchantEvent, error) {
	eventID := s.ids.New("evt")
	payload, err := json.Marshal(map[string]any{
		"id":         eventID,
		"type":       eventType,
		"created_at": createdAt,
		"data": map[string]any{
			"payment_id":        intent.ID,
			"merchant_order_id": intent.MerchantOrderID,
			"amount":            intent.Amount,
			"currency":          intent.Currency,
			"status":            intent.Status,
			"recovered":         intent.Recovered,
			"recovered_amount":  intent.RecoveredAmount,
		},
	})
	if err != nil {
		return domain.MerchantEvent{}, err
	}
	return domain.MerchantEvent{
		ID:              eventID,
		WorkspaceID:     intent.WorkspaceID,
		EventKey:        eventKey,
		EventType:       eventType,
		PaymentIntentID: intent.ID,
		Payload:         payload,
		CreatedAt:       createdAt,
	}, nil
}
