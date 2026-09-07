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
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
)

type PaymentService interface {
	CreatePayment(context.Context, orchestration.CreatePaymentRequest) (*domain.PaymentIntent, error)
	GetPayment(context.Context, string, string) (*domain.PaymentIntent, error)
	ListPaymentAttempts(context.Context, string, string) ([]domain.PaymentAttempt, error)
}

func registerPaymentRoutes(mux *http.ServeMux, opts Options) {
	if opts.Payments == nil || opts.APIKeys == nil {
		return
	}

	mux.HandleFunc("POST /v1/payment_intents", authorize(opts.APIKeys, handleCreatePayment(opts.Payments)))
	mux.HandleFunc("GET /v1/payment_intents/{id}", authorize(opts.APIKeys, handleGetPayment(opts.Payments)))
	mux.HandleFunc("GET /v1/payment_intents/{id}/attempts", authorize(opts.APIKeys, handleListAttempts(opts.Payments)))
}

type authorizedHandler func(http.ResponseWriter, *http.Request, authn.Principal)

func authorize(resolver authn.Resolver, next authorizedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "valid bearer API key required")
			return
		}
		principal, err := resolver.ResolveAPIKey(r.Context(), token)
		if err != nil {
			if errors.Is(err, authn.ErrUnauthorized) {
				writeAPIError(w, http.StatusUnauthorized, "unauthorized", "valid bearer API key required")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "authentication unavailable")
			return
		}
		next(w, r, principal)
	}
}

func bearerToken(value string) string {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

type createPaymentBody struct {
	MerchantOrderID string            `json:"merchant_order_id"`
	Amount          int64             `json:"amount"`
	Currency        string            `json:"currency"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

func handleCreatePayment(service PaymentService) authorizedHandler {
	return func(w http.ResponseWriter, r *http.Request, principal authn.Principal) {
		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if idempotencyKey == "" || len(idempotencyKey) > 255 {
			writeAPIError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key is required and must be at most 255 characters")
			return
		}

		var body createPaymentBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		body.MerchantOrderID = strings.TrimSpace(body.MerchantOrderID)
		body.Currency = strings.ToUpper(strings.TrimSpace(body.Currency))
		if body.MerchantOrderID == "" || len(body.MerchantOrderID) > 255 {
			writeAPIError(w, http.StatusBadRequest, "invalid_merchant_order_id", "merchant_order_id is required and must be at most 255 characters")
			return
		}
		if body.Amount <= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_amount", "amount must be a positive integer in centavos")
			return
		}
		if body.Currency != "BRL" {
			writeAPIError(w, http.StatusBadRequest, "unsupported_currency", "Pix v1 supports BRL only")
			return
		}

		intent, err := service.CreatePayment(r.Context(), orchestration.CreatePaymentRequest{
			WorkspaceID:     principal.WorkspaceID,
			IdempotencyKey:  idempotencyKey,
			MerchantOrderID: body.MerchantOrderID,
			Amount:          body.Amount,
			Currency:        body.Currency,
			Metadata:        body.Metadata,
			Environment:     principal.Environment,
		})
		if err != nil {
			writePaymentError(w, err)
			return
		}

		status := http.StatusOK
		if intent.Status == domain.PaymentStatusReconciling {
			status = http.StatusAccepted
		}
		writeJSON(w, status, toPaymentResponse(intent))
	}
}

func handleGetPayment(service PaymentService) authorizedHandler {
	return func(w http.ResponseWriter, r *http.Request, principal authn.Principal) {
		intent, err := service.GetPayment(r.Context(), principal.WorkspaceID, r.PathValue("id"))
		if err != nil {
			writePaymentError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toPaymentResponse(intent))
	}
}

func handleListAttempts(service PaymentService) authorizedHandler {
	return func(w http.ResponseWriter, r *http.Request, principal authn.Principal) {
		attempts, err := service.ListPaymentAttempts(r.Context(), principal.WorkspaceID, r.PathValue("id"))
		if err != nil {
			writePaymentError(w, err)
			return
		}
		items := make([]attemptResponse, 0, len(attempts))
		for i := range attempts {
			items = append(items, toAttemptResponse(&attempts[i]))
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

type pixResponse struct {
	CopyPaste string    `json:"copy_paste"`
	ExpiresAt time.Time `json:"expires_at"`
}

type paymentResponse struct {
	ID                 string            `json:"id"`
	MerchantOrderID    string            `json:"merchant_order_id"`
	Amount             int64             `json:"amount"`
	Currency           string            `json:"currency"`
	Status             domain.PaymentStatus `json:"status"`
	Pix                *pixResponse      `json:"pix,omitempty"`
	Recovered          bool              `json:"recovered"`
	RecoveredAmount    int64             `json:"recovered_amount"`
	FailureCode        string            `json:"failure_code,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	PaidAt             *time.Time        `json:"paid_at,omitempty"`
}

func toPaymentResponse(intent *domain.PaymentIntent) paymentResponse {
	response := paymentResponse{
		ID: intent.ID, MerchantOrderID: intent.MerchantOrderID, Amount: intent.Amount,
		Currency: intent.Currency, Status: intent.Status, Recovered: intent.Recovered,
		RecoveredAmount: intent.RecoveredAmount, FailureCode: intent.FailureCode,
		Metadata: intent.Metadata, CreatedAt: intent.CreatedAt, UpdatedAt: intent.UpdatedAt,
		PaidAt: intent.PaidAt,
	}
	if intent.Pix != nil {
		response.Pix = &pixResponse{CopyPaste: intent.Pix.CopyPaste, ExpiresAt: intent.Pix.ExpiresAt}
	}
	return response
}

type attemptResponse struct {
	ID                   string               `json:"id"`
	ProviderConnectionID string               `json:"provider_connection_id"`
	Sequence             int                  `json:"sequence"`
	Status               domain.AttemptStatus `json:"status"`
	FailureCode          string               `json:"failure_code,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

func toAttemptResponse(attempt *domain.PaymentAttempt) attemptResponse {
	return attemptResponse{
		ID: attempt.ID, ProviderConnectionID: attempt.ProviderConnectionID,
		Sequence: attempt.Sequence, Status: attempt.Status, FailureCode: attempt.FailureCode,
		CreatedAt: attempt.CreatedAt, UpdatedAt: attempt.UpdatedAt,
	}
}

func writePaymentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "payment not found")
	case errors.Is(err, domain.ErrIdempotencyConflict):
		writeAPIError(w, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was already used with a different request")
	case errors.Is(err, domain.ErrNoEligibleProvider):
		writeAPIError(w, http.StatusServiceUnavailable, "no_eligible_provider", "no payment provider is currently eligible")
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "request could not be completed")
	}
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
