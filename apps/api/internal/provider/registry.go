package provider

import (
	"context"
	"fmt"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

// Factory binds a provider connection to an adapter. Implementations may load
// per-connection credentials and must never expose plaintext credentials to the domain.
type Factory interface {
	AdapterFor(context.Context, domain.ProviderConnection) (Adapter, error)
}

type CredentialValidator interface {
	ValidateCredentials(context.Context, domain.Environment, map[string]string) error
}

type Integration interface {
	Factory
	CredentialValidator
}

type Registry interface {
	Resolve(context.Context, domain.ProviderConnection) (Adapter, error)
}

// MapRegistry is used by deterministic tests and simple static integrations.
type MapRegistry map[string]Adapter

func (r MapRegistry) Resolve(_ context.Context, connection domain.ProviderConnection) (Adapter, error) {
	adapter, ok := r[connection.ProviderKey]
	if !ok {
		return nil, fmt.Errorf("provider adapter %q not registered", connection.ProviderKey)
	}
	return adapter, nil
}

// FactoryRegistry resolves provider-specific factories by provider key.
type FactoryRegistry map[string]Factory

func (r FactoryRegistry) Resolve(ctx context.Context, connection domain.ProviderConnection) (Adapter, error) {
	factory, ok := r[connection.ProviderKey]
	if !ok {
		return nil, fmt.Errorf("provider factory %q not registered", connection.ProviderKey)
	}
	return factory.AdapterFor(ctx, connection)
}

// IntegrationRegistry serves both the data-plane adapter resolver and the
// control-plane credential validator for real providers.
type IntegrationRegistry map[string]Integration

func (r IntegrationRegistry) Resolve(ctx context.Context, connection domain.ProviderConnection) (Adapter, error) {
	integration, ok := r[connection.ProviderKey]
	if !ok {
		return nil, fmt.Errorf("provider integration %q not registered", connection.ProviderKey)
	}
	return integration.AdapterFor(ctx, connection)
}

func (r IntegrationRegistry) ValidateCredentials(ctx context.Context, providerKey string, environment domain.Environment, credentials map[string]string) error {
	integration, ok := r[providerKey]
	if !ok {
		return fmt.Errorf("provider integration %q not registered", providerKey)
	}
	return integration.ValidateCredentials(ctx, environment, credentials)
}
