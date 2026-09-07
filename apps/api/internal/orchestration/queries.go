package orchestration

import (
	"context"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

func (s *Service) GetPayment(ctx context.Context, workspaceID, paymentID string) (*domain.PaymentIntent, error) {
	intent, err := s.repo.GetPayment(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if intent.WorkspaceID != workspaceID {
		return nil, domain.ErrNotFound
	}
	return intent, nil
}

func (s *Service) ListPaymentAttempts(ctx context.Context, workspaceID, paymentID string) ([]domain.PaymentAttempt, error) {
	if _, err := s.GetPayment(ctx, workspaceID, paymentID); err != nil {
		return nil, err
	}
	return s.repo.ListAttempts(ctx, paymentID)
}
