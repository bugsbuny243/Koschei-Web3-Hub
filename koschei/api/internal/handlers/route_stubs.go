package handlers

import (
	"bytes"
	"encoding/json"
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
	baseURL := strings.TrimRight(strings.TrimSpace(configuredNeonAuthBaseURL()), "/")
	if baseURL == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "neon_auth_not_configured"})
		return
	}
	payload := map[string]string{"email": email, "password": req.Password, "callbackURL": absoluteEmailAuthCallbackURL(r, req.CallbackURL)}
	if strings.TrimSpace(req.Name) != "" {
		payload["name"] = strings.TrimSpace(req.Name)
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	neonReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, baseURL+neonPath, bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "auth_request_failed"})
		return
	}
	neonReq.Header.Set("Content-Type", "application/json")
	if origin := publicBaseURL(r); origin != "" {
		neonReq.Header.Set("Origin", origin)
	}
	resp, err := client.Do(neonReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "neon_auth_unavailable"})
		return
	}
	defer resp.Body.Close()
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		data = map[string]any{}
	}
	if resp.StatusCode/100 != 2 {
		if isNeonCallbackURLError(data) {
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "auth_callback_url_invalid",
				"message": "Auth callback URL is not configured correctly.",
			})
			return
		}
		writeJSON(w, resp.StatusCode, data)
		return
	}
	jwt := firstJWT(resp.Header.Get("set-auth-jwt"), data)
	if jwt == "" {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "token_missing"})
		return
	}
	claims, err := parseAndVerifyNeonJWT(jwt)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
		return
	}
	profile := map[string]any{"auth_subject": claims.Sub, "email": claims.Email, "role": "member", "plan_id": "free", "plan": "free", "credits": 0, "outputs_total": 0, "outputs_remaining": 0}
	if h.DB != nil {
		summary, err := h.provisionMember(r.Context(), claims)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile_provision_failed"})
			return
		}
		p, err := h.upsertProfile(r.Context(), claims.Sub, claims.Email)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile_provision_failed"})
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "token": jwt, "access_token": jwt, "token_type": "Bearer", "user": profile})
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
