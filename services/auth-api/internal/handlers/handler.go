package handlers

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei-bridge/services/auth-api/internal/db"
)

type Handler struct {
	client      *http.Client
	authBaseURL string
	jwksURL     string
	issuer      string
	database    *db.Client
	secret      []byte
	secure      bool
}

func New() (*Handler, error) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("NEON_AUTH_BASE_URL")), "/")
	jwksURL := strings.TrimSpace(os.Getenv("NEON_AUTH_JWKS_URL"))
	secret := strings.TrimSpace(os.Getenv("USER_SESSION_SECRET"))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if base == "" || jwksURL == "" || secret == "" || databaseURL == "" {
		return nil, errors.New("NEON_AUTH_BASE_URL, NEON_AUTH_JWKS_URL, DATABASE_URL, and USER_SESSION_SECRET are required")
	}
	issuer, err := normalizedIssuer(strings.TrimSpace(os.Getenv("NEON_AUTH_ISSUER")), base, jwksURL)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 12 * time.Second}
	database, err := db.New(databaseURL, client)
	if err != nil {
		return nil, err
	}
	return &Handler{client: client, authBaseURL: base, jwksURL: jwksURL, issuer: issuer, database: database, secret: []byte(secret), secure: envOr("APP_ENV", "development") == "production"}, nil
}
func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
