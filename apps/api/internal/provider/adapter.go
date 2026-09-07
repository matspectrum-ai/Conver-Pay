package provider

import (
	"context"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type CreateOutcomeKind string

const (
	CreateSucceeded      CreateOutcomeKind = "succeeded"
	CreateFailedSafe     CreateOutcomeKind = "failed_safe"
	CreateFailedTerminal CreateOutcomeKind = "failed_terminal"
	CreateUnknown        CreateOutcomeKind = "unknown"
)

type CreateRequest struct {
	PaymentIntentID string
	AttemptID       string
	Amount          int64
	Currency        string
	MerchantOrderID string
}

type CreateOutcome struct {
	Kind              CreateOutcomeKind
	ProviderPaymentID string
	Pix               *domain.Pix
	FailureCode       string
}

type ReconcileOutcomeKind string

const (
	ReconcileNotCreated ReconcileOutcomeKind = "not_created"
	ReconcileCreated    ReconcileOutcomeKind = "created"
	ReconcileUnknown    ReconcileOutcomeKind = "unknown"
	ReconcileTerminal   ReconcileOutcomeKind = "terminal"
)

type ReconcileRequest struct {
	PaymentIntentID   string
	AttemptID         string
	ProviderPaymentID string
}

type ReconcileOutcome struct {
	Kind              ReconcileOutcomeKind
	ProviderPaymentID string
	Pix               *domain.Pix
	FailureCode       string
}

type Adapter interface {
	Key() string
	CreatePix(context.Context, CreateRequest) CreateOutcome
	Reconcile(context.Context, ReconcileRequest) ReconcileOutcome
}
