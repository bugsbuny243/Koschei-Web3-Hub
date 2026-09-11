package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

// localJWTSecret remains only for compatibility with already-issued
// koschei-local JWTs. Local username/password registration and login are no
// longer part of the routed product surface.
func localJWTSecret() []byte {
	for _, key := range []string{"USER_SESSION_SECRET", "JWT_SECRET", "NEON_AUTH_STATE_SECRET", "OWNER_SECRET", "ADMIN_PASSWORD"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return []byte(value)
		}
	}
	return nil
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
	kid, ok := header["kid"].(string)
	if !ok || strings.TrimSpace(kid) != "koschei-local" {
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
