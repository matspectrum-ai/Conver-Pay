package providerconnections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

type testStore struct {
	connections []domain.ProviderConnection
	ciphertext  string
	disabledID  string
}

func (s *testStore) CreateProviderConnectionWithCredentials(_ context.Context, connection domain.ProviderConnection, ciphertext string, _ time.Time) error {
	s.connections = append(s.connections, connection)
	s.ciphertext = ciphertext
	return nil
}
func (s *testStore) ListProviderConnections(context.Context, string) ([]domain.ProviderConnection, error) {
	return append([]domain.ProviderConnection(nil), s.connections...), nil
}
func (s *testStore) DisableProviderConnection(_ context.Context, _, id string, _ domain.Environment, _ time.Time) error {
	s.disabledID = id
	return nil
}

type testValidator struct{ err error }

func (v testValidator) ValidateCredentials(context.Context, string, domain.Environment, map[string]string) error {
	return v.err
}

type testCipher struct{ plaintext []byte }

func (c *testCipher) Encrypt(p []byte) (string, error) {
	c.plaintext = append([]byte(nil), p...)
	return "v1.encrypted", nil
}

type testIDs struct{}

func (testIDs) New(string) string { return "pc_test" }

type testClock struct{ now time.Time }

func (c testClock) Now() time.Time { return c.now }

func TestConnectValidatesBeforePersistingAndStoresCiphertext(t *testing.T) {
	store := &testStore{}
	cipher := &testCipher{}
	svc, err := New(Options{Store: store, Validator: testValidator{}, Cipher: cipher, IDs: testIDs{}, Clock: testClock{now: time.Date(2026, 9, 7, 19, 0, 0, 0, time.UTC)}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	connection, err := svc.Connect(context.Background(), ConnectRequest{
		WorkspaceID: "ws_1", Environment: domain.EnvironmentTest, ProviderKey: " Woovi ", Priority: 10,
		Credentials: map[string]string{"app_id": "secret-app-id"},
	})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if connection.ProviderKey != "woovi" || !connection.CredentialsValid || !connection.Enabled {
		t.Fatalf("connection = %#v", connection)
	}
	if store.ciphertext != "v1.encrypted" || string(cipher.plaintext) == "" {
		t.Fatalf("ciphertext=%q plaintext=%q", store.ciphertext, cipher.plaintext)
	}
	if store.ciphertext == string(cipher.plaintext) {
		t.Fatal("credentials persisted as plaintext")
	}
}

func TestConnectDoesNotPersistInvalidCredentials(t *testing.T) {
	store := &testStore{}
	svc, _ := New(Options{Store: store, Validator: testValidator{err: provider.ErrInvalidCredentials}, Cipher: &testCipher{}})
	_, err := svc.Connect(context.Background(), ConnectRequest{WorkspaceID: "ws_1", Environment: domain.EnvironmentTest, ProviderKey: "woovi", Credentials: map[string]string{"app_id": "bad"}})
	if !errors.Is(err, provider.ErrInvalidCredentials) {
		t.Fatalf("Connect() error = %v", err)
	}
	if len(store.connections) != 0 {
		t.Fatalf("persisted invalid connection = %#v", store.connections)
	}
}
