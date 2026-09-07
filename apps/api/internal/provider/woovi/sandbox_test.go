package woovi

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
)

// TestWooviSandboxProbe is intentionally opt-in. It exercises the real Woovi
// sandbox without ever placing the AppID in source control or CI fixtures.
func TestWooviSandboxProbe(t *testing.T) {
	appID := strings.TrimSpace(os.Getenv("WOOVI_SANDBOX_APP_ID"))
	if appID == "" {
		t.Skip("WOOVI_SANDBOX_APP_ID not configured")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	factory, err := NewFactory(Options{
		Credentials: fakeCredentialReader{}, Cipher: fakeCipher{}, HTTPClient: client,
		SandboxBaseURL: "https://api.woovi-sandbox.com",
	})
	if err != nil {
		t.Fatalf("NewFactory() error = %v", err)
	}
	if err := factory.ValidateCredentials(context.Background(), domain.EnvironmentTest, map[string]string{"app_id": appID}); err != nil {
		t.Fatalf("sandbox credential validation failed: %v", err)
	}

	correlationID := sandboxUUID(t)
	adapter := &Adapter{appID: appID, baseURL: "https://api.woovi-sandbox.com", client: client, keys: factory.keys}
	request := provider.CreateRequest{
		PaymentIntentID: "sandbox-probe", AttemptID: correlationID,
		Amount: 1, Currency: "BRL", MerchantOrderID: "conver-pay-sandbox-probe",
	}
	first := adapter.CreatePix(context.Background(), request)
	if first.Kind != provider.CreateSucceeded || first.ProviderPaymentID == "" || first.Pix == nil || first.Pix.CopyPaste == "" {
		t.Fatalf("first create outcome = %#v", first)
	}

	second := adapter.CreatePix(context.Background(), request)
	if second.Kind != provider.CreateSucceeded || second.ProviderPaymentID != first.ProviderPaymentID {
		t.Fatalf("idempotent create mismatch: first=%#v second=%#v", first, second)
	}

	reconciled := adapter.Reconcile(context.Background(), provider.ReconcileRequest{AttemptID: correlationID, ProviderPaymentID: first.ProviderPaymentID})
	if reconciled.Kind != provider.ReconcileCreated || reconciled.ProviderPaymentID != first.ProviderPaymentID || reconciled.Pix == nil {
		t.Fatalf("reconcile outcome = %#v", reconciled)
	}
}

func sandboxUUID(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
