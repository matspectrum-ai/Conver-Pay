package orchestration

import (
	"context"
	"testing"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

type limitedConnector struct{}

func (limitedConnector) Manifest() provider.Manifest {
	return provider.Manifest{Key: "limited", DisplayName: "Limited", Version: "test", Capabilities: []provider.Capability{provider.CapabilityPaymentQuery}}
}

func (limitedConnector) CreatePix(context.Context, provider.CreateRequest) provider.CreateOutcome {
	return provider.CreateOutcome{Kind: provider.CreateFailedTerminal, FailureCode: "unsupported"}
}

func (limitedConnector) Reconcile(context.Context, provider.ReconcileRequest) provider.ReconcileOutcome {
	return provider.ReconcileOutcome{Kind: provider.ReconcileTerminal, FailureCode: "unsupported"}
}

func TestRouterExcludesUnsupportedCapability(t *testing.T) {
	connections := []domain.ProviderConnection{
		{ID: "limited", ProviderKey: "limited", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 1},
		{ID: "pix", ProviderKey: "pix", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 2},
	}
	pixConnector := limitedConnector{}
	// Reuse the fake connector shape for a provider that supports Pix.
	pixManifestConnector := testPixConnector{}
	router := Router{catalog: provider.MapRegistry{"limited": pixConnector, "pix": pixManifestConnector}}
	result := router.Select(connections, domain.EnvironmentTest, nil, provider.CapabilityPixCreate)
	if result.Selected == nil || result.Selected.ID != "pix" {
		t.Fatalf("selected = %#v, want pix", result.Selected)
	}
	if result.Candidates[0].ExclusionReason != "capability_unsupported" {
		t.Fatalf("limited exclusion = %q", result.Candidates[0].ExclusionReason)
	}
}

type testPixConnector struct{}

func (testPixConnector) Manifest() provider.Manifest {
	return provider.Manifest{Key: "pix", DisplayName: "Pix", Version: "test", Capabilities: []provider.Capability{provider.CapabilityPixCreate}}
}

func (testPixConnector) CreatePix(context.Context, provider.CreateRequest) provider.CreateOutcome {
	return provider.CreateOutcome{Kind: provider.CreateSucceeded}
}

func (testPixConnector) Reconcile(context.Context, provider.ReconcileRequest) provider.ReconcileOutcome {
	return provider.ReconcileOutcome{Kind: provider.ReconcileCreated}
}

func TestRouterUsesHealthScoreBeforePriority(t *testing.T) {
	connections := []domain.ProviderConnection{
		{ID: "a", ProviderKey: "a", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 1},
		{ID: "b", ProviderKey: "b", Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: 2},
	}
	health := map[string]domain.ProviderHealthSnapshot{
		"a": {ProviderConnectionID: "a", HealthScore: 0.61, ScoreVersion: domain.RoutingScoreVersion, SampleCount: 100},
		"b": {ProviderConnectionID: "b", HealthScore: 0.94, ScoreVersion: domain.RoutingScoreVersion, SampleCount: 100},
	}

	result := (Router{}).Select(connections, domain.EnvironmentTest, nil, provider.CapabilityPixCreate, health)
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

	result := (Router{}).Select(connections, domain.EnvironmentTest, nil, provider.CapabilityPixCreate, health)
	if result.Selected == nil || result.Selected.ID != "closed" {
		t.Fatalf("selected = %#v, want closed", result.Selected)
	}

	result = (Router{}).Select(connections, domain.EnvironmentTest, map[string]bool{"closed": true}, provider.CapabilityPixCreate, health)
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
		selected := (Router{}).Select(connections, domain.EnvironmentTest, nil, provider.CapabilityPixCreate, health).Selected
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
