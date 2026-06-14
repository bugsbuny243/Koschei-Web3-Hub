package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

type localAuthInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (h *Handler) LocalRegister(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	if err := ensureLocalAuthSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "auth schema unavailable", "message": err.Error()})
		return
	}
	var req localAuthInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	if !strings.Contains(email, "@") || len(password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid credentials"})
		return
	}
	sub := "local:" + email
	salt, hash, err := hashLocalPassword(password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "password hashing failed"})
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `
		INSERT INTO local_auth_users (email, auth_subject, password_salt, password_hash, created_at, updated_at)
		VALUES (lower($1), $2, $3, $4, now(), now())
		ON CONFLICT (email) DO UPDATE
		SET password_salt = EXCLUDED.password_salt, password_hash = EXCLUDED.password_hash, auth_subject = EXCLUDED.auth_subject, updated_at = now()`, email, sub, salt, hash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "auth user save failed", "message": err.Error()})
		return
	}
	_, _ = h.upsertProfile(r.Context(), sub, email)
	token, err := signLocalJWT(sub, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token signing failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "token": token, "access_token": token, "jwt": token, "user": map[string]string{"email": email, "auth_subject": sub}})
}

func (h *Handler) LocalLogin(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	if err := ensureLocalAuthSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "auth schema unavailable", "message": err.Error()})
		return
	}
	var req localAuthInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	var sub, salt, storedHash string
	err := h.DB.QueryRowContext(r.Context(), `SELECT auth_subject, password_salt, password_hash FROM local_auth_users WHERE lower(email)=lower($1)`, email).Scan(&sub, &salt, &storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "auth lookup failed", "message": err.Error()})
		return
	}
	if !verifyLocalPassword(password, salt, storedHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if strings.TrimSpace(sub) == "" {
		sub = "local:" + email
	}
	_, _ = h.upsertProfile(r.Context(), sub, email)
	token, err := signLocalJWT(sub, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token signing failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "token": token, "access_token": token, "jwt": token, "user": map[string]string{"email": email, "auth_subject": sub}})
}

func ensureLocalAuthSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS local_auth_users (
			email text PRIMARY KEY,
			auth_subject text NOT NULL,
			password_salt text NOT NULL,
			password_hash text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`)
	return err
}

func hashLocalPassword(password string) (string, string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	salt := hex.EncodeToString(raw)
	return salt, localPasswordDigest(password, salt), nil
}

func verifyLocalPassword(password, salt, stored string) bool {
	expected := localPasswordDigest(password, salt)
	return hmac.Equal([]byte(expected), []byte(stored))
}

func localPasswordDigest(password, salt string) string {
	data := []byte(salt + ":" + password)
	for i := 0; i < 120000; i++ {
		sum := sha256.Sum256(data)
		data = sum[:]
	}
	return hex.EncodeToString(data)
}

func localJWTSecret() []byte {
	for _, key := range []string{"USER_SESSION_SECRET", "JWT_SECRET", "NEON_AUTH_STATE_SECRET", "OWNER_SECRET", "ADMIN_PASSWORD"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return []byte(value)
		}
	}
	return nil
}

func signLocalJWT(sub, email string) (string, error) {
	secret := localJWTSecret()
	if len(secret) == 0 {
		return "", errors.New("local auth secret missing")
	}
	header := map[string]string{"alg": "HS256", "typ": "JWT", "kid": "koschei-local"}
	now := time.Now().Unix()
	payload := map[string]any{"sub": sub, "email": email, "iss": "koschei-local", "exp": now + 60*60*24*30, "iat": now}
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(unsigned))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}

func tryLocalJWT(token string) (neonJWTClaims, bool, error) {
	var claims neonJWTClaims
	if !tokenLooksLikeJWT(token) {
		return claims, false, nil
	}
	parts := strings.Split(token, ".")
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, false, nil
	}
	var header map[string]any
	if json.Unmarshal(headerRaw, &header) != nil {
		return claims, false, nil
	}
	if strings.TrimSpace(header["kid"].(string)) != "koschei-local" {
		return claims, false, nil
	}
	secret := localJWTSecret()
	if len(secret) == 0 {
		return claims, true, errors.New("local auth secret missing")
	}
	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(unsigned))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(expected, actual) {
		return claims, true, errors.New("invalid local token signature")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, true, err
	}
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return claims, true, err
	}
	if claims.Exp < time.Now().Unix() {
		return claims, true, errors.New("expired local token")
	}
	if strings.TrimSpace(claims.Email) == "" || strings.TrimSpace(claims.Sub) == "" {
		return claims, true, errors.New("invalid local token claims")
	}
	return claims, true, nil
}
