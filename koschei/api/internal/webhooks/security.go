package webhooks

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var errUnsafeWebhookDestination = errors.New("unsafe webhook destination")

func ValidateEndpointURL(ctx context.Context, raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid webhook url: %w", err)
	}
	if u.User != nil || u.Hostname() == "" || u.Fragment != "" {
		return nil, errors.New("invalid webhook url")
	}
	allowHTTP := strings.EqualFold(strings.TrimSpace(os.Getenv("WEBHOOK_ALLOW_HTTP")), "true") && !strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	if u.Scheme != "https" && !(allowHTTP && u.Scheme == "http") {
		return nil, errors.New("webhook url must use https")
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return nil, errUnsafeWebhookDestination
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return nil, fmt.Errorf("webhook host lookup failed: %w", err)
	}
	if len(addresses) == 0 {
		return nil, errors.New("webhook host has no addresses")
	}
	for _, address := range addresses {
		if !isPublicIP(address.IP) {
			return nil, errUnsafeWebhookDestination
		}
	}
	return u, nil
}

func NewDeliveryClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		IdleConnTimeout:       45 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	transport.DialContext = safeDialContext
	return &http.Client{
		Timeout:   12 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many webhook redirects")
			}
			_, err := ValidateEndpointURL(req.Context(), req.URL.String())
			return err
		},
	}
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	for _, candidate := range addresses {
		if !isPublicIP(candidate.IP) {
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errUnsafeWebhookDestination
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		first, second := v4[0], v4[1]
		if first == 0 || first == 100 && second >= 64 && second <= 127 || first == 127 || first >= 224 {
			return false
		}
		if first == 169 && second == 254 || first == 192 && second == 0 || first == 198 && (second == 18 || second == 19) {
			return false
		}
		return true
	}
	return !ip.IsInterfaceLocalMulticast()
}
