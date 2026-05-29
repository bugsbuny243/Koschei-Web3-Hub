package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestStartOTPLoginSendsStackAuthRequest(t *testing.T) {
	t.Setenv("NEON_AUTH_PROJECT_ID", "project-id")
	t.Setenv("NEON_AUTH_PUBLISHABLE_CLIENT_KEY", "pck_test")
	t.Setenv("NEON_AUTH_STACK_API_BASE_URL", "https://stack.example.test/api/v1")
	oldClient := stackAuthHTTPClient
	defer func() { stackAuthHTTPClient = oldClient }()

	stackAuthHTTPClient = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://stack.example.test/api/v1/auth/otp/send-sign-in-code" {
			t.Fatalf("unexpected Stack Auth URL: %s", r.URL.String())
		}
		if got := r.Header.Get("X-Stack-Access-Type"); got != "client" {
			t.Fatalf("X-Stack-Access-Type = %q, want client", got)
		}
		if got := r.Header.Get("X-Stack-Project-Id"); got != "project-id" {
			t.Fatalf("X-Stack-Project-Id = %q, want project-id", got)
		}
		if got := r.Header.Get("X-Stack-Publishable-Client-Key"); got != "pck_test" {
			t.Fatalf("X-Stack-Publishable-Client-Key = %q, want pck_test", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"email":"user@example.com"`) {
			t.Fatalf("request body did not include normalized email: %s", string(body))
		}
		if !strings.Contains(string(body), `"callback_url":"https://app.example.test/login.html"`) {
			t.Fatalf("request body did not include callback URL: %s", string(body))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"nonce":"nonce-123"}`)), Header: make(http.Header)}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/start", strings.NewReader(`{"email":"User@Example.COM","callback_url":"/login.html"}`))
	req.Host = "app.example.test"
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()

	(&Handler{}).StartOTPLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"nonce":"nonce-123"`) {
		t.Fatalf("response did not include nonce: %s", w.Body.String())
	}
}

func TestStartOTPLoginRejectsCrossOriginCallback(t *testing.T) {
	t.Setenv("NEON_AUTH_PROJECT_ID", "project-id")
	t.Setenv("NEON_AUTH_PUBLISHABLE_CLIENT_KEY", "pck_test")
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/start", strings.NewReader(`{"email":"user@example.com","callback_url":"https://evil.example/login.html"}`))
	req.Host = "app.example.test"
	w := httptest.NewRecorder()

	(&Handler{}).StartOTPLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestVerifyOTPLoginRequiresFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(`{"email":"user@example.com"}`))
	w := httptest.NewRecorder()

	(&Handler{}).VerifyOTPLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}
