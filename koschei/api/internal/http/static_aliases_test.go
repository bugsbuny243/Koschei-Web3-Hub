package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRetiredCustomerRoutesRedirectToCustomerPanel(t *testing.T) {
	tests := []struct {
		path     string
		location string
	}{
		{path: "/scan", location: "/dashboard#transaction-preflight"},
		{path: "/scan/", location: "/dashboard#transaction-preflight"},
		{path: "/scan.html", location: "/dashboard#transaction-preflight"},
		{path: "/transaction-shield", location: "/dashboard#transaction-preflight"},
		{path: "/transaction-shield/", location: "/dashboard#transaction-preflight"},
		{path: "/transaction-shield.html", location: "/dashboard#transaction-preflight"},
		{path: "/safe-check", location: "/dashboard#capabilities"},
		{path: "/safe-check/", location: "/dashboard#capabilities"},
		{path: "/safe-check.html", location: "/dashboard#capabilities"},
		{path: "/security-radar", location: "/dashboard#capabilities"},
		{path: "/security-radar/", location: "/dashboard#capabilities"},
		{path: "/security-radar.html", location: "/dashboard#capabilities"},
		{path: "/security-radar?target=Mint123", location: "/dashboard#capabilities"},
		{path: "/launches", location: "/dashboard#capabilities"},
		{path: "/launches/", location: "/dashboard#capabilities"},
		{path: "/launches.html", location: "/dashboard#capabilities"},
	}

	mux := http.NewServeMux()
	registerStaticAliases(mux, t.TempDir())

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusPermanentRedirect {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusPermanentRedirect)
			}
			if location := response.Header().Get("Location"); location != test.location {
				t.Fatalf("Location = %q, want %q", location, test.location)
			}
		})
	}
}

func TestRetiredCustomerRedirectsRejectWrites(t *testing.T) {
	for _, route := range []string{"/scan", "/transaction-shield", "/safe-check", "/security-radar", "/launches"} {
		t.Run(route, func(t *testing.T) {
			mux := http.NewServeMux()
			registerStaticAliases(mux, t.TempDir())
			request := httptest.NewRequest(http.MethodPost, route, nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestRetiredScanFileCannotEscapeThroughStaticFallback(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("home"), 0o600); err != nil {
		t.Fatalf("write index fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "scan.html"), []byte("stale scan console"), 0o600); err != nil {
		t.Fatalf("write stale scan fixture: %v", err)
	}

	mux := http.NewServeMux()
	registerStatic(mux, staticDir)
	request := httptest.NewRequest(http.MethodGet, "/scan.html", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusPermanentRedirect {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusPermanentRedirect)
	}
	if location := response.Header().Get("Location"); location != "/dashboard#transaction-preflight" {
		t.Fatalf("Location = %q, want %q", location, "/dashboard#transaction-preflight")
	}
	if body := response.Body.String(); body == "stale scan console" {
		t.Fatal("retired scan file escaped through static fallback")
	}
}
