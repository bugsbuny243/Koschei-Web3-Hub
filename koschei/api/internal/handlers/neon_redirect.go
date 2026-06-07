package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// NeonLogin redirects user to Neon Auth login page
func (h *Handler) NeonLogin(w http.ResponseWriter, r *http.Request) {
	baseURL := strings.TrimRight(os.Getenv("NEON_AUTH_BASE_URL"), "/")
	if baseURL == "" {
		http.Error(w, "NEON_AUTH_BASE_URL is not configured", http.StatusInternalServerError)
		return
	}

	// Get redirect target after login (default to hub)
	redirectTo := r.URL.Query().Get("redirect")
	if redirectTo == "" {
		redirectTo = "/hub.html"
	}

	// Build callback URL (where Neon will send the user back)
	callbackURL := fmt.Sprintf("%s://%s/api/auth/neon-callback", getScheme(r), r.Host)

	// Neon Auth login URL
	loginURL := fmt.Sprintf("%s/login?redirect_uri=%s&state=%s",
		baseURL,
		url.QueryEscape(callbackURL),
		url.QueryEscape(redirectTo),
	)

	http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)
}

// NeonRegister redirects user to Neon Auth register page
func (h *Handler) NeonRegister(w http.ResponseWriter, r *http.Request) {
	baseURL := strings.TrimRight(os.Getenv("NEON_AUTH_BASE_URL"), "/")
	if baseURL == "" {
		http.Error(w, "NEON_AUTH_BASE_URL is not configured", http.StatusInternalServerError)
		return
	}

	redirectTo := r.URL.Query().Get("redirect")
	if redirectTo == "" {
		redirectTo = "/hub.html"
	}

	callbackURL := fmt.Sprintf("%s://%s/api/auth/neon-callback", getScheme(r), r.Host)

	registerURL := fmt.Sprintf("%s/register?redirect_uri=%s&state=%s",
		baseURL,
		url.QueryEscape(callbackURL),
		url.QueryEscape(redirectTo),
	)

	http.Redirect(w, r, registerURL, http.StatusTemporaryRedirect)
}

// getScheme returns http or https
func getScheme(r *http.Request) string {
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}