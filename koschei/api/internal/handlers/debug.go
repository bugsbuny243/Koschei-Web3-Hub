package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

func (h *Handler) DebugToken(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(strings.TrimSpace(auth), "Bearer ")
	if token == "" {
		writeJSON(w, 400, map[string]string{"error": "no token"})
		return
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		writeJSON(w, 400, map[string]string{"error": "invalid jwt format"})
		return
	}
	padded := parts[1]
	for len(padded)%4 != 0 {
		padded += "="
	}
	payload, err := base64.URLEncoding.DecodeString(padded)
	if err != nil {
		padded = parts[1]
		for len(padded)%4 != 0 {
			padded += "="
		}
		payload, err = base64.StdEncoding.DecodeString(padded)
	}
	var claims map[string]interface{}
	json.Unmarshal(payload, &claims)
	writeJSON(w, 200, map[string]interface{}{
		"jwt_claims":         claims,
		"neon_auth_issuer":   os.Getenv("NEON_AUTH_ISSUER"),
		"neon_auth_jwks_url": os.Getenv("NEON_AUTH_JWKS_URL"),
		"neon_auth_base_url": os.Getenv("EXPO_PUBLIC_NEON_AUTH_URL"),
	})
}
