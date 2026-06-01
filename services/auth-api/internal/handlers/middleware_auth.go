package handlers

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

	"koschei-bridge/services/auth-api/internal/db"
)

const memberCookieName = "koschei_member_session"
const memberSessionTTL = 7 * 24 * time.Hour

type memberSession struct {
	Sub       string `json:"sub"`
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expiresAt"`
}

func (handler *Handler) Me(w http.ResponseWriter, r *http.Request) {
	member, err := handler.readMemberCookie(r)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"loggedIn": true, "sub": member.Sub, "email": member.Email})
}
func (handler *Handler) Logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: memberCookieName, Value: "", Path: "/", HttpOnly: true, Secure: handler.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	WriteJSON(w, http.StatusOK, map[string]any{"loggedIn": false})
}
func (handler *Handler) setMemberCookie(w http.ResponseWriter, member db.Profile) {
	session := memberSession{Sub: member.Subject, Email: member.Email, ExpiresAt: time.Now().Add(memberSessionTTL).UnixMilli()}
	payload, _ := json.Marshal(session)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	http.SetCookie(w, &http.Cookie{Name: memberCookieName, Value: encoded + "." + handler.sign(encoded), Path: "/", HttpOnly: true, Secure: handler.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(memberSessionTTL.Seconds())})
}
func (handler *Handler) readMemberCookie(r *http.Request) (memberSession, error) {
	cookie, err := r.Cookie(memberCookieName)
	if err != nil {
		return memberSession{}, err
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(handler.sign(parts[0])), []byte(parts[1])) {
		return memberSession{}, errors.New("invalid cookie signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return memberSession{}, err
	}
	var session memberSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return memberSession{}, err
	}
	if session.Sub == "" || !strings.Contains(session.Email, "@") || session.ExpiresAt <= time.Now().UnixMilli() {
		return memberSession{}, errors.New("member session is invalid")
	}
	return session, nil
}
func (handler *Handler) sign(value string) string {
	digest := hmac.New(sha256.New, handler.secret)
	digest.Write([]byte(value))
	return hex.EncodeToString(digest.Sum(nil))
}
