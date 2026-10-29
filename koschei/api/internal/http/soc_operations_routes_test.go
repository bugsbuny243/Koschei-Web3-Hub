package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSOCOperationsRoutesRequireOwnerAndDoNotFakeAvailability(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "soc-test-secret")
	t.Setenv("OWNER_WALLET", "")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "owner-soc.html"), []byte("PRIVATE SOC WORKSPACE"), 0600); err != nil {
		t.Fatal(err)
	}
	server := NewServer(nil, "", "", "", dir)
	for _, path := range []string{"/api/owner/soc/queue", "/api/owner/soc/case", "/owner/soc", "/owner-soc.html"} {
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 404 || strings.Contains(rec.Body.String(), "PRIVATE SOC") {
			t.Fatalf("unauthorized %s: %d %s", path, rec.Code, rec.Body.String())
		}
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("x-koschei-secret", "soc-test-secret")
		rec = httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		want := 200
		if strings.HasPrefix(path, "/api/") {
			want = 503
		}
		if rec.Code != want {
			t.Fatalf("authorized %s: %d want %d %s", path, rec.Code, want, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("cacheable private surface %s", path)
		}
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/owner/soc/case", nil)
	req.Header.Set("x-koschei-secret", "soc-test-secret")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != 405 {
		t.Fatalf("unexpected delete status %d", rec.Code)
	}
}
