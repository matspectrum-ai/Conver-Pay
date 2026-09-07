package orchestration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type IDGenerator interface {
	New(prefix string) string
}

type randomIDGenerator struct{}

func (randomIDGenerator) New(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

type Service struct {
	repo        Repository
	providers   provider.Registry
	router      Router
	clock       Clock
	ids         IDGenerator
	maxAttempts int
}

type Options struct {
	Repository  Repository
	Providers   provider.Registry
	Clock       Clock
	IDs         IDGenerator
	MaxAttempts int
}

func New(opts Options) *Service {
	clock := opts.Clock
	if clock == nil {
		clock = realClock{}
	}
	ids := opts.IDs
	if ids == nil {
		ids = randomIDGenerator{}
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	return &Service{
		repo:        opts.Repository,
		providers:   opts.Providers,
		router:      Router{},
		clock:       clock,
		ids:         ids,
		maxAttempts: maxAttempts,
	}
}

type CreatePaymentRequest struct {
	WorkspaceID     string
	IdempotencyKey  string
	MerchantOrderID string
	Amount          int64
	Currency        string
	Metadata        map[string]string
	Environment     domain.Environment
}

func (s *Service) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*domain.PaymentIntent, error) {
	fingerprint, err := requestFingerprint(req)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	intent := &domain.PaymentIntent{
		ID:                 s.ids.New("pi"),
		WorkspaceID:        req.WorkspaceID,
		MerchantOrderID:    req.MerchantOrderID,
		IdempotencyKey:     req.IdempotencyKey,
		RequestFingerprint: fingerprint,
		Amount:             req.Amount,
		Currency:           req.Currency,
		Metadata:           cloneMetadata(req.Metadata),
		Status:             domain.PaymentStatusCreated,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	stored, created, err := s.repo.GetOrCreatePayment(ctx, intent)
	if err != nil {
		return nil, err
	}
	if !created {
		return stored, nil
	}

	stored.Status = domain.PaymentStatusRouting
	stored.UpdatedAt = s.clock.Now()
	if err := s.repo.SavePayment(ctx, stored); err != nil {
		return nil, err
	}

	return s.executeRouting(ctx, stored, req.Environment)
}

func (s *Service) executeRouting(ctx context.Context, intent *domain.PaymentIntent, env domain.Environment) (*domain.PaymentIntent, error) {
	connections, err := s.repo.ListProviderConnections(ctx, intent.WorkspaceID)
	if err != nil {
		return nil, err
	}
	attempts, err := s.repo.ListAttempts(ctx, intent.ID)
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(attempts))
	for _, attempt := range attempts {
		excluded[attempt.ProviderConnectionID] = true
	}

	for len(attempts) < s.maxAttempts {
		route := s.router.Select(connections, env, excluded)
		if route.Selected == nil {
			intent.Status = domain.PaymentStatusFailed
			intent.FailureCode = "no_eligible_provider"
			intent.UpdatedAt = s.clock.Now()
			if err := s.repo.SavePayment(ctx, intent); err != nil {
				return nil, err
			}
			return intent, domain.ErrNoEligibleProvider
		}

		sequence := len(attempts) + 1
		now := s.clock.Now()
		attempt := &domain.PaymentAttempt{
			ID:                   s.ids.New("pa"),
			PaymentIntentID:      intent.ID,
			ProviderConnectionID: route.Selected.ID,
			Sequence:             sequence,
			Status:               domain.AttemptStatusCreated,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		decision := &domain.RoutingDecision{
			ID:                           s.ids.New("rd"),
			PaymentIntentID:              intent.ID,
			AttemptID:                    attempt.ID,
			SelectedProviderConnectionID: route.Selected.ID,
			Candidates:                   route.Candidates,
			ReasonCodes:                  []string{"priority"},
			CreatedAt:                    now,
		}
		if err := s.repo.AddRoutingDecision(ctx, decision); err != nil {
			return nil, err
		}
		if err := s.repo.AddAttempt(ctx, attempt); err != nil {
			return nil, err
		}
		intent.ActiveAttemptID = attempt.ID
		intent.UpdatedAt = now
		if err := s.repo.SavePayment(ctx, intent); err != nil {
			return nil, err
		}

		adapter, ok := s.providers.Get(route.Selected.ProviderKey)
		if !ok {
			attempt.Status = domain.AttemptStatusFailedTerminal
			attempt.FailureCode = "adapter_not_registered"
			attempt.UpdatedAt = s.clock.Now()
			_ = s.repo.SaveAttempt(ctx, attempt)
			intent.Status = domain.PaymentStatusFailed
			intent.FailureCode = attempt.FailureCode
			intent.UpdatedAt = s.clock.Now()
			_ = s.repo.SavePayment(ctx, intent)
			return intent, fmt.Errorf("provider adapter %q not registered", route.Selected.ProviderKey)
		}

		attempt.Status = domain.AttemptStatusRequesting
		attempt.RequestStartedAt = s.clock.Now()
		attempt.UpdatedAt = attempt.RequestStartedAt
		if err := s.repo.SaveAttempt(ctx, attempt); err != nil {
			return nil, err
		}

		outcome := adapter.CreatePix(ctx, provider.CreateRequest{
			PaymentIntentID: intent.ID,
			AttemptID:       attempt.ID,
			Amount:          intent.Amount,
			Currency:        intent.Currency,
			MerchantOrderID: intent.MerchantOrderID,
		})
		respondedAt := s.clock.Now()
		attempt.ResponseReceivedAt = &respondedAt
		attempt.UpdatedAt = respondedAt
		attempt.ProviderPaymentID = outcome.ProviderPaymentID
		attempt.FailureCode = outcome.FailureCode

		switch outcome.Kind {
		case provider.CreateSucceeded:
			attempt.Status = domain.AttemptStatusSucceeded
			attempt.Pix = clonePix(outcome.Pix)
			if err := s.repo.SaveAttempt(ctx, attempt); err != nil {
				return nil, err
			}
			intent.Status = domain.PaymentStatusAwaitingPayment
			intent.PresentedAttemptID = attempt.ID
			intent.Pix = clonePix(outcome.Pix)
			intent.FailureCode = ""
			intent.UpdatedAt = respondedAt
			if err := s.repo.SavePayment(ctx, intent); err != nil {
				return nil, err
			}
			return intent, nil

		case provider.CreateFailedSafe:
			attempt.Status = domain.AttemptStatusFailedSafe
			if err := s.repo.SaveAttempt(ctx, attempt); err != nil {
				return nil, err
			}
			excluded[route.Selected.ID] = true
			attempts = append(attempts, *attempt)
			continue

		case provider.CreateUnknown:
			attempt.Status = domain.AttemptStatusUnknown
			attempt.ReconciliationStatus = "required"
			if err := s.repo.SaveAttempt(ctx, attempt); err != nil {
				return nil, err
			}
			intent.Status = domain.PaymentStatusReconciling
			intent.FailureCode = outcome.FailureCode
			intent.UpdatedAt = respondedAt
			if err := s.repo.SavePayment(ctx, intent); err != nil {
				return nil, err
			}
			return intent, nil

		default:
			attempt.Status = domain.AttemptStatusFailedTerminal
			if err := s.repo.SaveAttempt(ctx, attempt); err != nil {
				return nil, err
			}
			intent.Status = domain.PaymentStatusFailed
			intent.FailureCode = outcome.FailureCode
			intent.UpdatedAt = respondedAt
			if err := s.repo.SavePayment(ctx, intent); err != nil {
				return nil, err
			}
			return intent, nil
		}
	}

	intent.Status = domain.PaymentStatusFailed
	intent.FailureCode = "fallback_budget_exhausted"
	intent.UpdatedAt = s.clock.Now()
	if err := s.repo.SavePayment(ctx, intent); err != nil {
		return nil, err
	}
	return intent, nil
}

func (s *Service) ReconcilePayment(ctx context.Context, paymentID string, env domain.Environment) (*domain.PaymentIntent, error) {
	intent, err := s.repo.GetPayment(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if intent.Status != domain.PaymentStatusReconciling {
		return nil, domain.ErrInvalidTransition
	}
	attempt, err := s.repo.GetAttempt(ctx, intent.ActiveAttemptID)
	if err != nil {
		return nil, err
	}
	if attempt.Status != domain.AttemptStatusUnknown {
		return nil, domain.ErrInvalidTransition
	}

	connections, err := s.repo.ListProviderConnections(ctx, intent.WorkspaceID)
	if err != nil {
		return nil, err
	}
	var conn *domain.ProviderConnection
	for i := range connections {
		if connections[i].ID == attempt.ProviderConnectionID {
			copy := connections[i]
			conn = &copy
			break
		}
	}
	if conn == nil {
		return nil, domain.ErrNotFound
	}
	adapter, ok := s.providers.Get(conn.ProviderKey)
	if !ok {
		return nil, fmt.Errorf("provider adapter %q not registered", conn.ProviderKey)
	}

	outcome := adapter.Reconcile(ctx, provider.ReconcileRequest{
		PaymentIntentID:   intent.ID,
		AttemptID:         attempt.ID,
		ProviderPaymentID: attempt.ProviderPaymentID,
	})
	now := s.clock.Now()

	switch outcome.Kind {
	case provider.ReconcileNotCreated:
		attempt.Status = domain.AttemptStatusFailedSafe
		attempt.ReconciliationStatus = "not_created"
		attempt.FailureCode = outcome.FailureCode
		attempt.UpdatedAt = now
		if err := s.repo.SaveAttempt(ctx, attempt); err != nil {
			return nil, err
		}
		intent.Status = domain.PaymentStatusRouting
		intent.FailureCode = ""
		intent.UpdatedAt = now
		if err := s.repo.SavePayment(ctx, intent); err != nil {
			return nil, err
		}
		return s.executeRouting(ctx, intent, env)

	case provider.ReconcileCreated:
		attempt.Status = domain.AttemptStatusSucceeded
		attempt.ReconciliationStatus = "created"
		attempt.ProviderPaymentID = outcome.ProviderPaymentID
		attempt.Pix = clonePix(outcome.Pix)
		attempt.FailureCode = ""
		attempt.UpdatedAt = now
		if err := s.repo.SaveAttempt(ctx, attempt); err != nil {
			return nil, err
		}
		intent.Status = domain.PaymentStatusAwaitingPayment
		intent.PresentedAttemptID = attempt.ID
		intent.Pix = clonePix(outcome.Pix)
		intent.FailureCode = ""
		intent.UpdatedAt = now
		if err := s.repo.SavePayment(ctx, intent); err != nil {
			return nil, err
		}
		return intent, nil

	case provider.ReconcileTerminal:
		attempt.Status = domain.AttemptStatusFailedTerminal
		attempt.ReconciliationStatus = "terminal"
		attempt.FailureCode = outcome.FailureCode
		attempt.UpdatedAt = now
		if err := s.repo.SaveAttempt(ctx, attempt); err != nil {
			return nil, err
		}
		intent.Status = domain.PaymentStatusFailed
		intent.FailureCode = outcome.FailureCode
		intent.UpdatedAt = now
		if err := s.repo.SavePayment(ctx, intent); err != nil {
			return nil, err
		}
		return intent, nil

	default:
		attempt.ReconciliationStatus = "still_unknown"
		attempt.UpdatedAt = now
		if err := s.repo.SaveAttempt(ctx, attempt); err != nil {
			return nil, err
		}
		return intent, nil
	}
}

func (s *Service) MarkPaid(ctx context.Context, paymentID, attemptID string) (*domain.PaymentIntent, error) {
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
	var failed *domain.PaymentAttempt
	for i := range attempts {
		if attempts[i].Sequence < attempt.Sequence && attempts[i].Status == domain.AttemptStatusFailedSafe {
			copy := attempts[i]
			failed = &copy
			break
		}
	}
	if failed != nil {
		created, err := s.repo.CreateRecoveryIfAbsent(ctx, &domain.RecoveryEvent{
			ID:                  s.ids.New("re"),
			PaymentIntentID:     intent.ID,
			FailedAttemptID:     failed.ID,
			SuccessfulAttemptID: attempt.ID,
			FailureReason:       failed.FailureCode,
			Amount:              intent.Amount,
			CreatedAt:           now,
		})
		if err != nil {
			return nil, err
		}
		if created {
			intent.Recovered = true
			intent.RecoveredAmount = intent.Amount
		}
	}

	if err := s.repo.SavePayment(ctx, intent); err != nil {
		return nil, err
	}
	return intent, nil
}

func requestFingerprint(req CreatePaymentRequest) (string, error) {
	canonical := struct {
		WorkspaceID     string             `json:"workspace_id"`
		MerchantOrderID string             `json:"merchant_order_id"`
		Amount          int64              `json:"amount"`
		Currency        string             `json:"currency"`
		Metadata        map[string]string  `json:"metadata"`
		Environment     domain.Environment `json:"environment"`
	}{
		WorkspaceID: req.WorkspaceID, MerchantOrderID: req.MerchantOrderID, Amount: req.Amount,
		Currency: req.Currency, Metadata: req.Metadata, Environment: req.Environment,
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func clonePix(pix *domain.Pix) *domain.Pix {
	if pix == nil {
		return nil
	}
	copy := *pix
	return &copy
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	copy := make(map[string]string, len(metadata))
	for k, v := range metadata {
		copy[k] = v
	}
	return copy
}
