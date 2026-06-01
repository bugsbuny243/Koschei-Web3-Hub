package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"koschei-bridge/services/auth-api/internal/auth"
	"koschei-bridge/services/auth-api/internal/db"
	"koschei-bridge/services/auth-api/internal/session"
)

type server struct {
	auth          *auth.Client
	db            *db.Store
	sessions      *session.Manager
	allowedOrigin string
}
type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func main() {
	store, err := db.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	authClient, err := auth.New(os.Getenv("NEON_AUTH_BASE_URL"), os.Getenv("NEON_AUTH_ISSUER"), os.Getenv("NEON_AUTH_JWKS_URL"))
	if err != nil {
		log.Fatal(err)
	}
	sessions, err := session.New(os.Getenv("USER_SESSION_SECRET"), os.Getenv("APP_ENV"))
	if err != nil {
		log.Fatal(err)
	}
	s := &server{auth: authClient, db: store, sessions: sessions, allowedOrigin: strings.TrimRight(strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGIN")), "/")}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /auth/signup", s.signup)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("GET /auth/me", s.me)
	mux.HandleFunc("POST /auth/logout", s.logout)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("auth-api listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, s.cors(mux)))
}
func (s *server) signup(w http.ResponseWriter, r *http.Request) { s.authenticate(w, r, "signup") }
func (s *server) login(w http.ResponseWriter, r *http.Request)  { s.authenticate(w, r, "login") }
func (s *server) authenticate(w http.ResponseWriter, r *http.Request, mode string) {
	var input credentials
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body.")
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !auth.IsEmail(input.Email) || len(input.Password) < 8 || len(input.Password) > 128 {
		if mode == "signup" {
			writeError(w, http.StatusBadRequest, "Enter a valid email and a password with at least 8 characters.")
		} else {
			writeError(w, http.StatusUnauthorized, "Invalid email or password.")
		}
		return
	}
	identity, err := s.auth.Authenticate(r.Context(), mode, input.Email, input.Password)
	if err != nil {
		var provider *auth.ProviderError
		if errors.As(err, &provider) {
			if mode == "signup" && (provider.Status == http.StatusConflict || provider.Status == http.StatusUnprocessableEntity) {
				writeError(w, http.StatusConflict, "Account already exists. Please sign in.")
				return
			}
			if provider.Status == http.StatusBadRequest || provider.Status == http.StatusUnauthorized || provider.Status == http.StatusForbidden || provider.Status == http.StatusUnprocessableEntity {
				writeError(w, http.StatusUnauthorized, "Invalid email or password.")
				return
			}
		}
		log.Printf("Neon Auth %s failed: %v", mode, err)
		writeError(w, http.StatusBadGateway, "Auth provider request failed.")
		return
	}
	if err := s.db.UpsertProfile(r.Context(), identity.Subject, identity.Email); err != nil {
		log.Printf("profile upsert failed: %v", err)
		writeError(w, http.StatusServiceUnavailable, "Could not create user profile.")
		return
	}
	if err := s.sessions.Set(w, identity.Subject, identity.Email); err != nil {
		log.Printf("session creation failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Could not create member session.")
		return
	}
	status := http.StatusOK
	if mode == "signup" {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]string{"email": identity.Email})
}
func (s *server) me(w http.ResponseWriter, r *http.Request) {
	value, err := s.sessions.Read(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"loggedIn": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"loggedIn": true, "email": value.Email})
}
func (s *server) logout(w http.ResponseWriter, _ *http.Request) {
	s.sessions.Clear(w)
	writeJSON(w, http.StatusOK, map[string]bool{"loggedIn": false})
}
func (s *server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		if s.allowedOrigin != "" && origin == s.allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("response encoding failed: %v", err)
	}
}
