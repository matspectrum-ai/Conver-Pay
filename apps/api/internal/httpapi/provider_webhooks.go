package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

type ProviderWebhookService interface {
	ProcessProviderWebhook(context.Context, string, provider.WebhookRequest) (*domain.PaymentIntent, error)
}

func registerProviderWebhookRoutes(mux *http.ServeMux, opts Options) {
	if opts.ProviderWebhooks == nil {
		return
	}
	mux.HandleFunc("POST /v1/provider_webhooks/{connection_id}", handleProviderWebhook(opts.ProviderWebhooks))
}

func handleProviderWebhook(service ProviderWebhookService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connectionID := r.PathValue("connection_id")
		if connectionID == "" {
			writeAPIError(w, http.StatusBadRequest, "invalid_connection", "provider connection is required")
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_webhook", "webhook body is invalid or too large")
			return
		}

		_, err = service.ProcessProviderWebhook(r.Context(), connectionID, provider.WebhookRequest{
			Headers: map[string][]string(r.Header.Clone()),
			Body:    body,
		})
		if err != nil {
			switch {
			case errors.Is(err, provider.ErrInvalidWebhookSignature):
				writeAPIError(w, http.StatusUnauthorized, "invalid_webhook_signature", "webhook signature is invalid")
			case errors.Is(err, provider.ErrUnsupportedWebhook):
				writeAPIError(w, http.StatusBadRequest, "unsupported_webhook", "webhook event is not supported")
			case errors.Is(err, domain.ErrNotFound):
				writeAPIError(w, http.StatusNotFound, "not_found", "provider connection or payment attempt was not found")
			case errors.Is(err, domain.ErrInvalidTransition):
				writeAPIError(w, http.StatusConflict, "event_conflict", "webhook is not valid for the current payment state")
			default:
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "webhook could not be processed")
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
