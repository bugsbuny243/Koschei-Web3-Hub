package requestmeta

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPIgnoresForwardedHeaderFromUntrustedPeer(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	r := httptest.NewRequest("GET", "https://example.test", nil)
	r.RemoteAddr = "203.0.113.8:41234"
	r.Header.Set("X-Forwarded-For", "198.51.100.9")
	if got := ClientIP(r); got != "203.0.113.8" {
		t.Fatalf("ClientIP() = %q, want direct peer", got)
	}
}

func TestClientIPUsesForwardedChainOnlyBehindTrustedProxy(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,192.0.2.10")
	r := httptest.NewRequest("GET", "https://example.test", nil)
	r.RemoteAddr = "10.2.3.4:443"
	r.Header.Set("X-Forwarded-For", "198.51.100.7, 192.0.2.10")
	if got := ClientIP(r); got != "198.51.100.7" {
		t.Fatalf("ClientIP() = %q, want original untrusted client", got)
	}
}

func TestClientIPRejectsMalformedForwardedChain(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	r := httptest.NewRequest("GET", "https://example.test", nil)
	r.RemoteAddr = "10.2.3.4:443"
	r.Header.Set("X-Forwarded-For", "198.51.100.7, not-an-ip")
	if got := ClientIP(r); got != "10.2.3.4" {
		t.Fatalf("ClientIP() = %q, want trusted peer fallback", got)
	}
}
