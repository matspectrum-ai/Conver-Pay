package orchestration

import (
	"testing"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

func TestRouterUsesHealthScoreBeforePriority(t *testing.T) {
	connections := []domain.ProviderConnection{
		{ID: "a", ProviderKey: "a", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 1},
		{ID: "b", ProviderKey: "b", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 2},
	}
	health := map[string]domain.ProviderHealthSnapshot{
		"a": {ProviderConnectionID: "a", HealthScore: 0.61, ScoreVersion: domain.RoutingScoreVersion, SampleCount: 100},
		"b": {ProviderConnectionID: "b", HealthScore: 0.94, ScoreVersion: domain.RoutingScoreVersion, SampleCount: 100},
	}

	result := (Router{}).Select(connections, domain.EnvironmentTest, nil, health)
	if result.Selected == nil || result.Selected.ID != "b" {
		t.Fatalf("selected = %#v, want b", result.Selected)
	}
	if result.ScoreVersion != domain.RoutingScoreVersion {
		t.Fatalf("score version = %q", result.ScoreVersion)
	}
	if len(result.ReasonCodes) != 1 || result.ReasonCodes[0] != "automatic_health_score" {
		t.Fatalf("reason codes = %#v", result.ReasonCodes)
	}
}

func TestRouterPrefersClosedCircuitOverHalfOpen(t *testing.T) {
	connections := []domain.ProviderConnection{
		{ID: "closed", ProviderKey: "closed", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 10},
		{ID: "probe", ProviderKey: "probe", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitHalfOpen, Priority: 1},
	}
	health := map[string]domain.ProviderHealthSnapshot{
		"closed": {HealthScore: 0.60, ScoreVersion: domain.RoutingScoreVersion},
		"probe":  {HealthScore: 0.99, ScoreVersion: domain.RoutingScoreVersion},
	}

	result := (Router{}).Select(connections, domain.EnvironmentTest, nil, health)
	if result.Selected == nil || result.Selected.ID != "closed" {
		t.Fatalf("selected = %#v, want closed", result.Selected)
	}

	result = (Router{}).Select(connections, domain.EnvironmentTest, map[string]bool{"closed": true}, health)
	if result.Selected == nil || result.Selected.ID != "probe" {
		t.Fatalf("selected with closed excluded = %#v, want probe", result.Selected)
	}
}

func TestAutomaticRouterOutperformsFixedPriorityBaselineSimulation(t *testing.T) {
	connections := []domain.ProviderConnection{
		{ID: "priority", ProviderKey: "a", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 1},
		{ID: "healthy", ProviderKey: "b", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 2},
	}
	health := map[string]domain.ProviderHealthSnapshot{
		"priority": {HealthScore: 0.58, ScoreVersion: domain.RoutingScoreVersion, SampleCount: 100, QRSuccessRate: 0.60, LatencyP95MS: 900},
		"healthy":  {HealthScore: 0.93, ScoreVersion: domain.RoutingScoreVersion, SampleCount: 100, QRSuccessRate: 0.95, LatencyP95MS: 180},
	}

	priorityOutcomes := make([]bool, 100)
	healthyOutcomes := make([]bool, 100)
	for i := 0; i < 100; i++ {
		priorityOutcomes[i] = i < 60
		healthyOutcomes[i] = i < 95
	}

	automaticSuccesses := 0
	baselineSuccesses := 0
	for i := 0; i < 100; i++ {
		selected := (Router{}).Select(connections, domain.EnvironmentTest, nil, health).Selected
		if selected == nil {
			t.Fatal("automatic router selected no provider")
		}
		if selected.ID == "healthy" && healthyOutcomes[i] {
			automaticSuccesses++
		}
		if priorityOutcomes[i] {
			baselineSuccesses++
		}
	}
	if automaticSuccesses != 95 || baselineSuccesses != 60 || automaticSuccesses <= baselineSuccesses {
		t.Fatalf("automatic=%d baseline=%d", automaticSuccesses, baselineSuccesses)
	}
}
