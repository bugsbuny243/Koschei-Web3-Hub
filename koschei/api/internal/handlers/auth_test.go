package handlers

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
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

func TestPostBetterAuthEmailPasswordFallbackTriesAPIRouteAfter404(t *testing.T) {
	oldClient := authProviderHTTPClient
	defer func() { authProviderHTTPClient = oldClient }()

	var gotURLs []string
	authProviderHTTPClient = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURLs = append(gotURLs, r.URL.String())
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "secret-password") && r.URL.String() == "" {
			t.Fatalf("unreachable password guard")
		}
		switch r.URL.String() {
		case "https://auth.example.test/sign-in/email":
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{"message":"not found"}`)), Header: make(http.Header)}, nil
		case "https://auth.example.test/api/auth/sign-in/email":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"session-token"}`)), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected fallback URL: %s", r.URL.String())
			return nil, nil
		}
	})

	resp, _, baseURL, err := postBetterAuthEmailPasswordWithFallback(
		context.Background(),
		betterAuthConfig{BaseURL: "https://auth.example.test", IssuerURL: "https://auth.example.test", JWKSURL: "https://auth.example.test/api/auth/jwks"},
		map[string]string{"email": "user@example.com", "password": "secret-password"},
	)
	if err != nil {
		t.Fatalf("postBetterAuthEmailPasswordWithFallback returned error: %v", err)
	}
	if baseURL != "https://auth.example.test/api/auth" {
		t.Fatalf("baseURL = %q, want API auth base", baseURL)
	}
	if resp["token"] != "session-token" {
		t.Fatalf("unexpected response payload: %#v", resp)
	}
	want := []string{"https://auth.example.test/sign-in/email", "https://auth.example.test/api/auth/sign-in/email"}
	if strings.Join(gotURLs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("attempted URLs = %#v, want %#v", gotURLs, want)
	}
}

func TestPostBetterAuthEmailPasswordFallbackStopsOnFirstNon404(t *testing.T) {
	oldClient := authProviderHTTPClient
	defer func() { authProviderHTTPClient = oldClient }()

	attempts := 0
	authProviderHTTPClient = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if r.URL.String() != "https://auth.example.test/sign-in/email" {
			t.Fatalf("unexpected URL after non-404 response: %s", r.URL.String())
		}
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"message":"Invalid credentials"}`)), Header: make(http.Header)}, nil
	})

	_, _, _, err := postBetterAuthEmailPasswordWithFallback(
		context.Background(),
		betterAuthConfig{BaseURL: "https://auth.example.test", IssuerURL: "https://auth.example.test", JWKSURL: "https://auth.example.test/api/auth/jwks"},
		map[string]string{"email": "user@example.com", "password": "secret-password"},
	)
	if err == nil {
		t.Fatal("expected auth provider error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	var httpErr authProviderHTTPError
	if !strings.Contains(publicAuthProviderError(err), "Invalid email or password") {
		t.Fatalf("public error should preserve invalid-credentials handling: %q", publicAuthProviderError(err))
	}
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %#v, want 401 authProviderHTTPError", err)
	}
}

func TestPostBetterAuthEmailPasswordFallbackAll404ReturnsClearError(t *testing.T) {
	oldClient := authProviderHTTPClient
	defer func() { authProviderHTTPClient = oldClient }()

	var gotURLs []string
	authProviderHTTPClient = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURLs = append(gotURLs, r.URL.String())
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{"message":"missing"}`)), Header: make(http.Header)}, nil
	})

	_, _, _, err := postBetterAuthEmailPasswordWithFallback(
		context.Background(),
		betterAuthConfig{BaseURL: "https://auth.example.test/auth", IssuerURL: "https://auth.example.test", JWKSURL: "https://auth.example.test/api/auth/jwks"},
		map[string]string{"email": "user@example.com", "password": "secret-password"},
	)
	if err == nil {
		t.Fatal("expected not found error")
	}
	wantMessage := "Neon Auth email/password endpoint not found. Check whether Auth URL includes /api/auth."
	if got := publicAuthProviderError(err); got != wantMessage {
		t.Fatalf("public error = %q, want %q", got, wantMessage)
	}
	if strings.Contains(publicAuthProviderError(err), "auth.example.test") || strings.Contains(publicAuthProviderError(err), "secret-password") {
		t.Fatalf("public error leaked URL or password: %q", publicAuthProviderError(err))
	}
	want := []string{
		"https://auth.example.test/auth/sign-in/email",
		"https://auth.example.test/auth/api/auth/sign-in/email",
		"https://auth.example.test/api/auth/sign-in/email",
	}
	if strings.Join(gotURLs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("attempted URLs = %#v, want %#v", gotURLs, want)
	}
}

func TestBetterAuthConfigRequiresNeonAuthEnv(t *testing.T) {
	t.Setenv("NEON_AUTH_BASE_URL", "")
	t.Setenv("NEON_AUTH_ISSUER", "")
	t.Setenv("NEON_AUTH_JWKS_URL", "")

	_, err := betterAuthConfigFromEnv()
	if err == nil {
		t.Fatal("betterAuthConfigFromEnv returned nil error, want missing env error")
	}
	msg := err.Error()
	for _, name := range []string{"NEON_AUTH_BASE_URL", "NEON_AUTH_ISSUER", "NEON_AUTH_JWKS_URL"} {
		if !strings.Contains(msg, name) {
			t.Fatalf("missing env error %q does not include %s", msg, name)
		}
	}
}

func TestLoginAndRegisterReturnJSONWhenNeonEnvMissing(t *testing.T) {
	t.Setenv("NEON_AUTH_BASE_URL", "")
	t.Setenv("NEON_AUTH_ISSUER", "")
	t.Setenv("NEON_AUTH_JWKS_URL", "")

	for _, tc := range []struct {
		name    string
		path    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "login", path: "/api/auth/login", handler: (&Handler{}).Login},
		{name: "register", path: "/api/auth/register", handler: (&Handler{}).Register},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"email":"user@example.com","password":"secret-password"}`))
			w := httptest.NewRecorder()

			tc.handler(w, req)

			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusServiceUnavailable, w.Body.String())
			}
			if w.Code == http.StatusGone {
				t.Fatalf("%s returned disabled 410 response", tc.name)
			}
			if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
				t.Fatalf("Content-Type = %q, want JSON", got)
			}
			body := w.Body.String()
			if !strings.Contains(body, "auth_not_configured") || !strings.Contains(body, "NEON_AUTH_BASE_URL") {
				t.Fatalf("response did not clearly explain missing auth env: %s", body)
			}
		})
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

func TestConfigExposesPublicNeonAuthURL(t *testing.T) {
	t.Setenv("EXPO_PUBLIC_NEON_AUTH_URL", "https://auth.example.test")
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	(&Handler{}).Config(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"neonAuthUrl":"https://auth.example.test"`) {
		t.Fatalf("/api/config did not expose public Neon Auth URL: %s", w.Body.String())
	}
}

func TestFrontendAuthUsesBackendEndpoints(t *testing.T) {
	body, err := os.ReadFile("../../public/js/koschei-auth.js")
	if err != nil {
		t.Fatalf("read frontend auth script: %v", err)
	}
	script := string(body)
	for _, want := range []string{"/api/auth/register", "/api/auth/login", "koschei_jwt"} {
		if !strings.Contains(script, want) {
			t.Fatalf("frontend auth script does not contain %s", want)
		}
	}
	for _, forbidden := range []string{"_neonRequest", "sign-up/email", "sign-in/email", "neonAuthUrl", "EXPO_PUBLIC_NEON_AUTH_URL"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("frontend auth script still contains direct Neon browser auth reference %s", forbidden)
		}
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
	if got := publicAuthProviderError(authProviderHTTPError{StatusCode: http.StatusNotFound}); got != "Neon Auth email/password endpoint not found. Check whether Auth URL includes /api/auth." {
		t.Fatalf("404 message = %q", got)
	}
	if got := publicAuthProviderError(authProviderHTTPError{StatusCode: http.StatusUnauthorized}); got != "Invalid email or password." {
		t.Fatalf("401 message = %q", got)
	}
	if got := publicAuthProviderError(authProviderHTTPError{StatusCode: http.StatusBadRequest, Body: `{"message":"Invalid credentials"}`}); got != "Invalid email or password." {
		t.Fatalf("invalid credentials message = %q", got)
	}
}

func TestExtractJWTIgnoresOpaqueSessionToken(t *testing.T) {
	payload := map[string]any{
		"token":   "opaque-session-token-without-jwt-segments",
		"session": map[string]any{"token": "another-opaque-session-token"},
	}
	if got := extractJWTFromAny(payload); got != "" {
		t.Fatalf("extractJWTFromAny() = %q, want empty opaque session tokens ignored", got)
	}
	if got := extractSessionToken(payload); got != "opaque-session-token-without-jwt-segments" {
		t.Fatalf("extractSessionToken() = %q, want opaque session token preserved for provider follow-up", got)
	}
}

func TestExtractJWTAcceptsJWTLookingToken(t *testing.T) {
	jwt := strings.Repeat("a", 18) + "." + strings.Repeat("b", 18) + "." + strings.Repeat("c", 18)
	payload := map[string]any{"session": map[string]any{"token": "opaque-session-token"}, "access_token": jwt}
	if got := extractJWTFromAny(payload); got != jwt {
		t.Fatalf("extractJWTFromAny() = %q, want JWT-looking token", got)
	}
}

func TestParseAndVerifyNeonJWTIssuerMismatchSafeError(t *testing.T) {
	_, token := setupEdDSANeonJWTTest(t, neonJWTClaims{Sub: "user_1", Email: "user@example.com", Iss: "https://wrong.example.test", Exp: time.Now().Add(time.Hour).Unix()}, "kid-issuer")
	t.Setenv("NEON_AUTH_ISSUER", "https://issuer.example.test")

	_, err := parseAndVerifyNeonJWT(token)
	if err == nil {
		t.Fatal("parseAndVerifyNeonJWT() error = nil, want issuer mismatch")
	}
	if got := publicAuthProviderError(err); got != "issuer_mismatch: check NEON_AUTH_ISSUER against provider token issuer" {
		t.Fatalf("publicAuthProviderError() = %q", got)
	}
	if strings.Contains(publicAuthProviderError(err), token) {
		t.Fatalf("public error must not contain JWT: %s", publicAuthProviderError(err))
	}
}

func TestParseAndVerifyNeonJWTJWKSKeyMissingSafeError(t *testing.T) {
	setupJWKS(t, neonJWKSDoc{Keys: []neonJWK{}})
	_, token := signedEdDSATestJWT(t, neonJWTClaims{Sub: "user_1", Email: "user@example.com", Iss: "https://issuer.example.test", Exp: time.Now().Add(time.Hour).Unix()}, "missing-kid")
	t.Setenv("NEON_AUTH_ISSUER", "https://issuer.example.test")

	_, err := parseAndVerifyNeonJWT(token)
	if err == nil {
		t.Fatal("parseAndVerifyNeonJWT() error = nil, want JWKS key missing")
	}
	if got := publicAuthProviderError(err); got != "jwks_key_not_found: check NEON_AUTH_JWKS_URL" {
		t.Fatalf("publicAuthProviderError() = %q", got)
	}
}

func TestFetchBetterAuthVerifiedJWTFallsBackWithCookies(t *testing.T) {
	_, token := setupEdDSANeonJWTTest(t, neonJWTClaims{Sub: "user_1", Email: "user@example.com", Iss: "https://issuer.example.test", Exp: time.Now().Add(time.Hour).Unix()}, "kid-fallback")
	t.Setenv("NEON_AUTH_ISSUER", "https://issuer.example.test")

	oldClient := authProviderHTTPClient
	defer func() { authProviderHTTPClient = oldClient }()
	var requested []string
	authProviderHTTPClient = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requested = append(requested, r.URL.Path)
		if got := r.Header.Get("Cookie"); got != "better-auth.session=signed-cookie; other=value" {
			t.Fatalf("Cookie header = %q, want sign-in Set-Cookie values", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer opaque-session" {
			t.Fatalf("Authorization header = %q, want opaque session bearer for provider follow-up", got)
		}
		switch r.URL.Path {
		case "/api/auth/token":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"opaque-token"}`)), Header: make(http.Header)}, nil
		case "/api/auth/get-session":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"user":{"email":"user@example.com"}}`)), Header: make(http.Header)}, nil
		case "/api/auth/session":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"access_token":"` + token + `"}`)), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected provider path: %s", r.URL.Path)
			return nil, nil
		}
	})

	gotToken, claims, err := fetchBetterAuthVerifiedJWT(
		context.Background(),
		betterAuthConfig{BaseURL: "https://auth.example.test/api/auth", IssuerURL: "https://issuer.example.test", JWKSURL: "unused"},
		[]string{"better-auth.session=signed-cookie; Path=/; HttpOnly", "other=value; Path=/"},
		"opaque-session",
	)
	if err != nil {
		t.Fatalf("fetchBetterAuthVerifiedJWT() error = %v", err)
	}
	if gotToken != token {
		t.Fatalf("token = %q, want fallback JWT", gotToken)
	}
	if claims.Email != "user@example.com" {
		t.Fatalf("claims.Email = %q", claims.Email)
	}
	if strings.Join(requested, ",") != "/api/auth/token,/api/auth/get-session,/api/auth/session" {
		t.Fatalf("requested paths = %v", requested)
	}
}

func setupEdDSANeonJWTTest(t *testing.T, claims neonJWTClaims, kid string) (neonJWK, string) {
	t.Helper()
	jwk, token := signedEdDSATestJWT(t, claims, kid)
	setupJWKS(t, neonJWKSDoc{Keys: []neonJWK{jwk}})
	return jwk, token
}

func signedEdDSATestJWT(t *testing.T, claims neonJWTClaims, kid string) (neonJWK, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	header := map[string]any{"alg": "EdDSA", "kid": kid, "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	headerPart := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadPart := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := headerPart + "." + payloadPart
	sig := ed25519.Sign(priv, []byte(signingInput))
	token := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
	jwk := neonJWK{Kid: kid, Kty: "OKP", Alg: "EdDSA", Crv: "Ed25519", X: base64.RawURLEncoding.EncodeToString(pub)}
	return jwk, token
}

func setupJWKS(t *testing.T, doc neonJWKSDoc) {
	t.Helper()
	jwksMu.Lock()
	jwksCache = nil
	jwksMu.Unlock()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		jwksMu.Lock()
		jwksCache = nil
		jwksMu.Unlock()
	})
	t.Setenv("NEON_AUTH_JWKS_URL", server.URL+"/jwks.json")
}
