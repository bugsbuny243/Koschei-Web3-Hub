package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type authUser struct {
	ID          string `json:"id"`
	AuthSubject string `json:"auth_subject,omitempty"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Plan        string `json:"plan"`
	Credits     int    `json:"credits"`
}

type emailPasswordLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type betterAuthConfig struct {
	BaseURL   string
	IssuerURL string
	JWKSURL   string
}

type authProviderTransport interface {
	Do(*http.Request) (*http.Response, error)
}

var authProviderHTTPClient authProviderTransport = &http.Client{Timeout: 10 * time.Second}

func (h *Handler) Register(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error":   "disabled",
		"message": "Use direct Neon Auth /sign-up/email from the frontend.",
	})
}

func (h *Handler) Login(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error":   "disabled",
		"message": "Use direct Neon Auth /sign-in/email from the frontend.",
	})
}

func validatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("Email and password are required.")
	}
	if len(password) < 8 {
		return errors.New("Password must be at least 8 characters.")
	}
	return nil
}

func defaultUserName(email string) string {
	name := strings.TrimSpace(strings.Split(email, "@")[0])
	if name == "" {
		return "User"
	}
	return name
}

func absoluteCallbackURL(r *http.Request, path string) string {
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if path == "/" {
		path = "/hub"
	}
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "https"
		if r.TLS != nil {
			scheme = "https"
		} else if r.URL != nil && r.URL.Scheme != "" {
			scheme = r.URL.Scheme
		}
	}
	if strings.Contains(scheme, ",") {
		scheme = strings.TrimSpace(strings.Split(scheme, ",")[0])
	}
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if strings.Contains(host, ",") {
		host = strings.TrimSpace(strings.Split(host, ",")[0])
	}
	return scheme + "://" + host + path
}