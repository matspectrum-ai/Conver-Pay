package provider

import "context"

// Capability describes one normalized downstream operation that a provider
// connector can safely fulfill. The router and control plane use capabilities
// to avoid selecting a connection that cannot execute the requested flow.
type Capability string

const (
	CapabilityPaymentCreate   Capability = "payment.create"
	CapabilityPixCreate       Capability = "pix.create"
	CapabilityPaymentQuery    Capability = "payment.query"
	CapabilityCreateReconcile Capability = "payment.create_reconcile"
	CapabilityWebhook         Capability = "webhook.receive"
	CapabilityRefund          Capability = "refund"
	CapabilityCancel          Capability = "cancel"
)

type Manifest struct {
	Key             string
	DisplayName     string
	Version         string
	Capabilities    []Capability
	CredentialTypes []string
}

func (m Manifest) Supports(capability Capability) bool {
	for _, item := range m.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

// ProviderConnector is the normalized data-plane contract between Conver Pay
// and a merchant-owned downstream payment provider. Provider-specific HTTP,
// authentication, payloads, response mapping and semantics stay behind this
// boundary.
type ProviderConnector interface {
	Manifest() Manifest
	CreatePix(context.Context, CreateRequest) CreateOutcome
	Reconcile(context.Context, ReconcileRequest) ReconcileOutcome
}

// WebhookConnector extends a provider connector with provider-specific
// authenticity verification and event normalization.
type WebhookConnector interface {
	ProviderConnector
	ParseWebhook(context.Context, WebhookRequest) (WebhookEvent, error)
}

// Compatibility aliases. New code should use the Connector names above.
type Connector = ProviderConnector
