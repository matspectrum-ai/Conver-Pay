package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

type Connector struct {
	key string

	mu                sync.Mutex
	createOutcomes    []provider.CreateOutcome
	reconcileOutcomes []provider.ReconcileOutcome
	createCalls       int
	reconcileCalls    int
}

type Adapter = Connector

func New(key string, creates []provider.CreateOutcome, reconciles []provider.ReconcileOutcome) *Connector {
	return &Connector{key: key, createOutcomes: creates, reconcileOutcomes: reconciles}
}

func (a *Connector) Key() string { return a.key }

func (a *Connector) Manifest() provider.Manifest {
	return provider.Manifest{
		Key: a.key, DisplayName: a.key, Version: "test",
		Capabilities: []provider.Capability{provider.CapabilityPaymentCreate, provider.CapabilityPixCreate, provider.CapabilityPaymentQuery, provider.CapabilityCreateReconcile},
	}
}

func (a *Connector) CreatePix(_ context.Context, _ provider.CreateRequest) provider.CreateOutcome {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.createCalls++
	if len(a.createOutcomes) == 0 {
		return provider.CreateOutcome{Kind: provider.CreateFailedTerminal, FailureCode: "fake_no_create_outcome"}
	}
	out := a.createOutcomes[0]
	a.createOutcomes = a.createOutcomes[1:]
	return out
}

func (a *Connector) Reconcile(_ context.Context, _ provider.ReconcileRequest) provider.ReconcileOutcome {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reconcileCalls++
	if len(a.reconcileOutcomes) == 0 {
		return provider.ReconcileOutcome{Kind: provider.ReconcileUnknown}
	}
	out := a.reconcileOutcomes[0]
	a.reconcileOutcomes = a.reconcileOutcomes[1:]
	return out
}

func (a *Connector) CreateCalls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.createCalls
}

func (a *Connector) ReconcileCalls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reconcileCalls
}

type Registry map[string]provider.ProviderConnector

func (r Registry) Resolve(_ context.Context, connection domain.ProviderConnection) (provider.ProviderConnector, error) {
	a, ok := r[connection.ProviderKey]
	if !ok {
		return nil, fmt.Errorf("fake provider connector %q not registered", connection.ProviderKey)
	}
	return a, nil
}

func (r Registry) Manifest(providerKey string) (provider.Manifest, bool) {
	a, ok := r[providerKey]
	if !ok {
		return provider.Manifest{}, false
	}
	return a.Manifest(), true
}
