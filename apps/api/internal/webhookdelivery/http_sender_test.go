package webhookdelivery

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPSenderSignsExactPayload(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	payload := []byte(`{"id":"evt_1","type":"payment.paid"}`)
	now := time.Date(2026, 9, 7, 9, 30, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || string(body) != string(payload) {
			t.Errorf("method=%s body=%s", r.Method, body)
		}
		if r.Header.Get("Conver-Pay-Event-ID") != "evt_1" {
			t.Errorf("event id = %q", r.Header.Get("Conver-Pay-Event-ID"))
		}
		timestamp := r.Header.Get("Conver-Pay-Timestamp")
		if timestamp != strconv.FormatInt(now.Unix(), 10) {
			t.Errorf("timestamp = %q", timestamp)
		}
		if !VerifySignature(secret, timestamp, body, r.Header.Get("Conver-Pay-Signature")) {
			t.Error("signature verification failed")
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sender := NewHTTPSender(HTTPOptions{AllowPrivateTargets: true, Timeout: time.Second})
	result := sender.Send(rContext(t), SendRequest{
		EventID: "evt_1", URL: server.URL, Payload: payload, Secret: secret, Now: now,
	})
	if result.ErrorCode != "" || result.HTTPStatus == nil || *result.HTTPStatus != http.StatusAccepted {
		t.Fatalf("result = %#v", result)
	}
}

func TestHTTPSenderDoesNotFollowRedirects(t *testing.T) {
	var sinkCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sink", http.StatusFound)
	})
	mux.HandleFunc("/sink", func(w http.ResponseWriter, _ *http.Request) {
		sinkCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	sender := NewHTTPSender(HTTPOptions{AllowPrivateTargets: true, Timeout: time.Second})
	result := sender.Send(rContext(t), SendRequest{
		EventID: "evt_redirect", URL: server.URL + "/start", Payload: []byte(`{}`),
		Secret: []byte("01234567890123456789012345678901"), Now: time.Now().UTC(),
	})
	if result.HTTPStatus == nil || *result.HTTPStatus != http.StatusFound {
		t.Fatalf("result = %#v", result)
	}
	if sinkCalls.Load() != 0 {
		t.Fatalf("redirect target calls = %d", sinkCalls.Load())
	}
}

func TestHTTPSenderBlocksPrivateTargetsByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender := NewHTTPSender(HTTPOptions{Timeout: time.Second})
	result := sender.Send(rContext(t), SendRequest{
		EventID: "evt_private", URL: server.URL, Payload: []byte(`{}`),
		Secret: []byte("01234567890123456789012345678901"), Now: time.Now().UTC(),
	})
	if result.ErrorCode != "target_not_public" || result.HTTPStatus != nil {
		t.Fatalf("result = %#v", result)
	}
}

func TestPublicIPClassification(t *testing.T) {
	private := []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "::1", "fd00::1"}
	for _, raw := range private {
		if isPublicWebhookIP(net.ParseIP(raw)) {
			t.Errorf("%s classified public", raw)
		}
	}
	if !isPublicWebhookIP(net.ParseIP("8.8.8.8")) {
		t.Error("8.8.8.8 classified non-public")
	}
}

func rContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}
