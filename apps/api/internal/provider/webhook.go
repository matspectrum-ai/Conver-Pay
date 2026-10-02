package provider

import "errors"

var (
	ErrInvalidWebhookSignature = errors.New("invalid webhook signature")
	ErrUnsupportedWebhook      = errors.New("unsupported webhook")
)

type WebhookRequest struct {
	Headers map[string][]string
	Body    []byte
}

type WebhookEvent struct {
	ExternalEventID   string
	Type              string
	ProviderPaymentID string
}

type WebhookAdapter = WebhookConnector
