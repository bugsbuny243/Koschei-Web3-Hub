package handlers

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const authContextKey contextKey = "auth_claims"

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(h, "Bearer ") {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		claims, err := parseAndVerifyNeonJWT(r.Context(), strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")))
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), authContextKey, claims))
		next(w, r)
	}
}

func userFromContext(ctx context.Context) (jwtClaims, bool) {
	v := ctx.Value(authContextKey)
	claims, ok := v.(jwtClaims)
	return claims, ok
}
