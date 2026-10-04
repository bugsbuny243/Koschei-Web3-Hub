package outboundhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestProductionURLBoundary(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	for _, raw := range []string{"http://api.github.com/x", "https://user:secret@api.github.com/x", "https://127.0.0.1/x", "https://10.0.0.1/x", "https://169.254.169.254/x", "https://[::ffff:127.0.0.1]/x", "https://localhost/x", "https://metadata.local/x", "https://api.github.com/x#fragment"} {
		if _, err := ValidateURL(raw); !errors.Is(err, ErrUnsafeDestination) {
			t.Errorf("accepted %s: %v", raw, err)
		}
	}
	if _, err := ValidateURL("https://api.github.com/repos/owner/repo"); err != nil {
		t.Fatal(err)
	}
}

func TestPublicAddressRanges(t *testing.T) {
	for _, raw := range []string{"0.0.0.0", "127.0.0.1", "10.0.0.1", "100.64.1.1", "172.16.1.1", "192.168.1.1", "169.254.1.1", "198.18.0.1", "203.0.113.1", "224.0.0.1", "240.1.1.1", "::", "::1", "fd00::1", "fe80::1", "::ffff:10.0.0.1", "64:ff9b::a00:1", "2001:db8::1"} {
		if publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("allowed special IP %s", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("rejected public IP %s", raw)
		}
	}
}

func TestDialPinsDNSAndRejectsMixedAnswers(t *testing.T) {
	for _, ips := range [][]netip.Addr{
		{netip.MustParseAddr("127.0.0.1")},
		{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")},
		{netip.MustParseAddr("8.8.8.8")},
	} {
		called := false
		lookup := func(context.Context, string, string) ([]netip.Addr, error) { return ips, nil }
		dial := func(_ context.Context, _ string, address string) (net.Conn, error) {
			called = true
			if address != "8.8.8.8:443" {
				t.Fatalf("DNS name escaped to dial: %s", address)
			}
			return nil, errors.New("intentional test dial failure")
		}
		_, err := dialPublic(context.Background(), "tcp", "provider.example:443", lookup, dial)
		if len(ips) != 1 || ips[0].String() != "8.8.8.8" {
			if called || !errors.Is(err, ErrUnsafeDestination) {
				t.Fatalf("private/mixed DNS reached dial: called=%v err=%v", called, err)
			}
		} else if !called {
			t.Fatal("public resolved address was not dialed")
		}
	}
}

func TestRedirectDoesNotReachOtherOrigin(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	called := 0
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called++ }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	req, err := NewRequest(context.Background(), http.MethodGet, source.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := Do(nil, req)
	if resp != nil {
		resp.Body.Close()
	}
	if !errors.Is(err, ErrUnsafeDestination) || called != 0 {
		t.Fatalf("redirect escaped: calls=%d err=%v", called, err)
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("provider unavailable")
}

func TestFailureDoesNotExposeURLCredentials(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	req, err := NewRequest(context.Background(), http.MethodGet, "https://provider.example/rpc?api-key=private-test-fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Do(&http.Client{Transport: failingTransport{}}, req)
	if err == nil || strings.Contains(err.Error(), "private-test-fixture") {
		t.Fatalf("URL leaked: %v", err)
	}
}

func TestProductionDoesNotAllowCustomNetworkBoundary(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	req, err := NewRequest(context.Background(), http.MethodGet, "https://provider.example/rpc", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Do(&http.Client{Transport: failingTransport{}}, req); err == nil {
		t.Fatal("custom network bypass accepted")
	}
	base := http.DefaultTransport.(*http.Transport)
	protected := protectedTransport(base)
	if protected.Proxy != nil || protected.DialContext == nil || protected.DialTLSContext != nil {
		t.Fatal("proxy or alternate TLS dial can bypass the guard")
	}
}
