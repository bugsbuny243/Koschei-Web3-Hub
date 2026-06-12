package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func notImplemented(w http.ResponseWriter, name string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "not_implemented", "handler": name})
}

type emailAuthRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Name        string `json:"name"`
	CallbackURL string `json:"callbackURL"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	h.emailPasswordAuth(w, r, "/sign-up/email")
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	h.emailPasswordAuth(w, r, "/sign-in/email")
}

type neonEmailAuthResult struct {
	StatusCode int
	Data       map[string]any
	Body       []byte
	Token      string
	TokenFound bool
}

func (h *Handler) emailPasswordAuth(w http.ResponseWriter, r *http.Request, neonPath string) {
	var req emailAuthRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || strings.TrimSpace(req.Password) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email_and_password_required"})
		return
	}
	result, ok := h.callNeonEmailPasswordAuth(r, emailAuthRequest{Email: email, Password: req.Password, Name: req.Name, CallbackURL: req.CallbackURL}, neonPath)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "neon_auth_not_configured"})
		return
	}
	endpoint := neonEmailAuthEndpointName(neonPath)
	if result.StatusCode/100 != 2 {
		if isNeonCallbackURLError(result.Data) {
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "auth_callback_url_invalid",
				"message": "Auth callback URL is not configured correctly.",
			})
			return
		}
		writeJSON(w, result.StatusCode, result.Data)
		return
	}
	jwt := result.Token
	if jwt == "" && neonPath == "/sign-up/email" {
		fallback, fallbackOK := h.callNeonEmailPasswordAuth(r, emailAuthRequest{Email: email, Password: req.Password, CallbackURL: req.CallbackURL}, "/sign-in/email")
		if !fallbackOK {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "neon_auth_not_configured"})
			return
		}
		if fallback.StatusCode/100 != 2 {
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "auth_session_missing",
				"message": "Account was created, but no session token was returned. Please sign in.",
			})
			return
		}
		jwt = fallback.Token
		result = fallback
		endpoint = "signup_fallback_login"
	}
	if jwt == "" {
		if neonPath == "/sign-in/email" {
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "auth_session_missing",
				"message": "Login succeeded, but no session token was returned by the auth provider.",
			})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error":   "auth_session_missing",
			"message": "Account was created, but no session token was returned. Please sign in.",
		})
		return
	}
	claims, err := parseAndVerifyNeonJWT(jwt)
	tokenVerified := err == nil
	safeAuthDebugLog(endpoint+"_verify", result.StatusCode, result.Body, nil, result.TokenFound, tokenVerified)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
		return
	}
	profile, err := h.authSuccessProfile(r, claims)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile_provision_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "token": jwt, "access_token": jwt, "token_type": "Bearer", "user": profile})
}

func (h *Handler) callNeonEmailPasswordAuth(r *http.Request, req emailAuthRequest, neonPath string) (neonEmailAuthResult, bool) {
	baseURL := strings.TrimRight(strings.TrimSpace(configuredNeonAuthBaseURL()), "/")
	if baseURL == "" {
		return neonEmailAuthResult{}, false
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	payload := map[string]string{"email": email, "password": req.Password, "callbackURL": absoluteEmailAuthCallbackURL(r, req.CallbackURL)}
	if strings.TrimSpace(req.Name) != "" {
		payload["name"] = strings.TrimSpace(req.Name)
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	neonReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, baseURL+neonPath, bytes.NewReader(body))
	if err != nil {
		return neonEmailAuthResult{StatusCode: http.StatusInternalServerError, Data: map[string]any{"error": "auth_request_failed"}}, true
	}
	neonReq.Header.Set("Content-Type", "application/json")
	if origin := publicBaseURL(r); origin != "" {
		neonReq.Header.Set("Origin", origin)
	}
	resp, err := client.Do(neonReq)
	if err != nil {
		return neonEmailAuthResult{StatusCode: http.StatusBadGateway, Data: map[string]any{"error": "neon_auth_unavailable"}}, true
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	data := map[string]any{}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &data)
	}
	jwt, tokenFound := extractAuthToken(resp, respBody)
	safeAuthDebugLog(neonEmailAuthEndpointName(neonPath), resp.StatusCode, respBody, resp.Cookies(), tokenFound, false)
	return neonEmailAuthResult{StatusCode: resp.StatusCode, Data: data, Body: respBody, Token: jwt, TokenFound: tokenFound}, true
}

func (h *Handler) authSuccessProfile(r *http.Request, claims neonJWTClaims) (map[string]any, error) {
	profile := map[string]any{"auth_subject": claims.Sub, "email": claims.Email, "role": "member", "plan_id": "free", "plan": "free", "credits": 0, "outputs_total": 0, "outputs_remaining": 0}
	if h.DB == nil {
		return profile, nil
	}
	summary, err := h.provisionMember(r.Context(), claims)
	if err != nil {
		return nil, err
	}
	p, err := h.upsertProfile(r.Context(), claims.Sub, claims.Email)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":                p.ID,
		"auth_subject":      p.AuthSubject,
		"email":             p.Email,
		"role":              firstNonEmpty(p.Role, "member"),
		"plan_id":           firstNonEmpty(summary.Plan, p.PlanID, "free"),
		"plan":              firstNonEmpty(summary.Plan, p.PlanID, "free"),
		"credits":           p.Credits,
		"outputs_total":     summary.OutputsTotal,
		"outputs_remaining": summary.OutputsRemaining,
	}, nil
}

func neonEmailAuthEndpointName(neonPath string) string {
	if neonPath == "/sign-up/email" {
		return "signup"
	}
	if neonPath == "/sign-in/email" {
		return "login"
	}
	return strings.Trim(strings.ReplaceAll(neonPath, "/", "_"), "_")
}

func absoluteEmailAuthCallbackURL(r *http.Request, requested string) string {
	requested = strings.TrimSpace(requested)
	fallback := absolutePublicURL(r, "/hub.html")
	if requested == "" || strings.ContainsAny(requested, "\r\n") {
		return fallback
	}
	if parsed, err := url.Parse(requested); err == nil && parsed.IsAbs() && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		baseURL, baseErr := url.Parse(publicBaseURL(r))
		if baseErr != nil || baseURL.Host == "" || strings.EqualFold(parsed.Host, baseURL.Host) {
			return strings.TrimRight(parsed.String(), "/")
		}
		return fallback
	}
	if strings.HasPrefix(requested, "/") && !strings.HasPrefix(requested, "//") {
		return absolutePublicURL(r, requested)
	}
	return fallback
}

func isNeonCallbackURLError(data map[string]any) bool {
	message := strings.ToLower(authErrorText(data))
	return strings.Contains(message, "origin header is required") ||
		(strings.Contains(message, "callbackurl") && strings.Contains(message, "absolute url")) ||
		(strings.Contains(message, "callback url") && strings.Contains(message, "absolute url"))
}

func authErrorText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case map[string]any:
		parts := make([]string, 0, len(v))
		for _, nested := range v {
			parts = append(parts, authErrorText(nested))
		}
		return strings.Join(parts, " ")
	case []any:
		parts := make([]string, 0, len(v))
		for _, nested := range v {
			parts = append(parts, authErrorText(nested))
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func firstJWT(header string, data map[string]any) string {
	if tokenLooksLikeJWT(header) {
		return header
	}
	for _, key := range []string{"access_token", "token", "jwt", "id_token"} {
		if value, ok := data[key].(string); ok && tokenLooksLikeJWT(value) {
			return value
		}
	}
	for _, key := range []string{"session", "data"} {
		if nested, ok := data[key].(map[string]any); ok {
			if jwt := firstJWT("", nested); jwt != "" {
				return jwt
			}
		}
	}
	return ""
}

func (h *Handler) StartOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "StartOTPLogin")
}
func (h *Handler) VerifyOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "VerifyOTPLogin")
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	profile := map[string]any{"auth_subject": claims.Sub, "email": claims.Email, "role": "member", "plan_id": "free", "plan": "free", "credits": 0, "outputs_total": 0, "outputs_remaining": 0}
	if h.DB != nil {
		summary, err := h.provisionMember(r.Context(), claims)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile_unavailable"})
			return
		}
		p, err := h.upsertProfile(r.Context(), claims.Sub, claims.Email)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile_unavailable"})
			return
		}
		profile = map[string]any{
			"id":                p.ID,
			"auth_subject":      p.AuthSubject,
			"email":             p.Email,
			"role":              firstNonEmpty(p.Role, "member"),
			"plan_id":           firstNonEmpty(summary.Plan, p.PlanID, "free"),
			"plan":              firstNonEmpty(summary.Plan, p.PlanID, "free"),
			"credits":           p.Credits,
			"outputs_total":     summary.OutputsTotal,
			"outputs_remaining": summary.OutputsRemaining,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": profile})
}

func (h *Handler) AIGenerate(w http.ResponseWriter, r *http.Request) { notImplemented(w, "AIGenerate") }
func (h *Handler) AIJobs(w http.ResponseWriter, r *http.Request)     { notImplemented(w, "AIJobs") }
