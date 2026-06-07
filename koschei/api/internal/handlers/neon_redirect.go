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

	redirectTo := r.URL.Query().Get("redirect")
	if redirectTo == "" {
		redirectTo = "/hub.html"
	}

	callbackURL := getAbsoluteCallbackURL(r)

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

	callbackURL := getAbsoluteCallbackURL(r)

	registerURL := fmt.Sprintf("%s/register?redirect_uri=%s&state=%s",
		baseURL,
		url.QueryEscape(callbackURL),
		url.QueryEscape(redirectTo),
	)

	http.Redirect(w, r, registerURL, http.StatusTemporaryRedirect)
}

// getAbsoluteCallbackURL builds a reliable full URL using proxy headers
func getAbsoluteCallbackURL(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	return fmt.Sprintf("%s://%s/api/auth/neon-callback", proto, host)
}
