package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/authn"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	providerfake "github.com/matspectrum-ai/conver-pay/apps/api/internal/provider/fake"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/store/memory"
)

type staticResolver map[string]authn.Principal

func (r staticResolver) ResolveAPIKey(_ context.Context, token string) (authn.Principal, error) {
	principal, ok := r[token]
	if !ok {
		return authn.Principal{}, authn.ErrUnauthorized
	}
	return principal, nil
}

func TestPaymentHTTPContract(t *testing.T) {
	store := memory.New()
	store.AddProviderConnection(domain.ProviderConnection{
		ID: "conn_a", WorkspaceID: "ws_a", ProviderKey: "provider_a",
		Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true,
		Circuit: domain.CircuitClosed, Priority: 1,
	})
	adapter := providerfake.New("provider_a", []provider.CreateOutcome{{
		Kind: provider.CreateSucceeded,
		ProviderPaymentID: "provider-payment-1",
		Pix: &domain.Pix{CopyPaste: "000201-test-pix", ExpiresAt: time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC)},
	}}, nil)
	service := orchestration.New(orchestration.Options{
		Repository: store,
		Providers:  providerfake.Registry{"provider_a": adapter},
	})
	server := New(Options{
		Payments: service,
		APIKeys: staticResolver{
			"cp_test_a_secret": {APIKeyID: "key_a", WorkspaceID: "ws_a", Environment: domain.EnvironmentTest},
			"cp_test_b_secret": {APIKeyID: "key_b", WorkspaceID: "ws_b", Environment: domain.EnvironmentTest},
			"cp_live_a_secret": {APIKeyID: "key_live_a", WorkspaceID: "ws_a", Environment: domain.EnvironmentLive},
		},
	})

	t.Run("missing API key is unauthorized", func(t *testing.T) {
		recorder := performRequest(server.Handler, http.MethodGet, "/v1/payment_intents/pi_missing", "", "", "")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})

	var paymentID string
	t.Run("create returns normalized Pix", func(t *testing.T) {
		recorder := performRequest(server.Handler, http.MethodPost, "/v1/payment_intents", "cp_test_a_secret", "idem-1", `{"merchant_order_id":"order_1","amount":5000,"currency":"BRL","metadata":{"source":"checkout"}}`)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
		}
		var response paymentResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		paymentID = response.ID
		if response.Status != domain.PaymentStatusAwaitingPayment || response.Pix == nil || response.Pix.CopyPaste != "000201-test-pix" {
			t.Fatalf("unexpected response = %#v", response)
		}
		if response.Metadata["source"] != "checkout" {
			t.Fatalf("metadata = %#v", response.Metadata)
		}
	})

	t.Run("identical retry returns same payment and does not call provider twice", func(t *testing.T) {
		recorder := performRequest(server.Handler, http.MethodPost, "/v1/payment_intents", "cp_test_a_secret", "idem-1", `{"merchant_order_id":"order_1","amount":5000,"currency":"BRL","metadata":{"source":"checkout"}}`)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
		}
		var response paymentResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.ID != paymentID {
			t.Fatalf("payment id = %q, want %q", response.ID, paymentID)
		}
		if adapter.CreateCalls() != 1 {
			t.Fatalf("provider create calls = %d, want 1", adapter.CreateCalls())
		}
	})

	t.Run("idempotency conflict returns 409", func(t *testing.T) {
		recorder := performRequest(server.Handler, http.MethodPost, "/v1/payment_intents", "cp_test_a_secret", "idem-1", `{"merchant_order_id":"order_1","amount":9000,"currency":"BRL"}`)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("same workspace can retrieve payment", func(t *testing.T) {
		recorder := performRequest(server.Handler, http.MethodGet, "/v1/payment_intents/"+paymentID, "cp_test_a_secret", "", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("other workspace receives 404", func(t *testing.T) {
		recorder := performRequest(server.Handler, http.MethodGet, "/v1/payment_intents/"+paymentID, "cp_test_b_secret", "", "")
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("attempts are tenant scoped", func(t *testing.T) {
		recorder := performRequest(server.Handler, http.MethodGet, "/v1/payment_intents/"+paymentID+"/attempts", "cp_test_a_secret", "", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			Data []attemptResponse `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode attempts: %v", err)
		}
		if len(response.Data) != 1 || response.Data[0].ProviderConnectionID != "conn_a" {
			t.Fatalf("attempts = %#v", response.Data)
		}
	})

	t.Run("live API key cannot route through test connection", func(t *testing.T) {
		recorder := performRequest(server.Handler, http.MethodPost, "/v1/payment_intents", "cp_live_a_secret", "idem-live", `{"merchant_order_id":"order_live","amount":5000,"currency":"BRL"}`)
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestCreatePaymentValidation(t *testing.T) {
	service := orchestration.New(orchestration.Options{Repository: memory.New(), Providers: providerfake.Registry{}})
	server := New(Options{
		Payments: service,
		APIKeys: staticResolver{"key": {APIKeyID: "k", WorkspaceID: "ws", Environment: domain.EnvironmentTest}},
	})

	tests := []struct {
		name string
		idempotency string
		body string
		want int
	}{
		{name: "missing idempotency", body: `{"merchant_order_id":"o","amount":1,"currency":"BRL"}`, want: http.StatusBadRequest},
		{name: "invalid amount", idempotency: "i1", body: `{"merchant_order_id":"o","amount":0,"currency":"BRL"}`, want: http.StatusBadRequest},
		{name: "unsupported currency", idempotency: "i2", body: `{"merchant_order_id":"o","amount":1,"currency":"USD"}`, want: http.StatusBadRequest},
		{name: "unknown field", idempotency: "i3", body: `{"merchant_order_id":"o","amount":1,"currency":"BRL","provider":"x"}`, want: http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(server.Handler, http.MethodPost, "/v1/payment_intents", "key", tc.idempotency, tc.body)
			if recorder.Code != tc.want {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func performRequest(handler http.Handler, method, path, token, idempotencyKey, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
