package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

type webhookStub struct {
	intent       *domain.PaymentIntent
	err          error
	connectionID string
	request      provider.WebhookRequest
}

func (s *webhookStub) ProcessProviderWebhook(_ context.Context, connectionID string, request provider.WebhookRequest) (*domain.PaymentIntent, error) {
	s.connectionID = connectionID
	s.request = request
	return s.intent, s.err
}

func TestProviderWebhookHTTPIngress(t *testing.T) {
	t.Run("success forwards raw request", func(t *testing.T) {
		stub := &webhookStub{intent: &domain.PaymentIntent{ID: "pi_1", Status: domain.PaymentStatusPaid}}
		server := New(Options{ProviderWebhooks: stub})
		req := httptest.NewRequest(http.MethodPost, "/v1/provider_webhooks/conn_a", strings.NewReader(`{"event":"paid"}`))
		req.Header.Set("X-Signature", "sig")
		recorder := httptest.NewRecorder()
		server.Handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
		}
		if stub.connectionID != "conn_a" || string(stub.request.Body) != `{"event":"paid"}` {
			t.Fatalf("forwarded connection=%q body=%q", stub.connectionID, string(stub.request.Body))
		}
		if len(stub.request.Headers["X-Signature"]) != 1 || stub.request.Headers["X-Signature"][0] != "sig" {
			t.Fatalf("headers = %#v", stub.request.Headers)
		}
	})

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid signature", err: provider.ErrInvalidWebhookSignature, want: http.StatusUnauthorized},
		{name: "unsupported event", err: provider.ErrUnsupportedWebhook, want: http.StatusBadRequest},
		{name: "not found", err: domain.ErrNotFound, want: http.StatusNotFound},
		{name: "state conflict", err: domain.ErrInvalidTransition, want: http.StatusConflict},
		{name: "internal", err: errors.New("database unavailable"), want: http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &webhookStub{err: tc.err}
			server := New(Options{ProviderWebhooks: stub})
			req := httptest.NewRequest(http.MethodPost, "/v1/provider_webhooks/conn_a", strings.NewReader(`{}`))
			recorder := httptest.NewRecorder()
			server.Handler.ServeHTTP(recorder, req)
			if recorder.Code != tc.want {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
