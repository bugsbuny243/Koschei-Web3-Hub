package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type authUser struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Plan    string `json:"plan"`
	Credits int    `json:"credits"`
}

type emailPasswordLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type betterAuthConfig struct {
	BaseURL   string
	IssuerURL string
	JWKSURL   string
}

type authProviderTransport interface {
	Do(*http.Request) (*http.Response, error)
}

var authProviderHTTPClient authProviderTransport = &http.Client{Timeout: 10 * time.Second}

func (h *Handler) Register(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error":   "disabled",
		"message": "Use direct Neon Auth /sign-up/email from the frontend.",
	})
}

func (h *Handler) Login(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error":   "disabled",
		"message": "Use direct Neon Auth /sign-in/email from the frontend.",
	})
}

func validatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("Email and password are required.")
	}
	if len(password) < 8 {
		return errors.New("Password must be at least 8 characters.")
	}
	return nil
}

func defaultUserName(email string) string {
	name := strings.TrimSpace(strings.Split(email, "@")[0])
	if name == "" {
		return "User"
	}
	return name
}

func (h *Handler) finishEmailPasswordAuth(w http.ResponseWriter, r *http.Request, cfg betterAuthConfig, email, password string, provision bool) {
	signInResp, setCookies, signInBaseURL, err := postBetterAuthEmailPasswordWithFallback(r.Context(), cfg, map[string]string{"email": email, "password": password})
	if err != nil {
		writeJSON(w, authProviderStatusCode(err), map[string]string{"error": "auth_provider_failed", "message": publicAuthProviderError(err)})
		return
	}
	cfg = cfg.withBaseURL(signInBaseURL)
	accessToken := extractJWTFromAny(signInResp)
	var firstVerifyErr error
	claims, err := parseAndVerifyNeonJWT(accessToken)
	if accessToken == "" || err != nil {
		if accessToken != "" {
			firstVerifyErr = err
		}
		accessToken, claims, err = fetchBetterAuthVerifiedJWT(r.Context(), cfg, setCookies, extractSessionToken(signInResp))
		if err != nil {
			if firstVerifyErr != nil {
				err = firstVerifyErr
			}
			writeJSON(w, authProviderStatusCode(err), map[string]string{"error": "auth_provider_failed", "message": publicAuthProviderError(err)})
			return
		}
	}
	user, err := h.upsertAppProfile(r.Context(), claims.Sub, claims.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if provision {
		summary, err := h.provisionMember(r.Context(), claims)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "account_provisioning_failed"})
			return
		}
		user.Plan = summary.Plan
		user.Credits = summary.OutputsRemaining
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": accessToken, "token_type": "Bearer", "user": user})
}

func (h *Handler) StartOTPLogin(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{"error": "email_password_required", "message": "Neon Auth / Better Auth email OTP is not enabled. Use email and password sign-in."})
}

func (h *Handler) VerifyOTPLogin(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{"error": "email_password_required", "message": "Neon Auth / Better Auth email OTP is not enabled. Use email and password sign-in."})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if err := h.dbAvailable(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Email) == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	user, err := h.upsertAppProfile(r.Context(), claims.Sub, claims.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *Handler) upsertAppProfile(ctx context.Context, subject, email string) (authUser, error) {
	out := authUser{Role: "user", Plan: "free", Credits: 0}
	q := `INSERT INTO app_user_profiles (auth_subject, email)
VALUES ($1, lower($2))
ON CONFLICT (auth_subject)
DO UPDATE SET email = EXCLUDED.email, updated_at = now()
RETURNING id::text, email, role, plan_id, credits;`
	err := h.runWithRetry(ctx, func(inner context.Context) error {
		return h.DB.QueryRowContext(inner, q, subject, strings.ToLower(strings.TrimSpace(email))).Scan(&out.ID, &out.Email, &out.Role, &out.Plan, &out.Credits)
	})
	if err != nil {
		log.Printf("upsertAppProfile failed: %v", err)
	}
	return out, err
}

func (h *Handler) runWithRetry(ctx context.Context, op func(context.Context) error) error {
	err := op(ctx)
	if !isTransientDBError(err) {
		return err
	}
	_ = h.dbAvailable(ctx)
	return op(ctx)
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 254 {
		return "", errors.New("invalid email")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || strings.Contains(email, " ") {
		return "", errors.New("invalid email")
	}
	return email, nil
}

func betterAuthConfigFromEnv() (betterAuthConfig, error) {
	cfg := betterAuthConfig{
		BaseURL:   strings.TrimRight(configuredNeonAuthBaseURL(), "/"),
		IssuerURL: strings.TrimRight(configuredNeonAuthIssuer(), "/"),
		JWKSURL:   configuredNeonAuthJWKSURL(),
	}
	missing := []string{}
	if cfg.BaseURL == "" {
		missing = append(missing, "NEON_AUTH_BASE_URL")
	}
	if cfg.IssuerURL == "" {
		missing = append(missing, "NEON_AUTH_ISSUER")
	}
	if cfg.JWKSURL == "" {
		missing = append(missing, "NEON_AUTH_JWKS_URL")
	}
	if len(missing) > 0 {
		return cfg, errors.New(strings.Join(missing, " and ") + " must be set for Neon Auth / Better Auth email/password login")
	}
	return cfg, nil
}

func (cfg betterAuthConfig) withBaseURL(baseURL string) betterAuthConfig {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return cfg
}

func postBetterAuth(ctx context.Context, cfg betterAuthConfig, path string, payload any, out any) error {
	respBody, _, err := postBetterAuthWithCookies(ctx, cfg, path, payload)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	encoded, err := json.Marshal(respBody)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, out)
}

func postBetterAuthWithCookies(ctx context.Context, cfg betterAuthConfig, path string, payload any) (map[string]any, []string, error) {
	return postBetterAuthWithCookiesURL(ctx, cfg.BaseURL+path, payload)
}

func postBetterAuthEmailPasswordWithFallback(ctx context.Context, cfg betterAuthConfig, payload any) (map[string]any, []string, string, error) {
	return postBetterAuthEmailPasswordPathWithFallback(ctx, cfg, "/sign-in/email", payload)
}

func postBetterAuthEmailPasswordPathWithFallback(ctx context.Context, cfg betterAuthConfig, path string, payload any) (map[string]any, []string, string, error) {
	for _, baseURL := range emailPasswordSignInBaseURLCandidates(cfg.BaseURL) {
		respBody, setCookies, err := postBetterAuthWithCookiesURL(ctx, baseURL+path, payload)
		if isAuthProviderNotFound(err) {
			continue
		}
		return respBody, setCookies, baseURL, err
	}
	return nil, nil, "", authProviderHTTPError{StatusCode: http.StatusNotFound}
}

func emailPasswordSignInBaseURLCandidates(baseURL string) []string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return []string{
		baseURL,
		strings.TrimSuffix(baseURL, "/api/auth") + "/api/auth",
		strings.TrimSuffix(baseURL, "/auth") + "/api/auth",
	}
}

func isAuthProviderNotFound(err error) bool {
	var httpErr authProviderHTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound
}

func postBetterAuthWithCookiesURL(ctx context.Context, url string, payload any) (map[string]any, []string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := authProviderHTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		return nil, nil, authProviderHTTPError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	out := map[string]any{}
	if len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, &out); err != nil {
			return nil, nil, err
		}
	}
	return out, resp.Header.Values("Set-Cookie"), nil
}

func fetchBetterAuthJWT(ctx context.Context, cfg betterAuthConfig, setCookies []string, sessionToken string) (string, error) {
	token, _, err := fetchBetterAuthVerifiedJWT(ctx, cfg, setCookies, sessionToken)
	return token, err
}

func fetchBetterAuthVerifiedJWT(ctx context.Context, cfg betterAuthConfig, setCookies []string, sessionToken string) (string, neonJWTClaims, error) {
	var lastVerifyErr error
	var sawJWT bool
	var lastHTTPErr error
	for _, path := range []string{"/token", "/get-session", "/session"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+path, nil)
		if err != nil {
			return "", neonJWTClaims{}, err
		}
		if cookieHeader := cookieHeaderFromSetCookies(setCookies); cookieHeader != "" {
			req.Header.Set("Cookie", cookieHeader)
		}
		if sessionToken != "" {
			req.Header.Set("Authorization", "Bearer "+sessionToken)
		}
		resp, err := authProviderHTTPClient.Do(req)
		if err != nil {
			lastHTTPErr = err
			continue
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			lastHTTPErr = authProviderHTTPError{StatusCode: resp.StatusCode, Body: string(respBody)}
			continue
		}
		for _, token := range extractJWTsFromResponse(resp.Header, respBody) {
			sawJWT = true
			claims, err := parseAndVerifyNeonJWT(token)
			if err == nil {
				return token, claims, nil
			}
			lastVerifyErr = err
		}
	}
	if lastVerifyErr != nil {
		return "", neonJWTClaims{}, lastVerifyErr
	}
	if sawJWT {
		return "", neonJWTClaims{}, errors.New("auth provider did not return a verifiable JWT")
	}
	if lastHTTPErr != nil {
		return "", neonJWTClaims{}, lastHTTPErr
	}
	return "", neonJWTClaims{}, errors.New("auth provider did not return a JWT")
}

func extractJWTsFromResponse(header http.Header, respBody []byte) []string {
	tokens := extractJWTsFromHeaders(header)
	trimmed := bytes.TrimSpace(respBody)
	if len(trimmed) == 0 {
		return tokens
	}
	tokens = append(tokens, extractJWTsFromAny(string(trimmed))...)
	var payload any
	if json.Unmarshal(trimmed, &payload) == nil {
		tokens = append(tokens, extractJWTsFromAny(payload)...)
	}
	return uniqueStrings(tokens)
}

func cookieHeaderFromSetCookies(values []string) string {
	parts := []string{}
	for _, value := range values {
		if first := strings.TrimSpace(strings.Split(value, ";")[0]); first != "" {
			parts = append(parts, first)
		}
	}
	return strings.Join(parts, "; ")
}

func extractSessionToken(payload map[string]any) string {
	for _, key := range []string{"token", "session_token", "sessionToken"} {
		if token, ok := payload[key].(string); ok && strings.TrimSpace(token) != "" && !tokenLooksLikeJWT(token) {
			return strings.TrimSpace(token)
		}
	}
	if session, ok := payload["session"].(map[string]any); ok {
		return extractSessionToken(session)
	}
	return ""
}

func extractJWTFromHeaders(header http.Header) string {
	if tokens := extractJWTsFromHeaders(header); len(tokens) > 0 {
		return tokens[0]
	}
	return ""
}

func extractJWTsFromHeaders(header http.Header) []string {
	tokens := []string{}
	for _, key := range []string{"Authorization", "set-auth-jwt"} {
		for _, value := range header.Values(key) {
			value = strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
			if tokenLooksLikeJWT(value) {
				tokens = append(tokens, value)
			}
		}
	}
	return tokens
}

func extractJWTFromAny(value any) string {
	if tokens := extractJWTsFromAny(value); len(tokens) > 0 {
		return tokens[0]
	}
	return ""
}

func extractJWTsFromAny(value any) []string {
	switch v := value.(type) {
	case string:
		v = strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
		if tokenLooksLikeJWT(v) {
			return []string{v}
		}
	case map[string]any:
		tokens := []string{}
		for _, key := range []string{"access_token", "accessToken", "id_token", "idToken", "token", "jwt"} {
			tokens = append(tokens, extractJWTsFromAny(v[key])...)
		}
		for _, item := range v {
			tokens = append(tokens, extractJWTsFromAny(item)...)
		}
		return uniqueStrings(tokens)
	case []any:
		tokens := []string{}
		for _, item := range v {
			tokens = append(tokens, extractJWTsFromAny(item)...)
		}
		return uniqueStrings(tokens)
	}
	return nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func tokenLooksLikeJWT(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), ".")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}

type authProviderHTTPError struct {
	StatusCode int
	Body       string
}

func (e authProviderHTTPError) Error() string {
	return fmt.Sprintf("auth provider returned %d", e.StatusCode)
}

type authProviderEmailEndpointNotFoundError struct{}

func (e authProviderEmailEndpointNotFoundError) Error() string {
	return "Neon Auth email/password endpoint not found"
}

func authProviderStatusCode(err error) int {
	var endpointErr authProviderEmailEndpointNotFoundError
	if errors.As(err, &endpointErr) {
		return http.StatusBadRequest
	}
	var httpErr authProviderHTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusTooManyRequests {
			return http.StatusTooManyRequests
		}
		if httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
			return http.StatusBadRequest
		}
	}
	return http.StatusBadGateway
}

func publicAuthProviderError(err error) string {
	var verifyErr neonJWTVerificationError
	if errors.As(err, &verifyErr) {
		switch verifyErr.Category {
		case neonJWTFailureIssuerMismatch:
			return "issuer_mismatch: check NEON_AUTH_ISSUER against provider token issuer"
		case neonJWTFailureJWKSKeyNotFound:
			return "jwks_key_not_found: check NEON_AUTH_JWKS_URL"
		default:
			return "auth provider returned a token that this API could not verify: " + string(verifyErr.Category)
		}
	}
	var httpErr authProviderHTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusNotFound {
			return "Neon Auth email/password endpoint not found. Check whether Auth URL includes /api/auth."
		}
		if httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden {
			return "Invalid email or password."
		}
		var payload map[string]any
		if json.Unmarshal([]byte(httpErr.Body), &payload) == nil {
			for _, key := range []string{"message", "error", "code"} {
				if v, ok := payload[key].(string); ok {
					message := strings.TrimSpace(v)
					if message == "" {
						continue
					}
					if authProviderMessageLooksLikeInvalidCredentials(message) {
						return "Invalid email or password."
					}
					return message
				}
			}
		}
		return "auth provider rejected the request"
	}
	if strings.Contains(err.Error(), "did not return a JWT") {
		return "auth provider did not return a bearer JWT"
	}
	return "auth provider is unavailable"
}

func authProviderMessageLooksLikeInvalidCredentials(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "invalid credential") ||
		strings.Contains(message, "invalid email") ||
		strings.Contains(message, "invalid password") ||
		strings.Contains(message, "wrong password") ||
		strings.Contains(message, "incorrect password")
}
