package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Neon Auth endpoint'leri
func (h *Handler) NeonSignUp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	neonURL := strings.TrimSpace(os.Getenv("NEON_AUTH_BASE_URL"))
	if neonURL == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "neon_auth_not_configured"})
		return
	}

	// Neon Auth'a istek at
	payload := map[string]any{
		"email":    req.Email,
		"password": req.Password,
		"name":     req.Name,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(neonURL+"/sign-up/email", "application/json", bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "neon_auth_request_failed"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "neon_auth_error", "detail": string(respBody)})
		return
	}

	// Neon'dan dönen JWT'yi al
	var neonResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(respBody, &neonResp); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "neon_response_parse_failed"})
		return
	}

	// JWT'yi doğrula
	claims, err := parseAndVerifyNeonJWT(neonResp.AccessToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_jwt"})
		return
	}

	// Kullanıcıyı veritabanına kaydet (idempotent)
	_, _ = h.DB.Exec(`
		INSERT INTO app_user_profiles (auth_subject, email, role, plan_id, credits)
		VALUES ($1, $2, 'user', 'free', 10)
		ON CONFLICT (email) DO NOTHING
	`, claims.Sub, strings.ToLower(claims.Email))

	_, _ = h.DB.Exec(`
		INSERT INTO entitlements (email, plan_id, outputs_total, outputs_remaining, status)
		VALUES ($1, 'free', 10, 10, 'active')
		ON CONFLICT DO NOTHING
	`, strings.ToLower(claims.Email))

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"access_token": neonResp.AccessToken,
		"token_type":   "Bearer",
		"email":        claims.Email,
	})
}

func (h *Handler) NeonSignIn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	neonURL := strings.TrimSpace(os.Getenv("NEON_AUTH_BASE_URL"))
	if neonURL == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "neon_auth_not_configured"})
		return
	}

	payload := map[string]any{
		"email":    req.Email,
		"password": req.Password,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(neonURL+"/sign-in/email", "application/json", bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "neon_auth_request_failed"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_credentials"})
		return
	}

	var neonResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(respBody, &neonResp); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "neon_response_parse_failed"})
		return
	}

	claims, err := parseAndVerifyNeonJWT(neonResp.AccessToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_jwt"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"access_token": neonResp.AccessToken,
		"token_type":   "Bearer",
		"email":        claims.Email,
	})
}
