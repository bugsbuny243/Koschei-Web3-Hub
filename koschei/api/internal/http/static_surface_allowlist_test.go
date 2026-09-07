package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticSurfaceAllowlistPreservesKnownPagesAndAssets(t *testing.T) {
	staticDir := t.TempDir()
	writeStaticFixture(t, staticDir, "index.html", "home")
	writeStaticFixture(t, staticDir, "dashboard.html", "dashboard")
	writeStaticFixture(t, staticDir, "login.html", "login")
	writeStaticFixture(t, staticDir, "agents.html", "agents")
	writeStaticFixture(t, staticDir, "css/koschei-home.css", "body{}")
	writeStaticFixture(t, staticDir, "js/koschei-dashboard.js", "console.log('ok')")
	writeStaticFixture(t, staticDir, "sdk/koschei-shield.js", "export{}")
	writeStaticFixture(t, staticDir, "full-scan-contract-v1.json", `{"ok":true}`)
	writeStaticFixture(t, staticDir, "app/login", "app login redirect")

	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	t.Cleanup(srv.Close)

	for route, want := range map[string]string{
		"/":                           "home",
		"/dashboard":                  "dashboard",
		"/dashboard.html":             "dashboard",
		"/login":                      "login",
		"/login.html":                 "login",
		"/agents":                     "agents",
		"/css/koschei-home.css":       "body{}",
		"/js/koschei-dashboard.js":    "console.log('ok')",
		"/sdk/koschei-shield.js":      "export{}",
		"/full-scan-contract-v1.json": `{"ok":true}`,
		"/app/login":                  "app login redirect",
	} {
		assertStaticResponse(t, srv.URL+route, http.StatusOK, want)
	}
}

func TestStaticSurfaceAllowlistRejectsAccidentalFilesAndUnknownRoutes(t *testing.T) {
	staticDir := t.TempDir()
	writeStaticFixture(t, staticDir, "index.html", "home")
	writeStaticFixture(t, staticDir, "dashboard.html", "dashboard")
	writeStaticFixture(t, staticDir, "graveyard.html", "must never be public")
	writeStaticFixture(t, staticDir, "vercel.json", `{"rewrites":[]}`)
	writeStaticFixture(t, staticDir, "secret.txt", "not public")

	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	t.Cleanup(srv.Close)

	for _, route := range []string{
		"/graveyard",
		"/graveyard.html",
		"/vercel.json",
		"/secret.txt",
		"/does-not-exist",
	} {
		assertStaticResponse(t, srv.URL+route, http.StatusNotFound, "")
	}
}

func TestStaticSurfaceAllowlistKeepsLegacyScannerRoutesRouterOwned(t *testing.T) {
	staticDir := t.TempDir()
	writeStaticFixture(t, staticDir, "index.html", "home")
	writeStaticFixture(t, staticDir, "dashboard.html", "dashboard")
	// If these stale files accidentally return, the router must still own the
	// legacy URLs and redirect to the canonical Customer Panel anchors.
	writeStaticFixture(t, staticDir, "scan.html", "stale scan")
	writeStaticFixture(t, staticDir, "safe-check.html", "stale safe check")

	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	t.Cleanup(srv.Close)
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}

	for _, test := range []struct {
		path     string
		location string
	}{
		{path: "/scan", location: "/dashboard#transaction-preflight"},
		{path: "/scan.html", location: "/dashboard#transaction-preflight"},
		{path: "/transaction-shield", location: "/dashboard#transaction-preflight"},
		{path: "/safe-check", location: "/dashboard#capabilities"},
		{path: "/security-radar", location: "/dashboard#capabilities"},
	} {
		req, err := http.NewRequest(http.MethodGet, srv.URL+test.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", test.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusPermanentRedirect {
			t.Fatalf("GET %s = %d, want %d", test.path, resp.StatusCode, http.StatusPermanentRedirect)
		}
		if got := resp.Header.Get("Location"); got != test.location {
			t.Fatalf("GET %s Location = %q, want %q", test.path, got, test.location)
		}
	}
}

func TestStaticSurfaceAllowlistRejectsWrites(t *testing.T) {
	staticDir := t.TempDir()
	writeStaticFixture(t, staticDir, "index.html", "home")
	writeStaticFixture(t, staticDir, "dashboard.html", "dashboard")

	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	t.Cleanup(srv.Close)
	resp, err := http.Post(srv.URL+"/dashboard", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /dashboard = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func writeStaticFixture(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func assertStaticResponse(t *testing.T, url string, wantStatus int, wantBody string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s = %d, want %d", url, resp.StatusCode, wantStatus)
	}
	if wantBody == "" {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	if string(body) != wantBody {
		t.Fatalf("GET %s body = %q, want %q", url, string(body), wantBody)
	}
}
