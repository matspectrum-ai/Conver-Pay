package orchestration

import (
	"context"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type Repository interface {
	GetOrCreatePayment(context.Context, *domain.PaymentIntent) (*domain.PaymentIntent, bool, error)
	GetPayment(context.Context, string) (*domain.PaymentIntent, error)
	SavePayment(context.Context, *domain.PaymentIntent) error

	ListProviderConnections(context.Context, string) ([]domain.ProviderConnection, error)

	AddAttemptWithRoutingDecision(context.Context, *domain.PaymentAttempt, *domain.RoutingDecision) error
	SaveAttempt(context.Context, *domain.PaymentAttempt) error
	GetAttempt(context.Context, string) (*domain.PaymentAttempt, error)
	ListAttempts(context.Context, string) ([]domain.PaymentAttempt, error)

	ListRoutingDecisions(context.Context, string) ([]domain.RoutingDecision, error)

	CreateRecoveryIfAbsent(context.Context, *domain.RecoveryEvent) (bool, error)
	ListRecoveryEvents(context.Context, string) ([]domain.RecoveryEvent, error)
}
