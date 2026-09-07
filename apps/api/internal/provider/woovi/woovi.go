package woovi

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

const Key = "woovi"

type CredentialReader interface {
	GetProviderCredentialCiphertext(context.Context, string) (string, error)
}

type Cipher interface {
	Decrypt(string) ([]byte, error)
}

type Options struct {
	Credentials       CredentialReader
	Cipher            Cipher
	HTTPClient        *http.Client
	ProductionBaseURL string
	SandboxBaseURL    string
	KeyCacheTTL       time.Duration
}

type Factory struct {
	credentials CredentialReader
	cipher      Cipher
	client      *http.Client
	production  string
	sandbox     string
	keys        *publicKeyCache
}

func NewFactory(opts Options) (*Factory, error) {
	if opts.Credentials == nil || opts.Cipher == nil {
		return nil, fmt.Errorf("woovi credential reader and cipher are required")
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	production := strings.TrimRight(opts.ProductionBaseURL, "/")
	if production == "" {
		production = "https://api.woovi.com"
	}
	sandbox := strings.TrimRight(opts.SandboxBaseURL, "/")
	if sandbox == "" {
		sandbox = "https://api.woovi-sandbox.com"
	}
	ttl := opts.KeyCacheTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &Factory{
		credentials: opts.Credentials, cipher: opts.Cipher, client: client,
		production: production, sandbox: sandbox,
		keys: &publicKeyCache{client: client, ttl: ttl, entries: make(map[string]cachedKeys)},
	}, nil
}

func (f *Factory) ValidateCredentials(ctx context.Context, environment domain.Environment, credentials map[string]string) error {
	appID := strings.TrimSpace(credentials["app_id"])
	if appID == "" {
		return provider.ErrInvalidCredentials
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL(environment)+"/api/v1/company", nil)
	if err != nil {
		return provider.ErrCredentialValidationUnavailable
	}
	request.Header.Set("Authorization", appID)
	request.Header.Set("Accept", "application/json")
	response, err := f.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", provider.ErrCredentialValidationUnavailable, err)
	}
	defer response.Body.Close()
	_, _ = io.CopyN(io.Discard, response.Body, 4096)
	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		return nil
	case response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500:
		return fmt.Errorf("%w: http_%d", provider.ErrCredentialValidationUnavailable, response.StatusCode)
	default:
		return provider.ErrInvalidCredentials
	}
}

func (f *Factory) AdapterFor(ctx context.Context, connection domain.ProviderConnection) (provider.Adapter, error) {
	ciphertext, err := f.credentials.GetProviderCredentialCiphertext(ctx, connection.ID)
	if err != nil {
		return nil, err
	}
	plaintext, err := f.cipher.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt woovi credentials: %w", err)
	}
	var credentials map[string]string
	if err := json.Unmarshal(plaintext, &credentials); err != nil {
		return nil, fmt.Errorf("decode woovi credentials: %w", err)
	}
	appID := strings.TrimSpace(credentials["app_id"])
	if appID == "" {
		return nil, provider.ErrInvalidCredentials
	}
	return &Adapter{appID: appID, baseURL: f.baseURL(connection.Environment), client: f.client, keys: f.keys}, nil
}

func (f *Factory) baseURL(environment domain.Environment) string {
	if environment == domain.EnvironmentLive {
		return f.production
	}
	return f.sandbox
}

type Adapter struct {
	appID   string
	baseURL string
	client  *http.Client
	keys    *publicKeyCache
}

func (a *Adapter) Key() string { return Key }

type chargePayload struct {
	CorrelationID string `json:"correlationID"`
	Value         int64  `json:"value"`
	Comment       string `json:"comment,omitempty"`
}

type chargeEnvelope struct {
	Charge struct {
		Identifier    string    `json:"identifier"`
		CorrelationID string    `json:"correlationID"`
		Status        string    `json:"status"`
		BRCode        string    `json:"brCode"`
		ExpiresDate   time.Time `json:"expiresDate"`
	} `json:"charge"`
	BRCode string `json:"brCode"`
}

func (a *Adapter) CreatePix(ctx context.Context, request provider.CreateRequest) provider.CreateOutcome {
	payload, _ := json.Marshal(chargePayload{CorrelationID: request.AttemptID, Value: request.Amount, Comment: request.MerchantOrderID})
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/v1/charge", bytes.NewReader(payload))
	if err != nil {
		return provider.CreateOutcome{Kind: provider.CreateFailedTerminal, FailureCode: "woovi_request_build_failed"}
	}
	a.authorize(httpRequest)
	response, err := a.client.Do(httpRequest)
	if err != nil {
		return provider.CreateOutcome{Kind: provider.CreateUnknown, FailureCode: "woovi_transport_unknown"}
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return provider.CreateOutcome{Kind: provider.CreateUnknown, FailureCode: "woovi_response_read_unknown"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return classifyCreateStatus(response.StatusCode)
	}
	charge, err := decodeCharge(body)
	if err != nil || charge.Charge.Identifier == "" || copyPaste(charge) == "" {
		return provider.CreateOutcome{Kind: provider.CreateUnknown, FailureCode: "woovi_response_ambiguous"}
	}
	return provider.CreateOutcome{
		Kind:              provider.CreateSucceeded,
		ProviderPaymentID: charge.Charge.Identifier,
		Pix:               &domain.Pix{CopyPaste: copyPaste(charge), ExpiresAt: charge.Charge.ExpiresDate},
	}
}

func classifyCreateStatus(status int) provider.CreateOutcome {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return provider.CreateOutcome{Kind: provider.CreateFailedTerminal, FailureCode: "woovi_http_" + strconv.Itoa(status)}
	default:
		// For an effectful create, an error response is not assumed to prove the
		// charge was not created. Reconciliation by correlationID adjudicates it.
		return provider.CreateOutcome{Kind: provider.CreateUnknown, FailureCode: "woovi_http_" + strconv.Itoa(status) + "_unknown"}
	}
}

func (a *Adapter) Reconcile(ctx context.Context, request provider.ReconcileRequest) provider.ReconcileOutcome {
	endpoint := a.baseURL + "/api/v1/charge/" + url.PathEscape(request.AttemptID)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return provider.ReconcileOutcome{Kind: provider.ReconcileTerminal, FailureCode: "woovi_request_build_failed"}
	}
	a.authorize(httpRequest)
	response, err := a.client.Do(httpRequest)
	if err != nil {
		return provider.ReconcileOutcome{Kind: provider.ReconcileUnknown, FailureCode: "woovi_reconcile_transport"}
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return provider.ReconcileOutcome{Kind: provider.ReconcileUnknown, FailureCode: "woovi_reconcile_read"}
	}
	if response.StatusCode == http.StatusNotFound {
		// A single negative read after an ambiguous create is not treated as proof
		// that no charge exists. Until sandbox evidence establishes a safe
		// adjudication window, keep the attempt unknown and block cross-provider
		// fallback.
		return provider.ReconcileOutcome{Kind: provider.ReconcileUnknown, FailureCode: "woovi_charge_not_found_unconfirmed"}
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return provider.ReconcileOutcome{Kind: provider.ReconcileTerminal, FailureCode: "woovi_reconcile_auth"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return provider.ReconcileOutcome{Kind: provider.ReconcileUnknown, FailureCode: "woovi_reconcile_http_" + strconv.Itoa(response.StatusCode)}
	}
	charge, err := decodeCharge(body)
	if err != nil || charge.Charge.Identifier == "" {
		return provider.ReconcileOutcome{Kind: provider.ReconcileUnknown, FailureCode: "woovi_reconcile_ambiguous"}
	}
	status := strings.ToUpper(charge.Charge.Status)
	if status == "EXPIRED" {
		return provider.ReconcileOutcome{Kind: provider.ReconcileTerminal, ProviderPaymentID: charge.Charge.Identifier, FailureCode: "woovi_charge_expired"}
	}
	pixCode := copyPaste(charge)
	if pixCode == "" {
		return provider.ReconcileOutcome{Kind: provider.ReconcileUnknown, ProviderPaymentID: charge.Charge.Identifier, FailureCode: "woovi_reconcile_missing_brcode"}
	}
	return provider.ReconcileOutcome{
		Kind:              provider.ReconcileCreated,
		ProviderPaymentID: charge.Charge.Identifier,
		Pix:               &domain.Pix{CopyPaste: pixCode, ExpiresAt: charge.Charge.ExpiresDate},
	}
}

func (a *Adapter) ParseWebhook(ctx context.Context, request provider.WebhookRequest) (provider.WebhookEvent, error) {
	signature := headerValue(request.Headers, "x-webhook-signature")
	if signature == "" {
		return provider.WebhookEvent{}, provider.ErrInvalidWebhookSignature
	}
	keys, err := a.keys.get(ctx, a.baseURL)
	if err != nil {
		return provider.WebhookEvent{}, fmt.Errorf("load woovi webhook keys: %w", err)
	}
	if !verifySignature(keys, request.Body, signature) {
		return provider.WebhookEvent{}, provider.ErrInvalidWebhookSignature
	}
	var payload struct {
		Event  string `json:"event"`
		Charge struct {
			Identifier    string `json:"identifier"`
			CorrelationID string `json:"correlationID"`
			Status        string `json:"status"`
		} `json:"charge"`
	}
	if err := json.Unmarshal(request.Body, &payload); err != nil {
		return provider.WebhookEvent{}, provider.ErrUnsupportedWebhook
	}
	if payload.Event != "OPENPIX:CHARGE_COMPLETED" || payload.Charge.Identifier == "" {
		return provider.WebhookEvent{}, provider.ErrUnsupportedWebhook
	}
	return provider.WebhookEvent{
		ExternalEventID:   "woovi:charge_completed:" + payload.Charge.Identifier,
		Type:              "payment.paid",
		ProviderPaymentID: payload.Charge.Identifier,
	}, nil
}

func (a *Adapter) authorize(request *http.Request) {
	request.Header.Set("Authorization", a.appID)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
}

func decodeCharge(body []byte) (chargeEnvelope, error) {
	var envelope chargeEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return chargeEnvelope{}, err
	}
	return envelope, nil
}

func copyPaste(envelope chargeEnvelope) string {
	if envelope.Charge.BRCode != "" {
		return envelope.Charge.BRCode
	}
	return envelope.BRCode
}

func headerValue(headers map[string][]string, name string) string {
	for key, values := range headers {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

type cachedKeys struct {
	keys      []*rsa.PublicKey
	expiresAt time.Time
}
type publicKeyCache struct {
	mu      sync.Mutex
	client  *http.Client
	ttl     time.Duration
	entries map[string]cachedKeys
}

type keyEnvelope struct {
	PublicKeys []struct {
		Key string `json:"key"`
	} `json:"public_keys"`
}

func (c *publicKeyCache) get(ctx context.Context, baseURL string) ([]*rsa.PublicKey, error) {
	now := time.Now().UTC()
	c.mu.Lock()
	entry, ok := c.entries[baseURL]
	if ok && now.Before(entry.expiresAt) {
		keys := append([]*rsa.PublicKey(nil), entry.keys...)
		c.mu.Unlock()
		return keys, nil
	}
	stale := append([]*rsa.PublicKey(nil), entry.keys...)
	c.mu.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/webhook/public-keys", nil)
	if err != nil {
		return nil, err
	}
	response, err := c.client.Do(request)
	if err != nil {
		if len(stale) > 0 {
			return stale, nil
		}
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if len(stale) > 0 {
			return stale, nil
		}
		return nil, fmt.Errorf("public keys http status %d", response.StatusCode)
	}
	var envelope keyEnvelope
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		if len(stale) > 0 {
			return stale, nil
		}
		return nil, err
	}
	keys := make([]*rsa.PublicKey, 0, len(envelope.PublicKeys))
	for _, item := range envelope.PublicKeys {
		block, _ := pem.Decode([]byte(item.Key))
		if block == nil {
			continue
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			continue
		}
		key, ok := parsed.(*rsa.PublicKey)
		if ok {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		if len(stale) > 0 {
			return stale, nil
		}
		return nil, fmt.Errorf("no usable woovi webhook public key")
	}
	c.mu.Lock()
	c.entries[baseURL] = cachedKeys{keys: keys, expiresAt: now.Add(c.ttl)}
	c.mu.Unlock()
	return append([]*rsa.PublicKey(nil), keys...), nil
}

func verifySignature(keys []*rsa.PublicKey, body []byte, encoded string) bool {
	signature, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(body)
	for _, key := range keys {
		if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) == nil {
			return true
		}
	}
	return false
}

var _ provider.Integration = (*Factory)(nil)
var _ provider.WebhookAdapter = (*Adapter)(nil)
