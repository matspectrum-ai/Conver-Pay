package routingtelemetry

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type Repository interface {
	AggregateProviderHealth(context.Context, string, time.Time, time.Time) (domain.ProviderHealthSnapshot, error)
	SaveProviderHealthSnapshot(context.Context, *domain.ProviderHealthSnapshot) error
	ListRecentProviderHealthSnapshots(context.Context, string, int) ([]domain.ProviderHealthSnapshot, error)
	LatestProviderAttemptSince(context.Context, string, time.Time) (*domain.PaymentAttempt, error)
	UpdateProviderCircuit(context.Context, string, domain.CircuitState, *time.Time) error
}

type Options struct {
	Window          time.Duration
	CircuitCooldown time.Duration
	MinimumSamples  int
}

type Service struct {
	repo            Repository
	window          time.Duration
	circuitCooldown time.Duration
	minimumSamples  int
}

func New(repo Repository, opts Options) *Service {
	window := opts.Window
	if window <= 0 {
		window = 15 * time.Minute
	}
	cooldown := opts.CircuitCooldown
	if cooldown <= 0 {
		cooldown = time.Minute
	}
	minimumSamples := opts.MinimumSamples
	if minimumSamples <= 0 {
		minimumSamples = 20
	}
	return &Service{repo: repo, window: window, circuitCooldown: cooldown, minimumSamples: minimumSamples}
}

func (s *Service) Refresh(ctx context.Context, connections []domain.ProviderConnection, env domain.Environment, now time.Time) (map[string]domain.ProviderHealthSnapshot, error) {
	result := make(map[string]domain.ProviderHealthSnapshot)
	for i := range connections {
		conn := connections[i]
		if conn.Environment != env {
			continue
		}
		previous, err := s.repo.ListRecentProviderHealthSnapshots(ctx, conn.ID, 1)
		if err != nil {
			return nil, err
		}
		snapshot, err := s.repo.AggregateProviderHealth(ctx, conn.ID, now.Add(-s.window), now)
		if err != nil {
			return nil, err
		}
		snapshot.ProviderConnectionID = conn.ID
		snapshot.WindowStart = now.Add(-s.window)
		snapshot.WindowEnd = now
		snapshot.ObservedAt = now
		snapshot.ScoreVersion = domain.RoutingScoreVersion
		snapshot.HealthScore = score(snapshot)
		if err := s.repo.SaveProviderHealthSnapshot(ctx, &snapshot); err != nil {
			return nil, err
		}
		if err := s.evaluateCircuit(ctx, conn, snapshot, previous, now); err != nil {
			return nil, err
		}
		result[conn.ID] = snapshot
	}
	return result, nil
}

func (s *Service) evaluateCircuit(ctx context.Context, conn domain.ProviderConnection, current domain.ProviderHealthSnapshot, previous []domain.ProviderHealthSnapshot, now time.Time) error {
	switch conn.Circuit {
	case domain.CircuitClosed:
		if unhealthy(current, s.minimumSamples) && len(previous) > 0 && !previous[0].ObservedAt.Before(now.Add(-s.window)) && unhealthy(previous[0], s.minimumSamples) {
			openedAt := now
			return s.repo.UpdateProviderCircuit(ctx, conn.ID, domain.CircuitOpen, &openedAt)
		}
	case domain.CircuitOpen:
		if conn.CircuitOpenedAt != nil && !now.Before(conn.CircuitOpenedAt.Add(s.circuitCooldown)) {
			return s.repo.UpdateProviderCircuit(ctx, conn.ID, domain.CircuitHalfOpen, conn.CircuitOpenedAt)
		}
	case domain.CircuitHalfOpen:
		if conn.CircuitOpenedAt == nil {
			openedAt := now
			return s.repo.UpdateProviderCircuit(ctx, conn.ID, domain.CircuitOpen, &openedAt)
		}
		attempt, err := s.repo.LatestProviderAttemptSince(ctx, conn.ID, *conn.CircuitOpenedAt)
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		switch attempt.Status {
		case domain.AttemptStatusSucceeded:
			return s.repo.UpdateProviderCircuit(ctx, conn.ID, domain.CircuitClosed, nil)
		case domain.AttemptStatusFailedSafe, domain.AttemptStatusFailedTerminal, domain.AttemptStatusUnknown:
			openedAt := now
			return s.repo.UpdateProviderCircuit(ctx, conn.ID, domain.CircuitOpen, &openedAt)
		}
	}
	return nil
}

func score(snapshot domain.ProviderHealthSnapshot) float64 {
	if snapshot.SampleCount < 5 {
		return 0.5
	}
	latencyScore := 1 - math.Min(snapshot.LatencyP95MS, 2000)/2000
	value := 0.50*snapshot.QRSuccessRate +
		0.20*(1-snapshot.ErrorRate) +
		0.15*(1-snapshot.TimeoutRate) +
		0.15*latencyScore
	return clamp01(value)
}

func unhealthy(snapshot domain.ProviderHealthSnapshot, minimumSamples int) bool {
	if snapshot.SampleCount < minimumSamples {
		return false
	}
	return snapshot.QRSuccessRate < 0.70 || snapshot.ErrorRate >= 0.30 || snapshot.TimeoutRate >= 0.20
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
