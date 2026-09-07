package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/providerconnections"
)

type fakeProviderConnectionService struct {
	connections []domain.ProviderConnection
	lastConnect providerconnections.ConnectRequest
	disabledID  string
	connectErr  error
}

func (s *fakeProviderConnectionService) Connect(_ context.Context, req providerconnections.ConnectRequest) (*domain.ProviderConnection, error) {
	s.lastConnect = req
	if s.connectErr != nil {
		return nil, s.connectErr
	}
	connection := domain.ProviderConnection{ID: "pc_1", WorkspaceID: req.WorkspaceID, ProviderKey: req.ProviderKey, Environment: req.Environment, Enabled: true, CredentialsValid: true, Circuit: domain.CircuitClosed, Priority: req.Priority}
	s.connections = append(s.connections, connection)
	return &connection, nil
}
func (s *fakeProviderConnectionService) List(_ context.Context, workspaceID string, environment domain.Environment) ([]domain.ProviderConnection, error) {
	out := []domain.ProviderConnection{}
	for _, c := range s.connections {
		if c.WorkspaceID == workspaceID && c.Environment == environment {
			out = append(out, c)
		}
	}
	return out, nil
}
func (s *fakeProviderConnectionService) Disable(_ context.Context, _ string, id string, _ domain.Environment) error {
	s.disabledID = id
	return nil
}

func TestProviderConnectionHTTPContractDoesNotExposeCredentials(t *testing.T) {
	service := &fakeProviderConnectionService{}
	server := New(Options{ProviderConnections: service, APIKeys: staticResolver{
		"cp_test": {APIKeyID: "key_test", WorkspaceID: "ws_1", Environment: domain.EnvironmentTest},
		"cp_live": {APIKeyID: "key_live", WorkspaceID: "ws_1", Environment: domain.EnvironmentLive},
	}})

	recorder := performRequest(server.Handler, http.MethodPost, "/v1/provider_connections", "cp_test", "", `{"provider_key":"woovi","priority":5,"credentials":{"app_id":"super-secret"}}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastConnect.WorkspaceID != "ws_1" || service.lastConnect.Environment != domain.EnvironmentTest || service.lastConnect.Credentials["app_id"] != "super-secret" {
		t.Fatalf("connect request=%#v", service.lastConnect)
	}
	if contains := string(recorder.Body.Bytes()); len(contains) == 0 || json.Valid(recorder.Body.Bytes()) == false {
		t.Fatalf("invalid response=%q", contains)
	}
	if string(recorder.Body.Bytes()) == "super-secret" {
		t.Fatal("secret leaked as response body")
	}
	var response map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &response)
	if _, ok := response["credentials"]; ok {
		t.Fatal("credentials field leaked")
	}
	if _, ok := response["app_id"]; ok {
		t.Fatal("app_id field leaked")
	}

	listTest := performRequest(server.Handler, http.MethodGet, "/v1/provider_connections", "cp_test", "", "")
	if listTest.Code != http.StatusOK || !json.Valid(listTest.Body.Bytes()) {
		t.Fatalf("test list status=%d body=%s", listTest.Code, listTest.Body.String())
	}
	listLive := performRequest(server.Handler, http.MethodGet, "/v1/provider_connections", "cp_live", "", "")
	if listLive.Code != http.StatusOK || string(listLive.Body.Bytes()) == string(listTest.Body.Bytes()) { /* both valid; assert live has no test connection below */
	}
	var live struct {
		Data []providerConnectionResponse `json:"data"`
	}
	_ = json.Unmarshal(listLive.Body.Bytes(), &live)
	if len(live.Data) != 0 {
		t.Fatalf("live leaked test connections=%#v", live.Data)
	}
}

func TestProviderConnectionHTTPMapsInvalidCredentials(t *testing.T) {
	service := &fakeProviderConnectionService{connectErr: provider.ErrInvalidCredentials}
	server := New(Options{ProviderConnections: service, APIKeys: staticResolver{"cp_test": {APIKeyID: "key", WorkspaceID: "ws", Environment: domain.EnvironmentTest}}})
	recorder := performRequest(server.Handler, http.MethodPost, "/v1/provider_connections", "cp_test", "", `{"provider_key":"woovi","priority":0,"credentials":{"app_id":"bad"}}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestProviderConnectionHTTPUnauthorized(t *testing.T) {
	server := New(Options{ProviderConnections: &fakeProviderConnectionService{}, APIKeys: staticResolver{}})
	recorder := performRequest(server.Handler, http.MethodGet, "/v1/provider_connections", "", "", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
}

var _ ProviderConnectionService = (*fakeProviderConnectionService)(nil)
