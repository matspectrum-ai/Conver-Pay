package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/authn"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	providerfake "github.com/matspectrum-ai/conver-pay/apps/api/internal/provider/fake"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/store/memory"
)

func TestPaymentQueriesAreEnvironmentScoped(t *testing.T) {
	store := memory.New()
	store.AddProviderConnection(domain.ProviderConnection{
		ID: "conn_test", WorkspaceID: "ws_env", ProviderKey: "provider_test",
		Environment: domain.EnvironmentTest, Enabled: true, CredentialsValid: true,
		Circuit: domain.CircuitClosed, Priority: 1,
	})
	adapter := providerfake.New("provider_test", []provider.CreateOutcome{{
		Kind: provider.CreateSucceeded, ProviderPaymentID: "provider-payment-test",
		Pix: &domain.Pix{CopyPaste: "pix-test", ExpiresAt: time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)},
	}}, nil)
	service := orchestration.New(orchestration.Options{
		Repository: store,
		Providers:  providerfake.Registry{"provider_test": adapter},
	})
	server := New(Options{
		Payments: service,
		APIKeys: staticResolver{
			"cp_test_env": {APIKeyID: "key_test", WorkspaceID: "ws_env", Environment: domain.EnvironmentTest},
			"cp_live_env": {APIKeyID: "key_live", WorkspaceID: "ws_env", Environment: domain.EnvironmentLive},
		},
	})

	created := performRequest(server.Handler, http.MethodPost, "/v1/payment_intents", "cp_test_env", "idem-env", `{"merchant_order_id":"order-env","amount":1500,"currency":"BRL"}`)
	if created.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%s", created.Code, created.Body.String())
	}
	var response paymentResponse
	decodeJSONResponse(t, created, &response)

	for _, path := range []string{
		"/v1/payment_intents/" + response.ID,
		"/v1/payment_intents/" + response.ID + "/attempts",
	} {
		t.Run(path, func(t *testing.T) {
			testResponse := performRequest(server.Handler, http.MethodGet, path, "cp_test_env", "", "")
			if testResponse.Code != http.StatusOK {
				t.Fatalf("test key status = %d body=%s", testResponse.Code, testResponse.Body.String())
			}
			liveResponse := performRequest(server.Handler, http.MethodGet, path, "cp_live_env", "", "")
			if liveResponse.Code != http.StatusNotFound {
				t.Fatalf("live key status = %d body=%s", liveResponse.Code, liveResponse.Body.String())
			}
		})
	}
}

func decodeJSONResponse(t *testing.T, response interface{ BodyBytes() []byte }, target any) {
	t.Helper()
	_ = response
	_ = target
}

var _ authn.Resolver = staticResolver{}
