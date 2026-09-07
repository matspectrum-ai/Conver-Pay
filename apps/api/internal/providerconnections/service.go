package providerconnections

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type Store interface {
	CreateProviderConnectionWithCredentials(context.Context, domain.ProviderConnection, string, time.Time) error
	ListProviderConnections(context.Context, string) ([]domain.ProviderConnection, error)
	DisableProviderConnection(context.Context, string, string, domain.Environment, time.Time) error
}

type Validator interface {
	ValidateCredentials(context.Context, string, domain.Environment, map[string]string) error
}

type Cipher interface {
	Encrypt([]byte) (string, error)
}

type Clock interface{ Now() time.Time }
type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type IDGenerator interface{ New(string) string }
type randomIDs struct{}

func (randomIDs) New(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

type Service struct {
	store     Store
	validator Validator
	cipher    Cipher
	clock     Clock
	ids       IDGenerator
}

type Options struct {
	Store     Store
	Validator Validator
	Cipher    Cipher
	Clock     Clock
	IDs       IDGenerator
}

func New(opts Options) (*Service, error) {
	if opts.Store == nil || opts.Validator == nil || opts.Cipher == nil {
		return nil, fmt.Errorf("provider connection store, validator and cipher are required")
	}
	clock := opts.Clock
	if clock == nil {
		clock = realClock{}
	}
	ids := opts.IDs
	if ids == nil {
		ids = randomIDs{}
	}
	return &Service{store: opts.Store, validator: opts.Validator, cipher: opts.Cipher, clock: clock, ids: ids}, nil
}

type ConnectRequest struct {
	WorkspaceID string
	Environment domain.Environment
	ProviderKey string
	Priority    int
	Credentials map[string]string
}

func (s *Service) Connect(ctx context.Context, req ConnectRequest) (*domain.ProviderConnection, error) {
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.ProviderKey = strings.ToLower(strings.TrimSpace(req.ProviderKey))
	if req.WorkspaceID == "" || req.ProviderKey == "" || req.Priority < 0 {
		return nil, fmt.Errorf("invalid provider connection request")
	}
	if req.Environment != domain.EnvironmentTest && req.Environment != domain.EnvironmentLive {
		return nil, fmt.Errorf("invalid provider environment")
	}
	if err := s.validator.ValidateCredentials(ctx, req.ProviderKey, req.Environment, cloneCredentials(req.Credentials)); err != nil {
		return nil, err
	}
	plaintext, err := json.Marshal(req.Credentials)
	if err != nil {
		return nil, fmt.Errorf("marshal provider credentials: %w", err)
	}
	ciphertext, err := s.cipher.Encrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt provider credentials: %w", err)
	}
	now := s.clock.Now()
	connection := domain.ProviderConnection{
		ID: s.ids.New("pc"), WorkspaceID: req.WorkspaceID, ProviderKey: req.ProviderKey,
		Environment: req.Environment, Enabled: true, CredentialsValid: true,
		Circuit: domain.CircuitClosed, Priority: req.Priority,
	}
	if err := s.store.CreateProviderConnectionWithCredentials(ctx, connection, ciphertext, now); err != nil {
		return nil, err
	}
	return &connection, nil
}

func (s *Service) List(ctx context.Context, workspaceID string, environment domain.Environment) ([]domain.ProviderConnection, error) {
	connections, err := s.store.ListProviderConnections(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ProviderConnection, 0, len(connections))
	for _, connection := range connections {
		if connection.Environment == environment {
			out = append(out, connection)
		}
	}
	return out, nil
}

func (s *Service) Disable(ctx context.Context, workspaceID, connectionID string, environment domain.Environment) error {
	return s.store.DisableProviderConnection(ctx, workspaceID, connectionID, environment, s.clock.Now())
}

func cloneCredentials(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
