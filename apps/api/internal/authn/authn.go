package authn

import (
	"context"
	"errors"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

var ErrUnauthorized = errors.New("unauthorized")

type Principal struct {
	APIKeyID    string
	WorkspaceID string
	Environment domain.Environment
}

type Resolver interface {
	ResolveAPIKey(context.Context, string) (Principal, error)
}
