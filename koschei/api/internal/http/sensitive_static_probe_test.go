package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSensitiveStaticProbePathRejectsSecretAndRepositoryPaths(t *testing.T) {
	for _, probe := range []string{
		"/.env",
		"/.env.local",
		"/.env-production",
		"/.git/config",
		"/nested/.svn/entries",
		"/.aws/credentials",
		"/.ssh/id_rsa",
		"/.npmrc",
		"/id_ed25519",
		"/service-account.json",
		"//.env",
		"/vercel.json",
		"/railway.json",
		"/RAILWAY.TOML",
		"/Dockerfile",
		"/docker-compose.yml",
		"/docker-compose.yaml",
		"/go.mod",
		"/go.sum",
		"/keys/service-account.pem",
		"/keys/archive.key",
		"/keys/client.p12",
		"/keys/client.pfx",
	} {
		if !sensitiveStaticProbePath(probe) {
			t.Errorf("probe %q was not rejected", probe)
		}
	}
}

func TestSensitiveStaticProbePathPreservesWellKnownNamespace(t *testing.T) {
	for _, path := range []string{
		"/.well-known/security.txt",
		"/.well-known/acme-challenge/token",
		"/assets/app.js",
		"/dashboard",
		"/scan",
		"/reports",
		"/watchlist",
		"/arvis-chat",
		"/validation-key.txt",
		"/full-scan-contract-v1.json",
		"/security-ecosystem.json",
		"/sitemap.xml",
	} {
		if sensitiveStaticProbePath(path) {
			t.Errorf("legitimate public path %q was rejected", path)
		}
	}
}

func TestSecurityHeadersReturns404BeforeSensitiveProbeCanReachStaticFallback(t *testing.T) {
	called := false
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("homepage fallback"))
	}))

	for _, target := range []string{
		"https://tradepigloball.co/.env",
		"https://tradepigloball.co/.git/config",
		"https://tradepigloball.co/.env%2elocal",
		"https://tradepigloball.co/vercel.json",
		"https://tradepigloball.co/railway.toml",
		"https://tradepigloball.co/keys/service.pem",
	} {
		called = false
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if called {
			t.Fatalf("sensitive target %s reached downstream handler", target)
		}
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("target %s status=%d want=%d", target, recorder.Code, http.StatusNotFound)
		}
		if recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("target %s cache-control=%q want no-store", target, recorder.Header().Get("Cache-Control"))
		}
	}
}

func TestSecurityHeadersAllowsWellKnownPathToReachRouter(t *testing.T) {
	called := false
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "https://tradepigloball.co/.well-known/security.txt", nil))
	if !called {
		t.Fatal("/.well-known request was blocked before the router")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d want=%d", recorder.Code, http.StatusNoContent)
	}
}
