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
	"net/url"
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

type otpStartRequest struct {
	Email       string `json:"email"`
	CallbackURL string `json:"callback_url"`
}

type otpVerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
	Nonce string `json:"nonce"`
}

type stackOTPStartResponse struct {
	Nonce string `json:"nonce"`
}

type stackOTPVerifyResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	UserID       string `json:"user_id"`
}

type stackAuthConfig struct {
	BaseURL              string
	ProjectID            string
	PublishableClientKey string
}

type stackAuthTransport interface {
	Do(*http.Request) (*http.Response, error)
}

var stackAuthHTTPClient stackAuthTransport = &http.Client{Timeout: 10 * time.Second}

const defaultStackAuthAPIBaseURL = "https://api.stack-auth.com/api/v1"

func (h *Handler) Register(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{"error": "custom_auth_disabled", "message": "Use Neon Auth email sign-in"})
}

func (h *Handler) Login(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{"error": "custom_auth_disabled", "message": "Use Neon Auth email sign-in"})
}

func (h *Handler) StartOTPLogin(w http.ResponseWriter, r *http.Request) {
	var in otpStartRequest
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	email, err := normalizeEmail(in.Email)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_email"})
		return
	}
	cfg, err := stackAuthConfigFromEnv()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth_not_configured", "message": err.Error()})
		return
	}
	callbackURL, err := safeAuthCallbackURL(r, in.CallbackURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_callback_url"})
		return
	}
	var out stackOTPStartResponse
	if err := postStackAuth(r.Context(), cfg, "/auth/otp/send-sign-in-code", map[string]string{"email": email, "callback_url": callbackURL}, &out); err != nil {
		writeJSON(w, stackAuthStatusCode(err), map[string]string{"error": "auth_provider_failed", "message": publicStackAuthError(err)})
		return
	}
	if strings.TrimSpace(out.Nonce) == "" {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "auth_provider_failed", "message": "auth provider did not return a nonce"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"nonce": out.Nonce, "email": email})
}

func (h *Handler) VerifyOTPLogin(w http.ResponseWriter, r *http.Request) {
	var in otpVerifyRequest
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	email, err := normalizeEmail(in.Email)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_email"})
		return
	}
	code := strings.TrimSpace(in.Code)
	nonce := strings.TrimSpace(in.Nonce)
	if code == "" || nonce == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_otp_fields"})
		return
	}
	cfg, err := stackAuthConfigFromEnv()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth_not_configured", "message": err.Error()})
		return
	}
	var out stackOTPVerifyResponse
	if err := postStackAuth(r.Context(), cfg, "/auth/otp/sign-in", map[string]string{"email": email, "code": code, "nonce": nonce}, &out); err != nil {
		writeJSON(w, stackAuthStatusCode(err), map[string]string{"error": "auth_provider_failed", "message": publicStackAuthError(err)})
		return
	}
	accessToken := strings.TrimSpace(out.AccessToken)
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

func stackAuthConfigFromEnv() (stackAuthConfig, error) {
	cfg := stackAuthConfig{
		BaseURL:              strings.TrimRight(strings.TrimSpace(os.Getenv("NEON_AUTH_STACK_API_BASE_URL")), "/"),
		ProjectID:            strings.TrimSpace(os.Getenv("NEON_AUTH_PROJECT_ID")),
		PublishableClientKey: strings.TrimSpace(os.Getenv("NEON_AUTH_PUBLISHABLE_CLIENT_KEY")),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultStackAuthAPIBaseURL
	}
	if cfg.ProjectID == "" || cfg.PublishableClientKey == "" {
		return cfg, errors.New("NEON_AUTH_PROJECT_ID and NEON_AUTH_PUBLISHABLE_CLIENT_KEY are required")
	}
	return cfg, nil
}

func safeAuthCallbackURL(r *http.Request, raw string) (string, error) {
	origin := requestOrigin(r)
	if strings.TrimSpace(raw) == "" {
		return origin + "/login.html", nil
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if !u.IsAbs() {
		if !strings.HasPrefix(u.Path, "/") {
			return "", errors.New("callback must be absolute path")
		}
		return origin + u.RequestURI(), nil
	}
	allowed, _ := url.Parse(origin)
	if !strings.EqualFold(u.Scheme, allowed.Scheme) || !strings.EqualFold(u.Host, allowed.Host) {
		return "", errors.New("callback origin mismatch")
	}
	return u.String(), nil
}

func requestOrigin(r *http.Request) string {
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	if idx := strings.Index(proto, ","); idx >= 0 {
		proto = strings.TrimSpace(proto[:idx])
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if idx := strings.Index(host, ","); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	return proto + "://" + host
}

func postStackAuth(ctx context.Context, cfg stackAuthConfig, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stack-Access-Type", "client")
	req.Header.Set("X-Stack-Project-Id", cfg.ProjectID)
	req.Header.Set("X-Stack-Publishable-Client-Key", cfg.PublishableClientKey)
	resp, err := stackAuthHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		return stackAuthHTTPError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return err
	}
	return nil
}

type stackAuthHTTPError struct {
	StatusCode int
	Body       string
}

func (e stackAuthHTTPError) Error() string {
	return fmt.Sprintf("stack auth returned %d", e.StatusCode)
}

func stackAuthStatusCode(err error) int {
	var httpErr stackAuthHTTPError
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

func publicStackAuthError(err error) string {
	var httpErr stackAuthHTTPError
	if errors.As(err, &httpErr) {
		var payload map[string]any
		if json.Unmarshal([]byte(httpErr.Body), &payload) == nil {
			for _, key := range []string{"message", "error", "code"} {
				if v, ok := payload[key].(string); ok && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
		}
		return "auth provider rejected the request"
	}
	return "auth provider is unavailable"
}
