package memory

import (
	"context"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

func (s *Store) GetPaymentEnvironment(_ context.Context, paymentID string) (domain.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	attemptIDs := s.attemptsByPay[paymentID]
	if len(attemptIDs) == 0 {
		return "", domain.ErrNotFound
	}
	attempt := s.attempts[attemptIDs[0]]
	if attempt == nil {
		return "", domain.ErrNotFound
	}
	for _, connections := range s.providers {
		for i := range connections {
			if connections[i].ID == attempt.ProviderConnectionID {
				return connections[i].Environment, nil
			}
		}
	}
	return "", domain.ErrNotFound
}
