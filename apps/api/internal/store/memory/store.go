package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type idempotencyRecord struct {
	paymentID   string
	fingerprint string
}

type Store struct {
	mu sync.Mutex

	payments        map[string]*domain.PaymentIntent
	idempotency     map[string]idempotencyRecord
	providers       map[string][]domain.ProviderConnection
	attempts        map[string]*domain.PaymentAttempt
	attemptsByPay   map[string][]string
	decisionsByPay  map[string][]domain.RoutingDecision
	recoveriesByPay map[string][]domain.RecoveryEvent
}

func New() *Store {
	return &Store{
		payments:        map[string]*domain.PaymentIntent{},
		idempotency:     map[string]idempotencyRecord{},
		providers:       map[string][]domain.ProviderConnection{},
		attempts:        map[string]*domain.PaymentAttempt{},
		attemptsByPay:   map[string][]string{},
		decisionsByPay:  map[string][]domain.RoutingDecision{},
		recoveriesByPay: map[string][]domain.RecoveryEvent{},
	}
}

func (s *Store) AddProviderConnection(conn domain.ProviderConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[conn.WorkspaceID] = append(s.providers[conn.WorkspaceID], conn)
}

func (s *Store) GetOrCreatePayment(_ context.Context, intent *domain.PaymentIntent) (*domain.PaymentIntent, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := intent.WorkspaceID + "\x00" + intent.IdempotencyKey
	if record, ok := s.idempotency[key]; ok {
		if record.fingerprint != intent.RequestFingerprint {
			return nil, false, domain.ErrIdempotencyConflict
		}
		return clonePayment(s.payments[record.paymentID]), false, nil
	}
	s.payments[intent.ID] = clonePayment(intent)
	s.idempotency[key] = idempotencyRecord{paymentID: intent.ID, fingerprint: intent.RequestFingerprint}
	return clonePayment(intent), true, nil
}

func (s *Store) GetPayment(_ context.Context, id string) (*domain.PaymentIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	intent, ok := s.payments[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return clonePayment(intent), nil
}

func (s *Store) SavePayment(_ context.Context, intent *domain.PaymentIntent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.payments[intent.ID]; !ok {
		return domain.ErrNotFound
	}
	s.payments[intent.ID] = clonePayment(intent)
	return nil
}

func (s *Store) ListProviderConnections(_ context.Context, workspaceID string) ([]domain.ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.ProviderConnection(nil), s.providers[workspaceID]...), nil
}

func (s *Store) AddAttemptWithRoutingDecision(_ context.Context, attempt *domain.PaymentAttempt, decision *domain.RoutingDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.attempts[attempt.ID]; exists {
		return domain.ErrInvalidTransition
	}
	s.attempts[attempt.ID] = cloneAttempt(attempt)
	s.attemptsByPay[attempt.PaymentIntentID] = append(s.attemptsByPay[attempt.PaymentIntentID], attempt.ID)
	copy := *decision
	copy.Candidates = append([]domain.CandidateSnapshot(nil), decision.Candidates...)
	copy.ReasonCodes = append([]string(nil), decision.ReasonCodes...)
	s.decisionsByPay[decision.PaymentIntentID] = append(s.decisionsByPay[decision.PaymentIntentID], copy)
	return nil
}

func (s *Store) SaveAttempt(_ context.Context, attempt *domain.PaymentAttempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.attempts[attempt.ID]; !ok {
		return domain.ErrNotFound
	}
	s.attempts[attempt.ID] = cloneAttempt(attempt)
	return nil
}

func (s *Store) GetAttempt(_ context.Context, id string) (*domain.PaymentAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.attempts[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneAttempt(attempt), nil
}

func (s *Store) ListAttempts(_ context.Context, paymentID string) ([]domain.PaymentAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.attemptsByPay[paymentID]
	out := make([]domain.PaymentAttempt, 0, len(ids))
	for _, id := range ids {
		out = append(out, *cloneAttempt(s.attempts[id]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
	return out, nil
}

func (s *Store) ListRoutingDecisions(_ context.Context, paymentID string) ([]domain.RoutingDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.RoutingDecision(nil), s.decisionsByPay[paymentID]...), nil
}

func (s *Store) CreateRecoveryIfAbsent(_ context.Context, event *domain.RecoveryEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.recoveriesByPay[event.PaymentIntentID]) > 0 {
		return false, nil
	}
	s.recoveriesByPay[event.PaymentIntentID] = []domain.RecoveryEvent{*event}
	return true, nil
}

func (s *Store) ListRecoveryEvents(_ context.Context, paymentID string) ([]domain.RecoveryEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.RecoveryEvent(nil), s.recoveriesByPay[paymentID]...), nil
}

func clonePayment(in *domain.PaymentIntent) *domain.PaymentIntent {
	if in == nil {
		return nil
	}
	out := *in
	if in.Pix != nil {
		pix := *in.Pix
		out.Pix = &pix
	}
	if in.Metadata != nil {
		out.Metadata = make(map[string]string, len(in.Metadata))
		for k, v := range in.Metadata {
			out.Metadata[k] = v
		}
	}
	if in.PaidAt != nil {
		paid := *in.PaidAt
		out.PaidAt = &paid
	}
	return &out
}

func cloneAttempt(in *domain.PaymentAttempt) *domain.PaymentAttempt {
	if in == nil {
		return nil
	}
	out := *in
	if in.Pix != nil {
		pix := *in.Pix
		out.Pix = &pix
	}
	if in.ResponseReceivedAt != nil {
		responded := *in.ResponseReceivedAt
		out.ResponseReceivedAt = &responded
	}
	return &out
}
