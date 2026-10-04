// Package outboundhttp protects external provider requests at connection time.
// Production requests require HTTPS, public IPs and same-origin redirects.
package outboundhttp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var ErrUnsafeDestination = errors.New("external destination rejected")

// ProtectedMiddleware is implemented only by audited internal middleware that
// recursively protects every leaf transport, including fallback providers.
type ProtectedMiddleware interface {
	ProtectOutboundTransport() (http.RoundTripper, error)
}

type validatedTransport struct{ base *http.Transport }

func (t validatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, ErrUnsafeDestination
	}
	if _, err := ValidateURL(req.URL.String()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func ProtectTransport(base http.RoundTripper) (http.RoundTripper, error) {
	if base == nil {
		base = http.DefaultTransport
	}
	if t, ok := base.(*http.Transport); ok {
		return validatedTransport{protectedTransport(t)}, nil
	}
	if middleware, ok := base.(ProtectedMiddleware); ok {
		return middleware.ProtectOutboundTransport()
	}
	if production() {
		return nil, errors.New("external custom transport requires a protected connection boundary")
	}
	return base, nil
}

func production() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

// ValidateURL checks syntax without doing a second, potentially rebound DNS lookup.
// DialContext resolves and pins the actual destination before opening a connection.
func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return nil, ErrUnsafeDestination
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && !production()) {
		return nil, ErrUnsafeDestination
	}
	if production() {
		host := strings.ToLower(u.Hostname())
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.Contains(host, "%") {
			return nil, ErrUnsafeDestination
		}
		if ip, err := netip.ParseAddr(host); err == nil && !publicAddress(ip) {
			return nil, ErrUnsafeDestination
		}
	}
	return u, nil
}

func NewRequest(ctx context.Context, method, raw string, body io.Reader) (*http.Request, error) {
	u, err := ValidateURL(raw)
	if err != nil {
		return nil, err
	}
	// #nosec G704 -- ValidateURL and Do enforce HTTPS/public-IP/same-origin policy; DNS is pinned in safeDial.
	return http.NewRequestWithContext(ctx, method, u.String(), body)
}

// Bound retained pools even when callers supply a new transport per request.
var transports = struct {
	sync.Mutex
	pools map[*http.Transport]*http.Transport
	order []*http.Transport
}{pools: make(map[*http.Transport]*http.Transport)}

func protectedTransport(base *http.Transport) *http.Transport {
	transports.Lock()
	defer transports.Unlock()
	if cached, ok := transports.pools[base]; ok {
		return cached
	}
	t := base.Clone()
	t.Proxy = nil
	t.DialContext = safeDial
	t.DialTLSContext = nil
	t.DialTLS = nil
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = &tls.Config{}
	} else {
		t.TLSClientConfig = t.TLSClientConfig.Clone()
	}
	t.TLSClientConfig.InsecureSkipVerify = false
	t.TLSClientConfig.ServerName = ""
	if t.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.TLSClientConfig.MinVersion = tls.VersionTLS12
	}
	if len(transports.order) >= 64 {
		evicted := transports.order[0]
		transports.pools[evicted].CloseIdleConnections()
		delete(transports.pools, evicted)
		transports.order = transports.order[1:]
	}
	transports.pools[base] = t
	transports.order = append(transports.order, base)
	return t
}

// Do preserves caller timeouts and standard transport settings. Production
// custom transports must not replace the connection validation boundary.
func Do(client *http.Client, req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, ErrUnsafeDestination
	}
	if _, err := ValidateURL(req.URL.String()); err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	copyClient := *client
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	var err error
	copyClient.Transport, err = ProtectTransport(base)
	if err != nil {
		return nil, err
	}
	previousRedirect := client.CheckRedirect
	copyClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("external redirect limit exceeded")
		}
		if _, err := ValidateURL(next.URL.String()); err != nil {
			return err
		}
		if len(via) == 0 || !strings.EqualFold(next.URL.Scheme, via[0].URL.Scheme) || !strings.EqualFold(next.URL.Host, via[0].URL.Host) {
			return ErrUnsafeDestination
		}
		if previousRedirect != nil {
			return previousRedirect(next, via)
		}
		return nil
	}
	// #nosec G704 -- protectedTransport pins public DNS addresses; redirects are restricted above. Tests cover rebinding and private destinations.
	resp, err := copyClient.Do(req)
	if err != nil {
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			err = requestErr.Err
		}
		return resp, fmt.Errorf("external request failed: %w", err)
	}
	return resp, nil
}

func Post(client *http.Client, raw, contentType string, body io.Reader) (*http.Response, error) {
	req, err := NewRequest(context.Background(), http.MethodPost, raw, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return Do(client, req)
}

func safeDial(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	if !production() {
		return dialer.DialContext(ctx, network, address)
	}
	return dialPublic(ctx, network, address, net.DefaultResolver.LookupNetIP, dialer.DialContext)
}

func dialPublic(ctx context.Context, network, address string, lookup func(context.Context, string, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrUnsafeDestination
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	addresses, err := lookup(lookupCtx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, ErrUnsafeDestination
	}
	// Reject mixed public/private answers rather than accepting a convenient one.
	for _, ip := range addresses {
		if !publicAddress(ip) {
			return nil, ErrUnsafeDestination
		}
	}
	var lastErr error
	for _, ip := range addresses {
		conn, err := dial(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

var deniedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

func publicAddress(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Zone() != "" {
		return false
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range deniedPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
