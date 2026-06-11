package http

import (
	"io"
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

func TestCleanRoutesExposeAllPublicModules(t *testing.T) {
	staticDir := t.TempDir()
	files := map[string]string{
		"index.html":           "index",
		"airdrop-checker.html": "airdrop",
		"launches.html":        "launches",
		"portfolio.html":       "portfolio",
		"program-scanner.html": "program",
		"risk-v2.html":         "risk-v2",
		"smart-money.html":     "smart-money",
		"token-scanner.html":   "token-scanner",
		"tx-decoder-pro.html":  "tx-decoder-pro",
		"wallet-score.html":    "wallet-score",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(staticDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	t.Cleanup(srv.Close)

	cases := map[string]string{
		"/airdrop-checker": "airdrop",
		"/launches":        "launches",
		"/portfolio":       "portfolio",
		"/program-scanner": "program",
		"/risk":            "risk-v2",
		"/smart-money":     "smart-money",
		"/token-scanner":   "token-scanner",
		"/tx-decoder":      "tx-decoder-pro",
		"/wallet-score":    "wallet-score",
	}
	for route, want := range cases {
		route, want := route, want
		t.Run(route, func(t *testing.T) {
			resp, err := http.Get(srv.URL + route)
			if err != nil {
				t.Fatalf("get %s: %v", route, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s = %d, want %d", route, resp.StatusCode, http.StatusOK)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read %s: %v", route, err)
			}
			if string(body) != want {
				t.Fatalf("GET %s body = %q, want %q", route, string(body), want)
			}
		})
	}
}
