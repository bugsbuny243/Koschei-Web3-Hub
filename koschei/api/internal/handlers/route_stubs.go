package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func notImplemented(w http.ResponseWriter, name string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "not_implemented", "handler": name})
}

type emailAuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
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
	payload := map[string]string{"email": email, "password": req.Password}
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
	profile := map[string]any{"auth_subject": claims.Sub, "email": claims.Email, "role": "member", "plan_id": "free", "credits": 0}
	if h.DB != nil {
		if p, err := h.upsertProfile(r.Context(), claims.Sub, claims.Email); err == nil {
			profile = map[string]any{"id": p.ID, "auth_subject": p.AuthSubject, "email": p.Email, "role": firstNonEmpty(p.Role, "member"), "plan_id": firstNonEmpty(p.PlanID, "free"), "credits": p.Credits}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "token": jwt, "access_token": jwt, "token_type": "Bearer", "user": profile})
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
	profile := map[string]any{"auth_subject": claims.Sub, "email": claims.Email, "role": "member", "plan_id": "free", "credits": 0}
	if h.DB != nil {
		p, err := h.upsertProfile(r.Context(), claims.Sub, claims.Email)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile_unavailable"})
			return
		}
		profile = map[string]any{"id": p.ID, "auth_subject": p.AuthSubject, "email": p.Email, "role": firstNonEmpty(p.Role, "member"), "plan_id": firstNonEmpty(p.PlanID, "free"), "credits": p.Credits}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": profile})
}

// AdminUsers requires the admin password (x-admin-password header),
// matching every other /api/admin/* endpoint. Previously this only
// checked for the presence of a "koschei_admin" cookie with no value
// validation, allowing anyone who set that cookie to read all user
// emails, roles, plans and credit balances.
func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	UsersHandler(w, r)
}

func (h *Handler) AdminUserAction(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	notImplemented(w, "AdminUserAction")
}
func (h *Handler) AdminSettings(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	notImplemented(w, "AdminSettings")
}
func (h *Handler) AIGenerate(w http.ResponseWriter, r *http.Request) { notImplemented(w, "AIGenerate") }
func (h *Handler) AIJobs(w http.ResponseWriter, r *http.Request)     { notImplemented(w, "AIJobs") }
