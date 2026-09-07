package webhookdelivery

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func validateEndpointURL(raw string, allowInsecureLocal bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("webhook url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid webhook url")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid webhook url")
	}
	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("invalid webhook url")
	}
	if parsed.Scheme == "https" {
		return parsed.String(), nil
	}
	if parsed.Scheme == "http" && allowInsecureLocal && isLocalHostname(host) {
		return parsed.String(), nil
	}
	return "", fmt.Errorf("webhook url must use https")
}

func isLocalHostname(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
