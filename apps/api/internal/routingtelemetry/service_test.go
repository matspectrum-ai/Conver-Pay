package routingtelemetry

import (
	"context"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type fakeRepository struct {
	aggregate domain.ProviderHealthSnapshot
	previous  []domain.ProviderHealthSnapshot
	latest    *domain.PaymentAttempt
	saved     []domain.ProviderHealthSnapshot
	updates   []circuitUpdate
}

type circuitUpdate struct {
	state    domain.CircuitState
	openedAt *time.Time
}

func (f *fakeRepository) AggregateProviderHealth(context.Context, string, time.Time, time.Time) (domain.ProviderHealthSnapshot, error) {
	return f.aggregate, nil
}
func (f *fakeRepository) SaveProviderHealthSnapshot(_ context.Context, snapshot *domain.ProviderHealthSnapshot) error {
	f.saved = append(f.saved, *snapshot)
	return nil
}
func (f *fakeRepository) ListRecentProviderHealthSnapshots(context.Context, string, int) ([]domain.ProviderHealthSnapshot, error) {
	return append([]domain.ProviderHealthSnapshot(nil), f.previous...), nil
}
func (f *fakeRepository) LatestProviderAttemptSince(context.Context, string, time.Time) (*domain.PaymentAttempt, error) {
	if f.latest == nil {
		return nil, domain.ErrNotFound
	}
	copy := *f.latest
	return &copy, nil
}
func (f *fakeRepository) UpdateProviderCircuit(_ context.Context, _ string, state domain.CircuitState, openedAt *time.Time) error {
	f.updates = append(f.updates, circuitUpdate{state: state, openedAt: openedAt})
	return nil
}

func TestScoreUsesNeutralColdStart(t *testing.T) {
	if got := score(domain.ProviderHealthSnapshot{SampleCount: 4, QRSuccessRate: 1}); got != 0.5 {
		t.Fatalf("score = %f, want 0.5", got)
	}
	got := score(domain.ProviderHealthSnapshot{
		SampleCount: 100, QRSuccessRate: 0.95, ErrorRate: 0.03,
		TimeoutRate: 0.02, LatencyP95MS: 200,
	})
	if got < 0.90 || got > 1.0 {
		t.Fatalf("healthy score = %f", got)
	}
}

func TestCircuitRequiresTwoRecentUnhealthySnapshots(t *testing.T) {
	now := time.Date(2026, 9, 7, 19, 0, 0, 0, time.UTC)
	unhealthySnapshot := domain.ProviderHealthSnapshot{
		SampleCount: 20, QRSuccessCount: 10, ErrorCount: 6, TimeoutCount: 4,
		QRSuccessRate: 0.50, ErrorRate: 0.30, TimeoutRate: 0.20,
		LatencyP95MS: 1200,
	}
	repo := &fakeRepository{
		aggregate: unhealthySnapshot,
		previous: []domain.ProviderHealthSnapshot{{
			SampleCount: 20, QRSuccessRate: 0.55, ErrorRate: 0.30,
			TimeoutRate: 0.15, ObservedAt: now.Add(-time.Minute),
		}},
	}
	service := New(repo, Options{Window: 15 * time.Minute, MinimumSamples: 20})
	_, err := service.Refresh(context.Background(), []domain.ProviderConnection{{
		ID: "conn", Environment: domain.EnvironmentTest, Circuit: domain.CircuitClosed,
	}}, domain.EnvironmentTest, now)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(repo.updates) != 1 || repo.updates[0].state != domain.CircuitOpen || repo.updates[0].openedAt == nil {
		t.Fatalf("updates = %#v", repo.updates)
	}
}

func TestCircuitDoesNotOpenFromSingleBadSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 7, 19, 0, 0, 0, time.UTC)
	repo := &fakeRepository{aggregate: domain.ProviderHealthSnapshot{
		SampleCount: 20, QRSuccessRate: 0.50, ErrorRate: 0.30, TimeoutRate: 0.20,
	}}
	service := New(repo, Options{})
	_, err := service.Refresh(context.Background(), []domain.ProviderConnection{{
		ID: "conn", Environment: domain.EnvironmentTest, Circuit: domain.CircuitClosed,
	}}, domain.EnvironmentTest, now)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(repo.updates) != 0 {
		t.Fatalf("unexpected updates = %#v", repo.updates)
	}
}

func TestOpenCircuitMovesToHalfOpenAfterCooldown(t *testing.T) {
	now := time.Date(2026, 9, 7, 19, 0, 0, 0, time.UTC)
	opened := now.Add(-2 * time.Minute)
	repo := &fakeRepository{}
	service := New(repo, Options{CircuitCooldown: time.Minute})
	_, err := service.Refresh(context.Background(), []domain.ProviderConnection{{
		ID: "conn", Environment: domain.EnvironmentTest, Circuit: domain.CircuitOpen, CircuitOpenedAt: &opened,
	}}, domain.EnvironmentTest, now)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(repo.updates) != 1 || repo.updates[0].state != domain.CircuitHalfOpen {
		t.Fatalf("updates = %#v", repo.updates)
	}
}

func TestHalfOpenCircuitClosesAfterSuccessfulProbe(t *testing.T) {
	now := time.Date(2026, 9, 7, 19, 0, 0, 0, time.UTC)
	opened := now.Add(-2 * time.Minute)
	repo := &fakeRepository{latest: &domain.PaymentAttempt{Status: domain.AttemptStatusSucceeded}}
	service := New(repo, Options{})
	_, err := service.Refresh(context.Background(), []domain.ProviderConnection{{
		ID: "conn", Environment: domain.EnvironmentTest, Circuit: domain.CircuitHalfOpen, CircuitOpenedAt: &opened,
	}}, domain.EnvironmentTest, now)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(repo.updates) != 1 || repo.updates[0].state != domain.CircuitClosed || repo.updates[0].openedAt != nil {
		t.Fatalf("updates = %#v", repo.updates)
	}
}
