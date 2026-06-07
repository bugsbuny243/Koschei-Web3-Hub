package handlers

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

func TestBackendPasswordAuthEndpointsAreDisabled(t *testing.T) {
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

			if w.Code != http.StatusGone {
				t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusGone, w.Body.String())
			}
			if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
				t.Fatalf("Content-Type = %q, want JSON", got)
			}
			body := w.Body.String()
			if !strings.Contains(body, "Use direct Neon Auth") {
				t.Fatalf("response did not explain direct Neon auth mode: %s", body)
			}
		})
	}
}

func TestConfigExposesOnlyPublicNeonAuthURL(t *testing.T) {
	t.Setenv("EXPO_PUBLIC_NEON_AUTH_URL", "https://public-auth.example.test/api/auth")
	t.Setenv("NEON_AUTH_BASE_URL", "https://server-auth.example.test/api/auth")
	t.Setenv("DATABASE_URL", "postgres://secret@example.test/db")
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	(&Handler{}).Config(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "neonAuthUrl") || !strings.Contains(body, "https://public-auth.example.test/api/auth") {
		t.Fatalf("/api/config did not expose public Neon Auth URL: %s", body)
	}
	if strings.Contains(body, "postgres://") || strings.Contains(body, "DATABASE_URL") || strings.Contains(body, "server-auth.example.test") {
		t.Fatalf("/api/config exposed private config: %s", body)
	}
}

func TestFrontendAuthUsesDirectNeonEndpoints(t *testing.T) {
	body, err := os.ReadFile("../../public/js/koschei-auth.js")
	if err != nil {
		t.Fatalf("read frontend auth script: %v", err)
	}
	script := string(body)
	for _, want := range []string{"neonAuthUrl", "sign-up/email", "sign-in/email", "set-auth-jwt", "koschei_jwt", "/api/me", "credentials: 'include'"} {
		if !strings.Contains(script, want) {
			t.Fatalf("frontend auth script does not contain %s", want)
		}
	}
	for _, forbidden := range []string{"/api/auth/register", "/api/auth/login", "/api/auth/neon-login", "/api/auth/neon-register", "callbackURL", "redirect_uri"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("frontend auth script still contains backend/redirect auth reference %s", forbidden)
		}
	}
}

func TestFrontendLoginRegisterUseEmailPasswordForms(t *testing.T) {
	cases := []struct {
		file      string
		wants     []string
		forbidden []string
	}{
		{
			file: "../../public/login.html",
			wants: []string{
				`id="loginForm"`,
				`name="email"`,
				`name="password"`,
				"KoscheiAuth.signIn",
			},
			forbidden: []string{"/api/auth/neon-login", "Continue with Neon Auth", "hosted UI only"},
		},
		{
			file: "../../public/register.html",
			wants: []string{
				`id="registerForm"`,
				`name="email"`,
				`name="password"`,
				"KoscheiAuth.signUp",
			},
			forbidden: []string{"/api/auth/neon-register", "Continue with Neon Auth", "hosted UI"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			body, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("read frontend page: %v", err)
			}
			page := string(body)
			for _, want := range tc.wants {
				if !strings.Contains(page, want) {
					t.Fatalf("frontend page %s does not contain %s", tc.file, want)
				}
			}
			for _, forbidden := range tc.forbidden {
				if strings.Contains(page, forbidden) {
					t.Fatalf("frontend page %s still contains hosted auth reference %s", tc.file, forbidden)
				}
			}
		})
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

type fakeSQLResult struct{}

func (fakeSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeSQLResult) RowsAffected() (int64, error) { return 0, nil }

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = r.values[i].(string)
		case *int:
			*d = r.values[i].(int)
		case *sql.NullString:
			if r.values[i] == nil {
				*d = sql.NullString{}
			} else {
				*d = sql.NullString{String: r.values[i].(string), Valid: true}
			}
		default:
			return fmt.Errorf("unsupported scan destination %T", dest[i])
		}
	}
	return nil
}

type fakeAuthStore struct {
	emailRowID       string
	subjectRowID     string
	queries          []string
	execs            []string
	freeInsertCount  int
	freeInsertArgs   []any
	profileReturnID  string
	summaryPlan      string
	summaryTotal     int
	summaryRemaining int
}

func (s *fakeAuthStore) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	s.execs = append(s.execs, query)
	if strings.Contains(query, "INSERT INTO entitlements") {
		s.freeInsertCount++
		s.freeInsertArgs = append([]any(nil), args...)
	}
	return fakeSQLResult{}, nil
}

func (s *fakeAuthStore) QueryRowContext(_ context.Context, query string, args ...any) rowScanner {
	s.queries = append(s.queries, query)
	sq := strings.ToLower(query)
	switch {
	case strings.Contains(sq, "select id::text") && strings.Contains(sq, "where lower(email)"):
		if s.emailRowID == "" {
			return fakeRow{err: sql.ErrNoRows}
		}
		return fakeRow{values: []any{s.emailRowID}}
	case strings.Contains(sq, "select id::text") && strings.Contains(sq, "where auth_subject"):
		if s.subjectRowID == "" {
			return fakeRow{err: sql.ErrNoRows}
		}
		return fakeRow{values: []any{s.subjectRowID}}
	case strings.Contains(sq, "returning id::text, email, role, plan_id, credits"):
		id := s.profileReturnID
		if id == "" {
			id = s.emailRowID
		}
		if id == "" {
			id = s.subjectRowID
		}
		if id == "" {
			id = "new-profile-id"
		}
		return fakeRow{values: []any{id, args[1].(string), "user", "free", 0}}
	case strings.Contains(sq, "with active_entitlements"):
		plan := s.summaryPlan
		if plan == "" {
			plan = "free"
		}
		return fakeRow{values: []any{plan, s.summaryTotal, s.summaryRemaining}}
	default:
		return fakeRow{err: fmt.Errorf("unexpected query: %s", query)}
	}
}

func TestUpsertAppProfileExistingNeonUserMissingLocalProfileInserts(t *testing.T) {
	store := &fakeAuthStore{}
	var user authUser
	if err := upsertAppProfileTx(context.Background(), store, "neon-sub-1", "USER@Example.COM", &user); err != nil {
		t.Fatalf("upsertAppProfileTx() error = %v", err)
	}
	if user.ID != "new-profile-id" || user.Email != "user@example.com" || user.Plan != "free" {
		t.Fatalf("unexpected user after insert: %#v", user)
	}
	if len(store.queries) != 3 || !strings.Contains(store.queries[2], "INSERT INTO app_user_profiles") {
		t.Fatalf("expected missing local profile path to insert once; queries=%v", store.queries)
	}
}

func TestUpsertAppProfileExistingEmailRowWithNewAuthSubjectUpdatesEmailRow(t *testing.T) {
	store := &fakeAuthStore{emailRowID: "profile-by-email"}
	var user authUser
	if err := upsertAppProfileTx(context.Background(), store, "new-neon-sub", "User@Example.com", &user); err != nil {
		t.Fatalf("upsertAppProfileTx() error = %v", err)
	}
	if user.ID != "profile-by-email" || user.Email != "user@example.com" {
		t.Fatalf("unexpected user after email-row update: %#v", user)
	}
	if len(store.queries) != 2 || !strings.Contains(store.queries[1], "WHERE id::text = $3") {
		t.Fatalf("expected email row update without duplicate insert; queries=%v", store.queries)
	}
	if len(store.execs) < 2 || !strings.Contains(store.execs[1], "SET auth_subject = NULL") {
		t.Fatalf("expected conflicting subject cleanup before email update; execs=%v", store.execs)
	}
}

func TestProvisionMemberCreatesFreeEntitlementOnce(t *testing.T) {
	store := &fakeAuthStore{summaryTotal: 10, summaryRemaining: 10}
	summary, err := provisionMemberTx(context.Background(), store, "neon-sub-1", "user@example.com")
	if err != nil {
		t.Fatalf("provisionMemberTx() error = %v", err)
	}
	if summary.Plan != "free" || summary.OutputsTotal != 10 || summary.OutputsRemaining != 10 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if store.freeInsertCount != 1 {
		t.Fatalf("free entitlement insert count = %d, want 1", store.freeInsertCount)
	}
	if got := store.freeInsertArgs[1]; got != 10 {
		t.Fatalf("free entitlement outputs argument = %#v, want 10", got)
	}
	if !strings.Contains(store.execs[len(store.execs)-1], "WHERE NOT EXISTS") || !strings.Contains(store.execs[len(store.execs)-1], "COALESCE(plan_id, 'free') = 'free'") {
		t.Fatalf("free entitlement insert must be idempotent; query=%s", store.execs[len(store.execs)-1])
	}
}
