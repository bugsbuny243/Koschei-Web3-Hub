package handlers

import (
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// RequireAuth is a middleware that checks for valid session token
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("koschei_session")
		if err != nil {
			http.Error(w, "Unauthorized: No session", http.StatusUnauthorized)
			return
		}

		tokenString := cookie.Value

		// Parse and validate token
		// TODO: Use proper JWKS validation with NEON_AUTH_JWKS_URL
		_, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("USER_SESSION_SECRET")), nil
		})

		if err != nil {
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		// Token is valid, continue
		next(w, r)
	}
}