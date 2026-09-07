package webhookdelivery

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var ErrInvalidEndpoint = errors.New("invalid webhook endpoint")

func invalidEndpoint(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidEndpoint, reason)
}

func validateEndpointURL(raw string, allowInsecureLocal bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", invalidEndpoint("url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", invalidEndpoint("url is invalid")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", invalidEndpoint("url is invalid")
	}
	host := parsed.Hostname()
	if host == "" {
		return "", invalidEndpoint("url is invalid")
	}
	if parsed.Scheme == "https" {
		return parsed.String(), nil
	}
	if parsed.Scheme == "http" && allowInsecureLocal && isLocalHostname(host) {
		return parsed.String(), nil
	}
	return "", invalidEndpoint("url must use https")
}

func isLocalHostname(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
