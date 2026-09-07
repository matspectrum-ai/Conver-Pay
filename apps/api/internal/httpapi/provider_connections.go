package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/authn"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/providerconnections"
)

type ProviderConnectionService interface {
	Connect(context.Context, providerconnections.ConnectRequest) (*domain.ProviderConnection, error)
	List(context.Context, string, domain.Environment) ([]domain.ProviderConnection, error)
	Disable(context.Context, string, string, domain.Environment) error
}

func registerProviderConnectionRoutes(mux *http.ServeMux, opts Options) {
	if opts.ProviderConnections == nil || opts.APIKeys == nil {
		return
	}
	mux.HandleFunc("POST /v1/provider_connections", authorize(opts.APIKeys, handleConnectProvider(opts.ProviderConnections)))
	mux.HandleFunc("GET /v1/provider_connections", authorize(opts.APIKeys, handleListProviders(opts.ProviderConnections)))
	mux.HandleFunc("DELETE /v1/provider_connections/{id}", authorize(opts.APIKeys, handleDisableProvider(opts.ProviderConnections)))
}

type connectProviderBody struct {
	ProviderKey string            `json:"provider_key"`
	Priority    int               `json:"priority"`
	Credentials map[string]string `json:"credentials"`
}

type providerConnectionResponse struct {
	ID               string              `json:"id"`
	ProviderKey      string              `json:"provider_key"`
	Environment      domain.Environment  `json:"environment"`
	Enabled          bool                `json:"enabled"`
	CredentialsValid bool                `json:"credentials_valid"`
	Circuit          domain.CircuitState `json:"circuit_state"`
	Priority         int                 `json:"priority"`
}

func handleConnectProvider(service ProviderConnectionService) authorizedHandler {
	return func(w http.ResponseWriter, r *http.Request, principal authn.Principal) {
		var body connectProviderBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		body.ProviderKey = strings.ToLower(strings.TrimSpace(body.ProviderKey))
		if body.ProviderKey == "" || body.Priority < 0 || len(body.Credentials) == 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_provider_connection", "provider_key, non-negative priority and credentials are required")
			return
		}
		connection, err := service.Connect(r.Context(), providerconnections.ConnectRequest{
			WorkspaceID: principal.WorkspaceID, Environment: principal.Environment,
			ProviderKey: body.ProviderKey, Priority: body.Priority, Credentials: body.Credentials,
		})
		if err != nil {
			writeProviderConnectionError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, toProviderConnectionResponse(connection))
	}
}

func handleListProviders(service ProviderConnectionService) authorizedHandler {
	return func(w http.ResponseWriter, r *http.Request, principal authn.Principal) {
		connections, err := service.List(r.Context(), principal.WorkspaceID, principal.Environment)
		if err != nil {
			writeProviderConnectionError(w, err)
			return
		}
		items := make([]providerConnectionResponse, 0, len(connections))
		for i := range connections {
			items = append(items, toProviderConnectionResponse(&connections[i]))
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

func handleDisableProvider(service ProviderConnectionService) authorizedHandler {
	return func(w http.ResponseWriter, r *http.Request, principal authn.Principal) {
		if err := service.Disable(r.Context(), principal.WorkspaceID, r.PathValue("id"), principal.Environment); err != nil {
			writeProviderConnectionError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func toProviderConnectionResponse(connection *domain.ProviderConnection) providerConnectionResponse {
	return providerConnectionResponse{
		ID: connection.ID, ProviderKey: connection.ProviderKey, Environment: connection.Environment,
		Enabled: connection.Enabled, CredentialsValid: connection.CredentialsValid,
		Circuit: connection.Circuit, Priority: connection.Priority,
	}
}

func writeProviderConnectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "provider connection not found")
	case errors.Is(err, provider.ErrInvalidCredentials):
		writeAPIError(w, http.StatusBadRequest, "invalid_provider_credentials", "provider credentials could not be validated")
	case errors.Is(err, provider.ErrCredentialValidationUnavailable):
		writeAPIError(w, http.StatusServiceUnavailable, "provider_validation_unavailable", "provider credential validation is temporarily unavailable")
	default:
		writeAPIError(w, http.StatusBadRequest, "invalid_provider_connection", "provider connection could not be configured")
	}
}
