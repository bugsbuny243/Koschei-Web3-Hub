package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticRootServesWeb3HubIndex(t *testing.T) {
	staticDir := filepath.Join("..", "..", "public")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	NewServer(nil, "DATABASE_URL is not set", "", "", staticDir).ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()
	bodyBytes, _ := io.ReadAll(res.Body)
	body := string(bodyBytes)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", res.StatusCode, http.StatusOK, body)
	}
	for _, want := range []string{"Koschei Web3 Hub", "Solana Security Tools", "Token Scanner", "TX Decoder", "google-adsense-account", "ca-pub-6081394144742471", "G-1QFWMJJC3"} {
		if !strings.Contains(body, want) {
			t.Fatalf("root index missing %q", want)
		}
	}
	oldPortalMarkers := []string{
		"Koschei " + "Games",
		"HTML5/" + "WebGL",
		"Tarayıcıda anında " + "oynanan",
		"Popüler " + "Oyunlar",
		"Oyunları " + "Keşfet",
		"Neon Drift " + "Arena",
		"Skyline " + "Defender",
		"Quantum Vault " + "Run",
	}
	for _, forbidden := range oldPortalMarkers {
		if strings.Contains(body, forbidden) {
			t.Fatalf("root index still contains old games content %q", forbidden)
		}
	}
}

func TestAdsTXTExactBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ads.txt", nil)
	w := httptest.NewRecorder()

	NewServer(nil, "DATABASE_URL is not set", "", "", "").ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()
	bodyBytes, _ := io.ReadAll(res.Body)
	body := string(bodyBytes)
	if body != "google.com, pub-6081394144742471, DIRECT, f08c47fec0942fa0" {
		t.Fatalf("ads.txt body = %q", body)
	}
}

func TestSecurityHeadersAllowAdsenseAndAnalytics(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	NewServer(nil, "DATABASE_URL is not set", "", "", "").ServeHTTP(w, req)

	csp := w.Result().Header.Get("Content-Security-Policy")
	for _, want := range []string{
		"script-src 'self' 'unsafe-inline' https://pagead2.googlesyndication.com https://www.googletagmanager.com",
		"connect-src 'self' https://www.google-analytics.com https://region1.google-analytics.com",
		"img-src 'self' data: https:",
		"frame-src https://googleads.g.doubleclick.net https://tpc.googlesyndication.com",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	} {
		if !strings.Contains(csp, want) {
			t.Fatalf("CSP missing %q in %q", want, csp)
		}
	}
}
