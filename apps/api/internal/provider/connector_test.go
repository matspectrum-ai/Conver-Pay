package provider

import (
	"context"
	"testing"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type catalogTestConnector struct{ manifest Manifest }

type catalogTestIntegration struct{ catalogTestConnector }

func (c catalogTestConnector) Manifest() Manifest { return c.manifest }
func (c catalogTestIntegration) ConnectorFor(context.Context, domain.ProviderConnection) (ProviderConnector, error) {
	return c.catalogTestConnector, nil
}
func (catalogTestIntegration) ValidateCredentials(context.Context, domain.Environment, map[string]string) error {
	return nil
}
func (catalogTestConnector) CreatePix(context.Context, CreateRequest) CreateOutcome {
	return CreateOutcome{Kind: CreateSucceeded}
}
func (catalogTestConnector) Reconcile(context.Context, ReconcileRequest) ReconcileOutcome {
	return ReconcileOutcome{Kind: ReconcileCreated}
}

func TestManifestSupportsCapability(t *testing.T) {
	manifest := Manifest{Key: "test", Capabilities: []Capability{CapabilityPaymentCreate, CapabilityPixCreate}}
	if !manifest.Supports(CapabilityPixCreate) {
		t.Fatal("pix create capability should be supported")
	}
	if manifest.Supports(CapabilityRefund) {
		t.Fatal("refund capability should not be supported")
	}
}

func TestConnectorRegistryManifestsAreDeterministic(t *testing.T) {
	r := ConnectorRegistry{
		"z": catalogTestIntegration{catalogTestConnector{manifest: Manifest{Key: "z", DisplayName: "Z"}}},
		"a": catalogTestIntegration{catalogTestConnector{manifest: Manifest{Key: "a", DisplayName: "A"}}},
	}
	manifests := r.Manifests()
	if len(manifests) != 2 || manifests[0].Key != "a" || manifests[1].Key != "z" {
		t.Fatalf("unexpected manifest order: %#v", manifests)
	}
}

func TestMapRegistryResolvesConnector(t *testing.T) {
	connector := catalogTestConnector{manifest: Manifest{Key: "test", Capabilities: []Capability{CapabilityPixCreate}}}
	r := MapRegistry{"test": connector}
	resolved, err := r.Resolve(context.Background(), domain.ProviderConnection{ProviderKey: "test"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Manifest().Key != "test" {
		t.Fatalf("unexpected connector manifest: %#v", resolved.Manifest())
	}
}
