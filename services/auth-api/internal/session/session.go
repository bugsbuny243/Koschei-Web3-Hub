package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const CookieName = "koschei_member_session"
const duration = 7 * 24 * time.Hour

type Session struct {
	Subject   string `json:"sub"`
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expiresAt"`
}

type Manager struct {
	secret []byte
	secure bool
}

func New(secret, appEnv string) (*Manager, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("USER_SESSION_SECRET is not configured")
	}
	return &Manager{secret: []byte(secret), secure: strings.EqualFold(strings.TrimSpace(appEnv), "production")}, nil
}

func (m *Manager) Set(w http.ResponseWriter, subject, email string) error {
	payload, err := json.Marshal(Session{Subject: subject, Email: email, ExpiresAt: time.Now().Add(duration).UnixMilli()})
	if err != nil {
		return err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: encoded + "." + m.sign(encoded), Path: "/", MaxAge: int(duration.Seconds()), HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode})
	return nil
}

func (m *Manager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode})
}

func (m *Manager) Read(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(m.sign(parts[0])), []byte(parts[1])) {
		return nil, errors.New("invalid session signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var value Session
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, err
	}
	if strings.TrimSpace(value.Subject) == "" || strings.TrimSpace(value.Email) == "" || value.ExpiresAt <= time.Now().UnixMilli() {
		return nil, errors.New("expired or incomplete session")
	}
	return &value, nil
}

func (m *Manager) sign(value string) string {
	digest := hmac.New(sha256.New, m.secret)
	_, _ = digest.Write([]byte(value))
	return hex.EncodeToString(digest.Sum(nil))
}
