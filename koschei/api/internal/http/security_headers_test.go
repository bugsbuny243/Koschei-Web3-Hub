package http

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestSecurityHeadersTransformsInlineHTMLWithoutUnsafeInline(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><style>body{color:white}</style></head><body><button style="color:red" onclick="doThing()">run</button><script>window.ready=true</script><script src="https://www.googletagmanager.com/gtag/js"></script></body></html>`))
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "https://tradepigloball.co/", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	policy := recorder.Header().Get("Content-Security-Policy")
	if policy == "" {
		t.Fatal("Content-Security-Policy header is missing")
	}
	if strings.Contains(policy, "'unsafe-inline'") {
		t.Fatalf("policy retained unsafe-inline: %s", policy)
	}
	if regexp.MustCompile(`(?:^|[ ;])https:(?:[ ;]|$)`).MatchString(policy) {
		t.Fatalf("policy retained a scheme-wide https source: %s", policy)
	}
	if regexp.MustCompile(`(?:^|[ ;])wss:(?:[ ;]|$)`).MatchString(policy) {
		t.Fatalf("policy retained a scheme-wide wss source: %s", policy)
	}
	nonceMatch := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(policy)
	if len(nonceMatch) != 2 || nonceMatch[1] == "" {
		t.Fatalf("policy does not contain a nonce: %s", policy)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `<style nonce="`+nonceMatch[1]+`">`) {
		t.Fatalf("inline style did not receive the response nonce: %s", body)
	}
	if !strings.Contains(body, `<script nonce="`+nonceMatch[1]+`">window.ready=true</script>`) {
		t.Fatalf("inline script did not receive the response nonce: %s", body)
	}
	if strings.Contains(body, `src="https://www.googletagmanager.com/gtag/js" nonce=`) {
		t.Fatalf("allowlisted external script unexpectedly received a nonce: %s", body)
	}
	for _, value := range []string{"doThing()", "color:red"} {
		digest := sha256.Sum256([]byte(value))
		hash := "sha256-" + base64.StdEncoding.EncodeToString(digest[:])
		if !strings.Contains(policy, "'"+hash+"'") {
			t.Fatalf("inline attribute hash %s is missing from policy: %s", hash, policy)
		}
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("HTML response cache policy = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}
}

func TestSecurityHeadersLeavesJSONBodyUnchanged(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "https://tradepigloball.co/api/version", nil))
	if recorder.Body.String() != `{"ok":true}` {
		t.Fatalf("JSON body was changed: %q", recorder.Body.String())
	}
	policy := recorder.Header().Get("Content-Security-Policy")
	if strings.Contains(policy, "'nonce-") || strings.Contains(policy, "'unsafe-inline'") {
		t.Fatalf("non-HTML policy is not strict and static: %s", policy)
	}
}

func TestSecurityHeadersFailClosedOnJavaScriptURL(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<a href="javascript:alert(1)">bad</a>`))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "https://tradepigloball.co/", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want fail-closed %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Header().Get("Content-Security-Policy"), "'unsafe-inline'") {
		t.Fatal("fail-closed response restored unsafe-inline")
	}
}

func TestBuildAllowedOriginsRejectsPublicHTTP(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	origins := buildAllowedOrigins(strings.Join([]string{
		"http://tradepigloball.co",
		"http://localhost:3000",
		"https://api.example.com/",
		"https://api.example.com/path",
		"ftp://api.example.com",
	}, ","))
	for _, origin := range []string{"https://tradepigloball.co", "https://www.tradepigloball.co", "https://api.example.com"} {
		if _, ok := origins[origin]; !ok {
			t.Fatalf("expected HTTPS origin is missing: %s", origin)
		}
	}
	for _, origin := range []string{"http://tradepigloball.co", "http://localhost:3000", "https://api.example.com/path"} {
		if _, ok := origins[origin]; ok {
			t.Fatalf("unsafe or non-origin value was accepted: %s", origin)
		}
	}
}

func TestBuildAllowedOriginsAllowsOnlyLoopbackHTTPInDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	origins := buildAllowedOrigins("http://localhost:3000,http://127.0.0.1:5173,http://example.com,https://example.com")
	for _, origin := range []string{"http://localhost:3000", "http://127.0.0.1:5173", "https://example.com"} {
		if _, ok := origins[origin]; !ok {
			t.Fatalf("expected development origin is missing: %s", origin)
		}
	}
	if _, ok := origins["http://example.com"]; ok {
		t.Fatal("public HTTP origin was accepted in development")
	}
}

func TestAllowedCORSOriginRequiresCanonicalExactOrigin(t *testing.T) {
	allowed := map[string]struct{}{"https://example.com": {}}
	if got := allowedCORSOrigin("https://EXAMPLE.com/", allowed); got != "https://example.com" {
		t.Fatalf("canonical origin = %q, want https://example.com", got)
	}
	for _, input := range []string{"https://example.com/path", "https://example.com?x=1", "https://user@example.com", "http://example.com"} {
		if got := allowedCORSOrigin(input, allowed); got != "" {
			t.Fatalf("non-canonical origin %q was accepted as %q", input, got)
		}
	}
}
