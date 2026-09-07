package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/webhookdelivery"
)

type endpointHTTPStub struct {
	lastWorkspace   string
	lastEnvironment domain.Environment
	lastURL         string
	lastSecret      string
	endpoint        *webhookdelivery.Endpoint
}

func (s *endpointHTTPStub) SetEndpoint(_ context.Context, workspace string, environment domain.Environment, url, secret string) (*webhookdelivery.Endpoint, error) {
	s.lastWorkspace = workspace
	s.lastEnvironment = environment
	s.lastURL = url
	s.lastSecret = secret
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	s.endpoint = &webhookdelivery.Endpoint{
		ID: "whe_1", WorkspaceID: workspace, Environment: environment, URL: url,
		SigningSecretCiphertext: "v1.secret-ciphertext", Enabled: true,
		CreatedAt: now, UpdatedAt: now,
	}
	return s.endpoint, nil
}

func (s *endpointHTTPStub) GetEndpoint(_ context.Context, workspace string, environment domain.Environment) (*webhookdelivery.Endpoint, error) {
	s.lastWorkspace = workspace
	s.lastEnvironment = environment
	if s.endpoint == nil || s.endpoint.WorkspaceID != workspace || s.endpoint.Environment != environment {
		return nil, domain.ErrNotFound
	}
	return s.endpoint, nil
}

func (s *endpointHTTPStub) DisableEndpoint(_ context.Context, workspace string, environment domain.Environment) error {
	s.lastWorkspace = workspace
	s.lastEnvironment = environment
	if s.endpoint == nil || s.endpoint.WorkspaceID != workspace || s.endpoint.Environment != environment {
		return domain.ErrNotFound
	}
	s.endpoint.Enabled = false
	return nil
}

func TestWebhookEndpointAPIUsesAPIKeyScopeAndNeverReturnsSecret(t *testing.T) {
	stub := &endpointHTTPStub{}
	server := New(Options{
		MerchantWebhooks: stub,
		APIKeys: staticResolver{
			"cp_test_webhook": {APIKeyID: "key_test", WorkspaceID: "ws_webhook", Environment: domain.EnvironmentTest},
			"cp_live_webhook": {APIKeyID: "key_live", WorkspaceID: "ws_webhook", Environment: domain.EnvironmentLive},
		},
	})
	secret := "01234567890123456789012345678901"
	put := performRequest(server.Handler, http.MethodPut, "/v1/webhook_endpoint", "cp_test_webhook", "", `{"url":"https://merchant.example/hooks/conver","signing_secret":"`+secret+`"}`)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", put.Code, put.Body.String())
	}
	if stub.lastWorkspace != "ws_webhook" || stub.lastEnvironment != domain.EnvironmentTest || stub.lastSecret != secret {
		t.Fatalf("scope workspace=%s env=%s", stub.lastWorkspace, stub.lastEnvironment)
	}
	if strings.Contains(put.Body.String(), secret) || strings.Contains(put.Body.String(), "ciphertext") || strings.Contains(put.Body.String(), "signing_secret") {
		t.Fatalf("secret leaked in response: %s", put.Body.String())
	}
	var response webhookEndpointResponse
	if err := json.Unmarshal(put.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Environment != domain.EnvironmentTest || response.URL != "https://merchant.example/hooks/conver" || !response.Enabled {
		t.Fatalf("response = %#v", response)
	}

	liveGet := performRequest(server.Handler, http.MethodGet, "/v1/webhook_endpoint", "cp_live_webhook", "", "")
	if liveGet.Code != http.StatusNotFound {
		t.Fatalf("live GET status=%d body=%s", liveGet.Code, liveGet.Body.String())
	}
	if stub.lastEnvironment != domain.EnvironmentLive {
		t.Fatalf("live GET environment = %s", stub.lastEnvironment)
	}
}

func TestWebhookEndpointAPIRejectsShortSecret(t *testing.T) {
	stub := &endpointHTTPStub{}
	server := New(Options{
		MerchantWebhooks: stub,
		APIKeys: staticResolver{"key": {APIKeyID: "key", WorkspaceID: "ws", Environment: domain.EnvironmentTest}},
	})
	response := performRequest(server.Handler, http.MethodPut, "/v1/webhook_endpoint", "key", "", `{"url":"https://merchant.example/hook","signing_secret":"short"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if stub.lastSecret != "" {
		t.Fatal("service called for invalid signing secret")
	}
}
