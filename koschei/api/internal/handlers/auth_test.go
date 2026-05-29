package handlers

import (
	"context"
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

func TestPostBetterAuthEmailPasswordUsesSignInEmailPayload(t *testing.T) {
	oldClient := authProviderHTTPClient
	defer func() { authProviderHTTPClient = oldClient }()

	authProviderHTTPClient = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://auth.example.test/api/auth/sign-in/email" {
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
		if !strings.Contains(string(body), `"password":"correct horse battery staple"`) {
			t.Fatalf("request body did not include provider password field: %s", string(body))
		}
		if strings.Contains(string(body), "otp") || strings.Contains(string(body), "callback_url") {
			t.Fatalf("request body should not include OTP or callback fields: %s", string(body))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"session-token"}`)), Header: make(http.Header)}, nil
	})

	_, _, err := postBetterAuthWithCookies(
		context.Background(),
		betterAuthConfig{BaseURL: "https://auth.example.test/api/auth", IssuerURL: "https://auth.example.test", JWKSURL: "https://auth.example.test/api/auth/jwks"},
		"/sign-in/email",
		map[string]string{"email": "user@example.com", "password": "correct horse battery staple"},
	)
	if err != nil {
		t.Fatalf("postBetterAuthWithCookies returned error: %v", err)
	}
}

func TestLoginRequiresEmailPasswordAuthEnv(t *testing.T) {
	t.Setenv("NEON_AUTH_BASE_URL", "")
	t.Setenv("NEON_AUTH_ISSUER", "")
	t.Setenv("NEON_AUTH_JWKS_URL", "")
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"user@example.com","password":"secret-password"}`))
	w := httptest.NewRecorder()

	(&Handler{}).Login(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	for _, name := range []string{"NEON_AUTH_BASE_URL", "NEON_AUTH_ISSUER", "NEON_AUTH_JWKS_URL"} {
		if !strings.Contains(w.Body.String(), name) {
			t.Fatalf("response did not explain missing %s env: %s", name, w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), "secret-password") {
		t.Fatalf("response must not echo password: %s", w.Body.String())
	}
}

func TestLoginRequiresPassword(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"user@example.com"}`))
	w := httptest.NewRecorder()

	(&Handler{}).Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "missing_password") {
		t.Fatalf("response did not explain missing password: %s", w.Body.String())
	}
}

func TestOTPLoginEndpointsAreDisabled(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "start", handler: (&Handler{}).StartOTPLogin},
		{name: "verify", handler: (&Handler{}).VerifyOTPLogin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/"+tc.name, strings.NewReader(`{}`))
			w := httptest.NewRecorder()

			tc.handler(w, req)

			if w.Code != http.StatusGone {
				t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusGone, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "email and password") {
				t.Fatalf("response did not explain email/password auth mode: %s", w.Body.String())
			}
		})
	}
}

func TestPublicAuthProviderErrorMessages(t *testing.T) {
	if got := publicAuthProviderError(authProviderHTTPError{StatusCode: http.StatusNotFound}); got != "Configured Neon Auth endpoint not found. Check NEON_AUTH_BASE_URL and auth method." {
		t.Fatalf("404 message = %q", got)
	}
	if got := publicAuthProviderError(authProviderHTTPError{StatusCode: http.StatusUnauthorized}); got != "Invalid email or password." {
		t.Fatalf("401 message = %q", got)
	}
	if got := publicAuthProviderError(authProviderHTTPError{StatusCode: http.StatusBadRequest, Body: `{"message":"Invalid credentials"}`}); got != "Invalid email or password." {
		t.Fatalf("invalid credentials message = %q", got)
	}
}
