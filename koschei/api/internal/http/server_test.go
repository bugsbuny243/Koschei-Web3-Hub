package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerStaticRouteServesLoginUI(t *testing.T) {
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

	ownerPage, err := http.Get(srv.URL + "/owner")
	if err != nil {
		t.Fatalf("get owner page: %v", err)
	}
	defer ownerPage.Body.Close()
	if ownerPage.StatusCode != http.StatusOK {
		t.Fatalf("GET /owner = %d, want %d", ownerPage.StatusCode, http.StatusOK)
	}

	apiResp, err := http.Get(srv.URL + "/api/owner/status")
	if err != nil {
		t.Fatalf("get owner api without secret: %v", err)
	}
	defer apiResp.Body.Close()
	if apiResp.StatusCode == http.StatusOK {
		t.Fatalf("GET /api/owner/status without secret = %d, want a protected non-OK response", apiResp.StatusCode)
	}
}

func TestCleanRoutesExposeAllPublicModules(t *testing.T) {
	staticDir := t.TempDir()
	files := map[string]string{
		"index.html":             "index",
		"account.html":           "account",
		"agent-api.html":         "agent-api",
		"airdrop-checker.html":   "airdrop",
		"chains.html":            "chains",
		"cross-chain-risk.html":  "cross-chain-risk",
		"dashboard.html":         "dashboard",
		"docs.html":              "docs",
		"docs-api.html":          "docs-api",
		"docs-sdk.html":          "docs-sdk",
		"funding-assistant.html": "funding-assistant",
		"graph.html":             "graph",
		"hub.html":               "hub",
		"impact.html":            "impact",
		"launches.html":          "launches",
		"login.html":             "login",
		"metadata.html":          "metadata",
		"mev-shield.html":        "mev-shield",
		"owner.html":             "owner",
		"pay-per-tool.html":      "pay-per-tool",
		"portfolio.html":         "portfolio",
		"pricing.html":           "pricing",
		"program-scanner.html":   "program",
		"project-radar.html":     "project-radar",
		"radar.html":             "radar",
		"register.html":          "register",
		"reports.html":           "reports",
		"risk-v2.html":           "risk-v2",
		"smart-money.html":       "smart-money",
		"support.html":           "support",
		"sybil-check.html":       "sybil-check",
		"token-scanner.html":     "token-scanner",
		"tx-decoder-pro.html":    "tx-decoder-pro",
		"wallet-score.html":      "wallet-score",
		"watchlist.html":         "watchlist",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(staticDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	t.Cleanup(srv.Close)

	cases := map[string]string{
		"/account":           "account",
		"/agent-api":         "agent-api",
		"/airdrop-checker":   "airdrop",
		"/chains":            "chains",
		"/cross-chain-risk":  "cross-chain-risk",
		"/dashboard":         "dashboard",
		"/docs":              "docs",
		"/docs/api":          "docs-api",
		"/docs/sdk":          "docs-sdk",
		"/funding-assistant": "funding-assistant",
		"/graph":             "graph",
		"/hub":               "hub",
		"/impact":            "impact",
		"/launches":          "launches",
		"/login":             "login",
		"/metadata":          "metadata",
		"/mev-shield":        "mev-shield",
		"/pay-per-tool":      "pay-per-tool",
		"/portfolio":         "portfolio",
		"/pricing":           "pricing",
		"/program-scanner":   "program",
		"/project-radar":     "project-radar",
		"/radar":             "radar",
		"/register":          "register",
		"/reports":           "reports",
		"/risk":              "risk-v2",
		"/risk-v2":           "risk-v2",
		"/smart-money":       "smart-money",
		"/support":           "support",
		"/sybil-check":       "sybil-check",
		"/token-scanner":     "token-scanner",
		"/tx-decoder":        "tx-decoder-pro",
		"/tx-decoder-pro":    "tx-decoder-pro",
		"/wallet-score":      "wallet-score",
		"/watchlist":         "watchlist",
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
