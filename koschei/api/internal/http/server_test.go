package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewServerRegistersUniqueRoutes(t *testing.T) {
	server := NewServer(nil, "database unavailable", "admin-password", "", "")
	if server == nil {
		t.Fatal("expected a server handler")
	}
}

func TestAdsTXTPublicPlainTextRoute(t *testing.T) {
	server := NewServer(nil, "database unavailable", "admin-password", "", "")
	req := httptest.NewRequest(http.MethodGet, "/ads.txt", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain content type, got %q", got)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, max-age=0" {
		t.Fatalf("expected no-cache Cache-Control, got %q", got)
	}
	if got := res.Header.Get("Pragma"); got != "no-cache" {
		t.Fatalf("expected no-cache Pragma, got %q", got)
	}
	if got := res.Header.Get("X-Robots-Tag"); got != "all" {
		t.Fatalf("expected X-Robots-Tag all, got %q", got)
	}
	if got := string(body); got != adsTXTBody {
		t.Fatalf("expected ads.txt body %q, got %q", adsTXTBody, got)
	}
}

func TestRobotsTXTPublicPlainTextRoute(t *testing.T) {
	server := NewServer(nil, "database unavailable", "admin-password", "", "")
	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain content type, got %q", got)
	}
	if got := string(body); got != robotsTXTBody {
		t.Fatalf("expected robots.txt body %q, got %q", robotsTXTBody, got)
	}
}
