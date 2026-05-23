package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type authReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if err := decodeJSON(r, &req); err != nil || !validEmail(req.Email) || len(req.Password) < 8 {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	hash := hashPassword(req.Password)
	_, err := h.DB.Exec(`INSERT INTO auth_accounts (email,password_hash,plan) VALUES ($1,$2,'free')`, strings.ToLower(strings.TrimSpace(req.Email)), hash)
	if err != nil {
		writeJSON(w, 409, map[string]string{"error": "account exists"})
		return
	}
	token, err := issueJWT(strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "token failed"})
		return
	}
	writeJSON(w, 201, map[string]any{"token": token, "email": req.Email})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if err := decodeJSON(r, &req); err != nil || !validEmail(req.Email) || req.Password == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	var hash string
	err := h.DB.QueryRow(`SELECT password_hash FROM auth_accounts WHERE email=$1`, strings.ToLower(strings.TrimSpace(req.Email))).Scan(&hash)
	if err == sql.ErrNoRows {
		writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	if hash != hashPassword(req.Password) {
		writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
		return
	}
	token, err := issueJWT(strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "token failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"token": token, "email": req.Email})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	email, ok := emailFromAuthHeader(r)
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	var plan string
	_ = h.DB.QueryRow(`SELECT COALESCE(plan,'free') FROM auth_accounts WHERE email=$1`, email).Scan(&plan)
	var credits int
	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM credits_ledger WHERE email=$1`, email).Scan(&credits)
	writeJSON(w, 200, map[string]any{"email": email, "plan": plan, "credits": credits})
}

func issueJWT(email string) (string, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	}
	if secret == "" {
		return "", fmt.Errorf("missing secret")
	}
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadObj := map[string]any{"sub": email, "exp": time.Now().Add(7 * 24 * time.Hour).Unix(), "iat": time.Now().Unix()}
	pb, _ := json.Marshal(payloadObj)
	payload := base64.RawURLEncoding.EncodeToString(pb)
	signing := head + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signing + "." + sig, nil
}

func emailFromAuthHeader(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(h, "Bearer "), ".")
	if len(parts) != 3 {
		return "", false
	}
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	}
	if secret == "" {
		return "", false
	}
	signing := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return "", false
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	var claims map[string]any
	if json.Unmarshal(pb, &claims) != nil {
		return "", false
	}
	exp, ok := claims["exp"].(float64)
	if !ok || int64(exp) < time.Now().Unix() {
		return "", false
	}
	sub, ok := claims["sub"].(string)
	if !ok || !validEmail(sub) {
		return "", false
	}
	return sub, true
}

func hashPassword(pw string) string {
	s := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if s == "" {
		s = "koschei-phase3"
	}
	mac := hmac.New(sha256.New, []byte(s))
	mac.Write([]byte(pw))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
