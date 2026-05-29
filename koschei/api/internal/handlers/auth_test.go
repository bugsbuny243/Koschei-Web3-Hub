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

func TestStartOTPLoginSendsBetterAuthRequest(t *testing.T) {
	t.Setenv("NEON_AUTH_BASE_URL", "https://auth.example.test/api/auth")
	t.Setenv("NEON_AUTH_JWKS_URL", "https://auth.example.test/api/auth/jwks")
	oldClient := authProviderHTTPClient
	defer func() { authProviderHTTPClient = oldClient }()

	authProviderHTTPClient = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://auth.example.test/api/auth/email-otp/send-verification-otp" {
			t.Fatalf("unexpected Better Auth URL: %s", r.URL.String())
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("X-Stack-Project-Id"); got != "" {
			t.Fatalf("unexpected Stack Auth project header: %q", got)
		}
		if got := r.Header.Get("X-Stack-Publishable-Client-Key"); got != "" {
			t.Fatalf("unexpected Stack Auth publishable key header: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"email":"user@example.com"`) {
			t.Fatalf("request body did not include normalized email: %s", string(body))
		}
		if !strings.Contains(string(body), `"type":"sign-in"`) {
			t.Fatalf("request body did not request sign-in OTP: %s", string(body))
		}
		if strings.Contains(string(body), "callback_url") {
			t.Fatalf("request body should not include Stack Auth callback URL: %s", string(body))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/start", strings.NewReader(`{"email":"User@Example.COM","callback_url":"/login.html"}`))
	req.Host = "app.example.test"
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()

	(&Handler{}).StartOTPLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"flow":"email_otp"`) {
		t.Fatalf("response did not include email OTP flow: %s", w.Body.String())
	}
}

func TestStartOTPLoginRequiresBetterAuthEnv(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/start", strings.NewReader(`{"email":"user@example.com","callback_url":"/login.html"}`))
	req.Host = "app.example.test"
	w := httptest.NewRecorder()

	(&Handler{}).StartOTPLogin(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "NEON_AUTH_BASE_URL") || !strings.Contains(w.Body.String(), "NEON_AUTH_JWKS_URL") {
		t.Fatalf("response did not explain missing Better Auth env: %s", w.Body.String())
	}
}

func TestStartOTPLoginRejectsCrossOriginCallback(t *testing.T) {
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
