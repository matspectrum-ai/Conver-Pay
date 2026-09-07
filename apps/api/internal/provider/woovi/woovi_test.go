package woovi

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

func TestCreatePixUsesAttemptAsCorrelationIDAndParsesCharge(t *testing.T) {
	var received chargePayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/charge" || r.Method != http.MethodPost {
			t.Fatalf("request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "app-test" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"charge":{"identifier":"charge-1","correlationID":"pa_123","status":"ACTIVE","brCode":"000201-test","expiresDate":"2026-09-08T00:00:00Z"}}`))
	}))
	defer server.Close()

	adapter := &Adapter{appID: "app-test", baseURL: server.URL, client: server.Client()}
	outcome := adapter.CreatePix(context.Background(), provider.CreateRequest{
		PaymentIntentID: "pi_1", AttemptID: "pa_123", Amount: 1990, Currency: "BRL", MerchantOrderID: "order-1",
	})
	if outcome.Kind != provider.CreateSucceeded || outcome.ProviderPaymentID != "charge-1" || outcome.Pix == nil || outcome.Pix.CopyPaste != "000201-test" {
		t.Fatalf("outcome = %#v", outcome)
	}
	if received.CorrelationID != "pa_123" || received.Value != 1990 || received.Comment != "order-1" {
		t.Fatalf("payload = %#v", received)
	}
}

func TestCreatePixKeepsAmbiguousResponsesUnknown(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"server_error": func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", http.StatusInternalServerError) },
		"malformed_success": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"charge":{"identifier":"charge-created"}}`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			adapter := &Adapter{appID: "app", baseURL: server.URL, client: server.Client()}
			outcome := adapter.CreatePix(context.Background(), provider.CreateRequest{AttemptID: "pa_1", Amount: 100})
			if outcome.Kind != provider.CreateUnknown {
				t.Fatalf("outcome = %#v", outcome)
			}
		})
	}
}

func TestReconcileByCorrelationID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/charge/pa_exists":
			_, _ = w.Write([]byte(`{"charge":{"identifier":"charge-2","correlationID":"pa_exists","status":"ACTIVE","brCode":"pix-code","expiresDate":"2026-09-08T00:00:00Z"}}`))
		case "/api/v1/charge/pa_missing":
			http.NotFound(w, r)
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter := &Adapter{appID: "app", baseURL: server.URL, client: server.Client()}

	created := adapter.Reconcile(context.Background(), provider.ReconcileRequest{AttemptID: "pa_exists"})
	if created.Kind != provider.ReconcileCreated || created.ProviderPaymentID != "charge-2" || created.Pix == nil {
		t.Fatalf("created = %#v", created)
	}
	unconfirmed := adapter.Reconcile(context.Background(), provider.ReconcileRequest{AttemptID: "pa_missing"})
	if unconfirmed.Kind != provider.ReconcileUnknown || unconfirmed.FailureCode != "woovi_charge_not_found_unconfirmed" {
		t.Fatalf("unconfirmed = %#v", unconfirmed)
	}
}

func TestWebhookVerifiesRSASignatureAndCachesKeys(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey() error = %v", err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	var keyCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/webhook/public-keys" {
			http.NotFound(w, r)
			return
		}
		keyCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"public_keys": []map[string]any{{"key": publicPEM, "is_current": true}}})
	}))
	defer server.Close()

	cache := &publicKeyCache{client: server.Client(), ttl: time.Hour, entries: map[string]cachedKeys{}}
	adapter := &Adapter{baseURL: server.URL, client: server.Client(), keys: cache}
	body := []byte(`{"event":"OPENPIX:CHARGE_COMPLETED","charge":{"identifier":"charge-paid","correlationID":"pa_1","status":"COMPLETED"}}`)
	digest := sha256.Sum256(body)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15() error = %v", err)
	}
	request := provider.WebhookRequest{Headers: map[string][]string{"X-Webhook-Signature": {base64.StdEncoding.EncodeToString(signature)}}, Body: body}

	for i := 0; i < 2; i++ {
		event, err := adapter.ParseWebhook(context.Background(), request)
		if err != nil {
			t.Fatalf("ParseWebhook() error = %v", err)
		}
		if event.Type != "payment.paid" || event.ProviderPaymentID != "charge-paid" || event.ExternalEventID != "woovi:charge_completed:charge-paid" {
			t.Fatalf("event = %#v", event)
		}
	}
	if keyCalls.Load() != 1 {
		t.Fatalf("public key calls = %d, want 1", keyCalls.Load())
	}

	request.Headers["X-Webhook-Signature"] = []string{base64.StdEncoding.EncodeToString([]byte("bad"))}
	if _, err := adapter.ParseWebhook(context.Background(), request); !errors.Is(err, provider.ErrInvalidWebhookSignature) {
		t.Fatalf("invalid signature error = %v", err)
	}
}

func TestValidateCredentialsUsesEnvironmentBaseURL(t *testing.T) {
	for name, status := range map[string]int{"valid": http.StatusOK, "invalid": http.StatusUnauthorized, "unavailable": http.StatusServiceUnavailable} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/company" {
					http.NotFound(w, r)
					return
				}
				if r.Header.Get("Authorization") != "app-id" {
					t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			factory, err := NewFactory(Options{Credentials: fakeCredentialReader{}, Cipher: fakeCipher{}, HTTPClient: server.Client(), SandboxBaseURL: server.URL, ProductionBaseURL: server.URL})
			if err != nil {
				t.Fatalf("NewFactory() error = %v", err)
			}
			err = factory.ValidateCredentials(context.Background(), domain.EnvironmentTest, map[string]string{"app_id": "app-id"})
			switch name {
			case "valid":
				if err != nil {
					t.Fatalf("validation error = %v", err)
				}
			case "invalid":
				if !errors.Is(err, provider.ErrInvalidCredentials) {
					t.Fatalf("error = %v", err)
				}
			case "unavailable":
				if !errors.Is(err, provider.ErrCredentialValidationUnavailable) {
					t.Fatalf("error = %v", err)
				}
			}
		})
	}
}

type fakeCredentialReader struct{}

func (fakeCredentialReader) GetProviderCredentialCiphertext(context.Context, string) (string, error) {
	return "cipher", nil
}

type fakeCipher struct{}

func (fakeCipher) Decrypt(string) ([]byte, error) { return []byte(`{"app_id":"app"}`), nil }
