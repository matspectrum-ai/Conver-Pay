package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

// ConnectorFactory binds one merchant-owned provider connection to a connector.
// Implementations may load per-connection credentials and must never expose
// plaintext credentials to the domain.
type ConnectorFactory interface {
	ConnectorFor(context.Context, domain.ProviderConnection) (ProviderConnector, error)
}

type CredentialValidator interface {
	ValidateCredentials(context.Context, domain.Environment, map[string]string) error
}

// ConnectorIntegration is the provider control-plane + data-plane integration.
// The manifest is static provider metadata; credentials remain connection-scoped.
type ConnectorIntegration interface {
	ConnectorFactory
	CredentialValidator
	Manifest() Manifest
}

type Registry interface {
	Resolve(context.Context, domain.ProviderConnection) (ProviderConnector, error)
	Manifest(string) (Manifest, bool)
}

// MapRegistry is used by deterministic tests and simple static integrations.
type MapRegistry map[string]ProviderConnector

func (r MapRegistry) Resolve(_ context.Context, connection domain.ProviderConnection) (ProviderConnector, error) {
	connector, ok := r[connection.ProviderKey]
	if !ok {
		return nil, fmt.Errorf("provider connector %q not registered", connection.ProviderKey)
	}
	return connector, nil
}

func (r MapRegistry) Manifest(providerKey string) (Manifest, bool) {
	connector, ok := r[providerKey]
	if !ok {
		return Manifest{}, false
	}
	return connector.Manifest(), true
}

// ConnectorFactoryRegistry resolves provider connector factories by provider key.
type ConnectorFactoryRegistry map[string]ConnectorFactory

func (r ConnectorFactoryRegistry) Resolve(ctx context.Context, connection domain.ProviderConnection) (ProviderConnector, error) {
	factory, ok := r[connection.ProviderKey]
	if !ok {
		return nil, fmt.Errorf("provider connector factory %q not registered", connection.ProviderKey)
	}
	return factory.ConnectorFor(ctx, connection)
}

// ConnectorRegistry serves provider connector resolution, capability discovery
// and control-plane credential validation.
type ConnectorRegistry map[string]ConnectorIntegration

func (r ConnectorRegistry) Resolve(ctx context.Context, connection domain.ProviderConnection) (ProviderConnector, error) {
	integration, ok := r[connection.ProviderKey]
	if !ok {
		return nil, fmt.Errorf("provider connector %q not registered", connection.ProviderKey)
	}
	return integration.ConnectorFor(ctx, connection)
}

func (r ConnectorRegistry) ValidateCredentials(ctx context.Context, providerKey string, environment domain.Environment, credentials map[string]string) error {
	integration, ok := r[providerKey]
	if !ok {
		return fmt.Errorf("provider connector %q not registered", providerKey)
	}
	return integration.ValidateCredentials(ctx, environment, credentials)
}

func (r ConnectorRegistry) Manifest(providerKey string) (Manifest, bool) {
	integration, ok := r[providerKey]
	if !ok {
		return Manifest{}, false
	}
	return integration.Manifest(), true
}

func (r ConnectorRegistry) Manifests() []Manifest {
	manifests := make([]Manifest, 0, len(r))
	for _, integration := range r {
		manifests = append(manifests, integration.Manifest())
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].Key < manifests[j].Key })
	return manifests
}

// Compatibility aliases. Existing code can migrate incrementally.
type Factory = ConnectorFactory
type Integration = ConnectorIntegration
type FactoryRegistry = ConnectorFactoryRegistry
type IntegrationRegistry = ConnectorRegistry
