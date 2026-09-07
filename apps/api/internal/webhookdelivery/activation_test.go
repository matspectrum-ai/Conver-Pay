package webhookdelivery

import (
	"context"
	"testing"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

func TestEndpointActivationWindowSurvivesRotationAndResetsAfterDisable(t *testing.T) {
	repo := &testRepo{}
	service := newTestService(t, repo, nil, 0)
	secret := "01234567890123456789012345678901"

	first, err := service.SetEndpoint(context.Background(), "ws_1", domain.EnvironmentTest, "http://127.0.0.1:8081/first", secret)
	if err != nil {
		t.Fatalf("first SetEndpoint() error = %v", err)
	}
	firstActiveSince := first.ActiveSince

	rotated, err := service.SetEndpoint(context.Background(), "ws_1", domain.EnvironmentTest, "http://127.0.0.1:8081/rotated", secret)
	if err != nil {
		t.Fatalf("rotated SetEndpoint() error = %v", err)
	}
	if !rotated.ActiveSince.Equal(firstActiveSince) {
		t.Fatalf("rotation active_since = %s, want %s", rotated.ActiveSince, firstActiveSince)
	}

	if err := service.DisableEndpoint(context.Background(), "ws_1", domain.EnvironmentTest); err != nil {
		t.Fatalf("DisableEndpoint() error = %v", err)
	}
	reactivated, err := service.SetEndpoint(context.Background(), "ws_1", domain.EnvironmentTest, "http://127.0.0.1:8081/reactivated", secret)
	if err != nil {
		t.Fatalf("reactivated SetEndpoint() error = %v", err)
	}
	if !reactivated.ActiveSince.After(firstActiveSince) {
		t.Fatalf("reactivated active_since = %s, want after %s", reactivated.ActiveSince, firstActiveSince)
	}
}
