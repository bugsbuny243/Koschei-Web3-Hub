package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// NeonCallback handles the callback from Neon Auth
func NeonCallback(w http.ResponseWriter, r *http.Request) {
	// Get token from query or form
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.FormValue("token")
	}

	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	// Validate token using JWKS (simplified for now)
	// In production, use proper JWKS validation with NEON_AUTH_JWKS_URL
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		// TODO: Fetch public key from NEON_AUTH_JWKS_URL
		return []byte(os.Getenv("USER_SESSION_SECRET")), nil
	})

	if err != nil || !parsedToken.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// TODO: Extract user info from claims
	// For now, just set a simple session cookie

	http.SetCookie(w, &http.Cookie{
		Name:     "koschei_session",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		Path:     "/",
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production",
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Logged in successfully",
	})
}

// NeonLogin redirects to Neon Auth login page
func NeonLogin(w http.ResponseWriter, r *http.Request) {
	baseURL := os.Getenv("NEON_AUTH_BASE_URL")
	if baseURL == "" {
		http.Error(w, "NEON_AUTH_BASE_URL not configured", http.StatusInternalServerError)
		return
	}

	callbackURL := "https://" + r.Host + "/api/auth/neon-callback"
	redirectURL := fmt.Sprintf("%s/login?callback_url=%s", baseURL, callbackURL)

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// NeonRegister redirects to Neon Auth register page
func NeonRegister(w http.ResponseWriter, r *http.Request) {
	baseURL := os.Getenv("NEON_AUTH_BASE_URL")
	if baseURL == "" {
		http.Error(w, "NEON_AUTH_BASE_URL not configured", http.StatusInternalServerError)
		return
	}

	callbackURL := "https://" + r.Host + "/api/auth/neon-callback"
	redirectURL := fmt.Sprintf("%s/register?callback_url=%s", baseURL, callbackURL)

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}