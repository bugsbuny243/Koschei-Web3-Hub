package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerStaticRouteRequiresSecretOnly(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "owner.html"), []byte("owner panel"), 0o644); err != nil {
		t.Fatalf("write owner: %v", err)
	}
	t.Setenv("OWNER_SECRET", "test-secret")
	t.Setenv("OWNER_WALLET", "")

	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	t.Cleanup(srv.Close)

	unauthorized, err := http.Get(srv.URL + "/owner")
	if err != nil {
		t.Fatalf("get owner without secret: %v", err)
	}
	defer unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /owner without secret = %d, want %d", unauthorized.StatusCode, http.StatusNotFound)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/owner", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-Koschei-Secret", "test-secret")
	authorized, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get owner with secret: %v", err)
	}
	defer authorized.Body.Close()
	if authorized.StatusCode != http.StatusOK {
		t.Fatalf("GET /owner with secret = %d, want %d", authorized.StatusCode, http.StatusOK)
	}
}

func TestImpactRouteServesStaticPageAndMetricsFallback(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "impact.html"), []byte("impact page"), 0o644); err != nil {
		t.Fatalf("write impact: %v", err)
	}

	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	t.Cleanup(srv.Close)

	page, err := http.Get(srv.URL + "/impact")
	if err != nil {
		t.Fatalf("get impact page: %v", err)
	}
	defer page.Body.Close()
	if page.StatusCode != http.StatusOK {
		t.Fatalf("GET /impact = %d, want %d", page.StatusCode, http.StatusOK)
	}

	metrics, err := http.Get(srv.URL + "/api/public/metrics")
	if err != nil {
		t.Fatalf("get impact metrics: %v", err)
	}
	defer metrics.Body.Close()
	if metrics.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/public/metrics = %d, want %d", metrics.StatusCode, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(metrics.Body).Decode(&body); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if body["demo_mode"] != true {
		t.Fatalf("demo_mode = %v, want true", body["demo_mode"])
	}
}

func TestCORSAllowsTradepigloballOrigin(t *testing.T) {
	srv := httptest.NewServer(NewServer(nil, "", "", "", ""))
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/api/auth/register", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "https://tradepigloball.co")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("options request: %v", err)
	}
	defer res.Body.Close()
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://tradepigloball.co" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want tradepigloball origin", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want true", got)
	}
}
