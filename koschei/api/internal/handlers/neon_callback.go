package handlers

import (
	"fmt"
	"net/http"
	"net/url"
)

// NeonCallback handles redirect back from Neon Auth
// It receives the token and redirects to frontend with the token in URL
func (h *Handler) NeonCallback(w http.ResponseWriter, r *http.Request) {
	// Neon usually sends token in fragment or query. We try both.
	token := r.URL.Query().Get("access_token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	// Get original redirect target from state param
	state := r.URL.Query().Get("state")
	if state == "" {
		state = "/hub.html"
	}

	if token != "" {
		// Redirect to frontend with token in hash so JS can read it
		redirectURL := fmt.Sprintf("%s#access_token=%s", state, url.QueryEscape(token))
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	// If no token, just redirect to the target page
	http.Redirect(w, r, state, http.StatusTemporaryRedirect)
}