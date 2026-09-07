package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/authn"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/webhookdelivery"
)

type MerchantWebhookService interface {
	SetEndpoint(context.Context, string, domain.Environment, string, string) (*webhookdelivery.Endpoint, error)
	GetEndpoint(context.Context, string, domain.Environment) (*webhookdelivery.Endpoint, error)
	DisableEndpoint(context.Context, string, domain.Environment) error
}

func registerMerchantWebhookRoutes(mux *http.ServeMux, opts Options) {
	if opts.MerchantWebhooks == nil || opts.APIKeys == nil {
		return
	}
	mux.HandleFunc("PUT /v1/webhook_endpoint", authorize(opts.APIKeys, handlePutWebhookEndpoint(opts.MerchantWebhooks)))
	mux.HandleFunc("GET /v1/webhook_endpoint", authorize(opts.APIKeys, handleGetWebhookEndpoint(opts.MerchantWebhooks)))
	mux.HandleFunc("DELETE /v1/webhook_endpoint", authorize(opts.APIKeys, handleDeleteWebhookEndpoint(opts.MerchantWebhooks)))
}

type webhookEndpointBody struct {
	URL           string `json:"url"`
	SigningSecret string `json:"signing_secret"`
}

type webhookEndpointResponse struct {
	ID          string             `json:"id"`
	URL         string             `json:"url"`
	Environment domain.Environment `json:"environment"`
	Enabled     bool               `json:"enabled"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

func handlePutWebhookEndpoint(service MerchantWebhookService) authorizedHandler {
	return func(w http.ResponseWriter, r *http.Request, principal authn.Principal) {
		var body webhookEndpointBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		body.URL = strings.TrimSpace(body.URL)
		if len([]byte(body.SigningSecret)) < 32 || len([]byte(body.SigningSecret)) > 512 {
			writeAPIError(w, http.StatusBadRequest, "invalid_signing_secret", "signing_secret must be between 32 and 512 bytes")
			return
		}
		endpoint, err := service.SetEndpoint(r.Context(), principal.WorkspaceID, principal.Environment, body.URL, body.SigningSecret)
		if err != nil {
			if errors.Is(err, webhookdelivery.ErrInvalidEndpoint) {
				writeAPIError(w, http.StatusBadRequest, "invalid_webhook_url", "webhook url is invalid or not allowed")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "webhook endpoint could not be saved")
			return
		}
		writeJSON(w, http.StatusOK, toWebhookEndpointResponse(endpoint))
	}
}

func handleGetWebhookEndpoint(service MerchantWebhookService) authorizedHandler {
	return func(w http.ResponseWriter, r *http.Request, principal authn.Principal) {
		endpoint, err := service.GetEndpoint(r.Context(), principal.WorkspaceID, principal.Environment)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				writeAPIError(w, http.StatusNotFound, "not_found", "webhook endpoint not found")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "webhook endpoint could not be loaded")
			return
		}
		writeJSON(w, http.StatusOK, toWebhookEndpointResponse(endpoint))
	}
}

func handleDeleteWebhookEndpoint(service MerchantWebhookService) authorizedHandler {
	return func(w http.ResponseWriter, r *http.Request, principal authn.Principal) {
		if err := service.DisableEndpoint(r.Context(), principal.WorkspaceID, principal.Environment); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				writeAPIError(w, http.StatusNotFound, "not_found", "webhook endpoint not found")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "webhook endpoint could not be disabled")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func toWebhookEndpointResponse(endpoint *webhookdelivery.Endpoint) webhookEndpointResponse {
	return webhookEndpointResponse{
		ID: endpoint.ID,
		URL: endpoint.URL,
		Environment: endpoint.Environment,
		Enabled: endpoint.Enabled,
		CreatedAt: endpoint.CreatedAt,
		UpdatedAt: endpoint.UpdatedAt,
	}
}
