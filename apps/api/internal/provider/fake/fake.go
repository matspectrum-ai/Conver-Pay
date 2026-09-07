package fake

import (
	"context"
	"sync"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

type Adapter struct {
	key string

	mu                sync.Mutex
	createOutcomes    []provider.CreateOutcome
	reconcileOutcomes []provider.ReconcileOutcome
	createCalls       int
	reconcileCalls    int
}

func New(key string, creates []provider.CreateOutcome, reconciles []provider.ReconcileOutcome) *Adapter {
	return &Adapter{key: key, createOutcomes: creates, reconcileOutcomes: reconciles}
}

func (a *Adapter) Key() string { return a.key }

func (a *Adapter) CreatePix(_ context.Context, _ provider.CreateRequest) provider.CreateOutcome {
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

func (a *Adapter) Reconcile(_ context.Context, _ provider.ReconcileRequest) provider.ReconcileOutcome {
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

func (a *Adapter) CreateCalls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.createCalls
}

func (a *Adapter) ReconcileCalls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reconcileCalls
}

type Registry map[string]provider.Adapter

func (r Registry) Get(key string) (provider.Adapter, bool) {
	a, ok := r[key]
	return a, ok
}
