package webhookdelivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var errTargetNotPublic = errors.New("webhook target is not public")

type HTTPOptions struct {
	Timeout             time.Duration
	AllowPrivateTargets bool
	Resolver            *net.Resolver
}

type HTTPSender struct {
	client              *http.Client
	allowPrivateTargets bool
	resolver            *net.Resolver
}

func NewHTTPSender(opts HTTPOptions) *HTTPSender {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	resolver := opts.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	sender := &HTTPSender{allowPrivateTargets: opts.AllowPrivateTargets, resolver: resolver}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := resolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, candidate := range ips {
				if !sender.allowPrivateTargets && !isPublicWebhookIP(candidate.IP) {
					continue
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
			}
			return nil, errTargetNotPublic
		},
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2: true,
		MaxIdleConns: 50,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout: 30 * time.Second,
		ResponseHeaderTimeout: timeout,
		DisableCompression: true,
	}
	sender.client = &http.Client{
		Timeout: timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return sender
}

func (s *HTTPSender) Send(ctx context.Context, request SendRequest) SendResult {
	started := time.Now()
	timestamp := strconv.FormatInt(request.Now.Unix(), 10)
	signature := signWebhook(request.Secret, timestamp, request.Payload)

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, request.URL, bytes.NewReader(request.Payload))
	if err != nil {
		return SendResult{Latency: time.Since(started), ErrorCode: "invalid_url"}
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("User-Agent", "Conver-Pay-Webhooks/1.0")
	httpRequest.Header.Set("Conver-Pay-Event-ID", request.EventID)
	httpRequest.Header.Set("Conver-Pay-Timestamp", timestamp)
	httpRequest.Header.Set("Conver-Pay-Signature", "v1="+signature)

	response, err := s.client.Do(httpRequest)
	latency := time.Since(started)
	if err != nil {
		return SendResult{Latency: latency, ErrorCode: classifyNetworkError(err)}
	}
	defer response.Body.Close()
	_, _ = io.CopyN(io.Discard, response.Body, 4096)
	status := response.StatusCode
	return SendResult{HTTPStatus: &status, Latency: latency}
}

func signWebhook(secret []byte, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func classifyNetworkError(err error) string {
	if errors.Is(err, errTargetNotPublic) || strings.Contains(err.Error(), errTargetNotPublic.Error()) {
		return "target_not_public"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "request_timeout"
	}
	return "network_error"
}

func isPublicWebhookIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 100.64.0.0/10 carrier-grade NAT and documentation ranges are not valid public webhook targets.
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
		if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2 {
			return false
		}
		if ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100 {
			return false
		}
		if ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113 {
			return false
		}
	}
	return true
}

func VerifySignature(secret []byte, timestamp string, payload []byte, header string) bool {
	if !strings.HasPrefix(header, "v1=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(header, "v1="))
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(signWebhook(secret, timestamp, payload))
	if err != nil {
		return false
	}
	return hmac.Equal(provided, expected)
}

var _ Sender = (*HTTPSender)(nil)

func _compileGuard() {
	_ = fmt.Sprintf
}
