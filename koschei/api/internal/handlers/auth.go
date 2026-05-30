package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
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
	writeJSON(w, http.StatusGone, map[string]string{"error": "custom_auth_disabled", "message": "Use Neon Auth / Better Auth email and password sign-in"})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in emailPasswordLoginRequest
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	email, err := normalizeEmail(in.Email)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_email"})
		return
	}
	if in.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_password", "message": "Email and password are required."})
		return
	}
	cfg, err := betterAuthConfigFromEnv()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth_not_configured", "message": err.Error()})
		return
	}
	signInResp, setCookies, signInBaseURL, err := postBetterAuthEmailPasswordWithFallback(r.Context(), cfg, map[string]string{"email": email, "password": in.Password})
	if err != nil {
		writeJSON(w, authProviderStatusCode(err), map[string]string{"error": "auth_provider_failed", "message": publicAuthProviderError(err)})
		return
	}
	accessToken := extractJWTFromAny(signInResp)
	if accessToken == "" {
		accessToken, err = fetchBetterAuthJWT(r.Context(), cfg.withBaseURL(signInBaseURL), setCookies, extractSessionToken(signInResp))
		if err != nil {
			writeJSON(w, authProviderStatusCode(err), map[string]string{"error": "auth_provider_failed", "message": publicAuthProviderError(err)})
			return
		}
	}
	claims, err := parseAndVerifyNeonJWT(accessToken)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "auth_provider_failed", "message": "auth provider returned a token that this API could not verify"})
		return
	}
	user, err := h.upsertAppProfile(r.Context(), claims.Sub, claims.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
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
	out := authUser{}
	q := `INSERT INTO app_user_profiles (auth_subject, email)
VALUES ($1, $2)
ON CONFLICT (auth_subject) DO UPDATE SET email=EXCLUDED.email, updated_at=now()
RETURNING id::text, email, role, plan_id, credits`
	err := h.runWithRetry(ctx, func(inner context.Context) error {
		return h.DB.QueryRowContext(inner, q, subject, strings.ToLower(strings.TrimSpace(email))).Scan(&out.ID, &out.Email, &out.Role, &out.Plan, &out.Credits)
	})
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
		BaseURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("NEON_AUTH_BASE_URL")), "/"),
		IssuerURL: strings.TrimRight(strings.TrimSpace(os.Getenv("NEON_AUTH_ISSUER")), "/"),
		JWKSURL:   strings.TrimSpace(os.Getenv("NEON_AUTH_JWKS_URL")),
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
	for _, baseURL := range emailPasswordSignInBaseURLCandidates(cfg.BaseURL) {
		respBody, setCookies, err := postBetterAuthWithCookiesURL(ctx, baseURL+"/sign-in/email", payload)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+"/token", nil)
	if err != nil {
		return "", err
	}
	if cookieHeader := cookieHeaderFromSetCookies(setCookies); cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}
	if sessionToken != "" {
		req.Header.Set("Authorization", "Bearer "+sessionToken)
	}
	resp, err := authProviderHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		return "", authProviderHTTPError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	if token := extractJWTFromHeaders(resp.Header); token != "" {
		return token, nil
	}
	if token := extractJWTFromAny(string(bytes.TrimSpace(respBody))); token != "" {
		return token, nil
	}
	var payload any
	if len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, &payload); err != nil {
			return "", err
		}
	}
	if token := extractJWTFromAny(payload); token != "" {
		return token, nil
	}
	return "", errors.New("auth provider did not return a JWT")
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
	for _, key := range []string{"Authorization", "set-auth-jwt"} {
		for _, value := range header.Values(key) {
			value = strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
			if tokenLooksLikeJWT(value) {
				return value
			}
		}
	}
	return ""
}

func extractJWTFromAny(value any) string {
	switch v := value.(type) {
	case string:
		v = strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
		if tokenLooksLikeJWT(v) {
			return v
		}
	case map[string]any:
		for _, key := range []string{"access_token", "accessToken", "id_token", "idToken", "token", "jwt"} {
			if token := extractJWTFromAny(v[key]); token != "" {
				return token
			}
		}
		for _, item := range v {
			if token := extractJWTFromAny(item); token != "" {
				return token
			}
		}
	case []any:
		for _, item := range v {
			if token := extractJWTFromAny(item); token != "" {
				return token
			}
		}
	}
	return ""
}

func tokenLooksLikeJWT(value string) bool {
	return strings.Count(value, ".") == 2 && len(value) > 40
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
	var endpointErr authProviderEmailEndpointNotFoundError
	if errors.As(err, &endpointErr) {
		return "Neon Auth email/password endpoint not found. Check whether Auth URL includes /api/auth."
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
