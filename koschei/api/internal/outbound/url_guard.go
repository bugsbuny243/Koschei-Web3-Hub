package outbound

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// ValidateOperatorURL is the shared SSRF boundary for operator-configured
// outbound endpoints. Production only accepts HTTPS destinations that do not
// resolve to loopback, private, link-local, unspecified, or multicast IPs.
// Non-production permits loopback HTTP so local integration tests can use
// httptest servers without weakening the production boundary.
func ValidateOperatorURL(ctx context.Context, raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("outbound endpoint is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid outbound endpoint")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("outbound endpoint userinfo is not allowed")
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	production := strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	loopbackHost := host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "::1" || strings.HasPrefix(host, "127.")

	switch strings.ToLower(parsed.Scheme) {
	case "https":
	case "http":
		if production || !loopbackHost {
			return nil, fmt.Errorf("outbound endpoint must use HTTPS")
		}
	default:
		return nil, fmt.Errorf("outbound endpoint must use HTTPS")
	}

	if ip := net.ParseIP(host); ip != nil {
		if forbiddenIP(ip) && !(loopbackHost && !production) {
			return nil, fmt.Errorf("outbound endpoint IP is not public")
		}
		return parsed, nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		if !production && loopbackHost {
			return parsed, nil
		}
		return nil, fmt.Errorf("outbound endpoint host is not public")
	}

	if production {
		if ctx == nil {
			ctx = context.Background()
		}
		resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(resolved) == 0 {
			if err == nil {
				err = fmt.Errorf("no addresses returned")
			}
			return nil, fmt.Errorf("resolve outbound endpoint host: %w", err)
		}
		for _, candidate := range resolved {
			if forbiddenIP(candidate.IP) {
				return nil, fmt.Errorf("outbound endpoint resolved to a non-public IP")
			}
		}
	}
	return parsed, nil
}

// ValidateFixedHTTPSHost validates a URL whose destination authority belongs to
// a small code-owned allowlist. Paths and queries may vary; hosts may not.
func ValidateFixedHTTPSHost(raw string, allowedHosts ...string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid fixed-host outbound endpoint")
	}
	if !strings.EqualFold(parsed.Scheme, "https") || parsed.User != nil {
		return nil, fmt.Errorf("fixed-host outbound endpoint must use HTTPS without userinfo")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	for _, allowed := range allowedHosts {
		if host == strings.ToLower(strings.TrimSpace(allowed)) {
			return parsed, nil
		}
	}
	return nil, fmt.Errorf("fixed-host outbound endpoint host is not allowed")
}

// HardenOperatorClient clones a client and validates every redirect target
// through the same boundary. Callers still validate the initial endpoint before
// constructing a request.
func HardenOperatorClient(ctx context.Context, client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	clone := *client
	previous := client.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many outbound redirects")
		}
		if req == nil || req.URL == nil {
			return fmt.Errorf("outbound redirect target is unavailable")
		}
		if _, err := ValidateOperatorURL(ctx, req.URL.String()); err != nil {
			return fmt.Errorf("outbound redirect rejected: %w", err)
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	return &clone
}

func forbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
