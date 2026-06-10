package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bugsbuny243/Koschei-Web3-Hub/koschei/api/pkg/audit"
	"github.com/bugsbuny243/Koschei-Web3-Hub/koschei/api/pkg/utils"
)

var (
	adminEmail    string
	adminPassword string
	adminHash     string
	dbConn        *sql.DB
)

// InitAdminHandler initializes the admin handler with DB and config
func InitAdminHandler(database *sql.DB, email, password string) {
	dbConn = database
	adminEmail = email
	adminPassword = password

	// If not already hashed, hash it once on startup
	if !utils.IsArgon2Hash(adminPassword) {
		hash, _ := utils.HashPassword(adminPassword)
		adminHash = hash
	} else {
		adminHash = adminPassword
	}
}

// AdminLogin handles admin login request
func AdminLogin(w http.ResponseWriter, r *http.Request) {
	type RequestBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var req RequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Geçersiz istek", http.StatusBadRequest)
		return
	}

	// Rate limiting (simple in-memory version)	clientIP := getClientIP(r)
	if isBlocked(clientIP) {
		time.Sleep(3 * time.Second) // Slow down brute force
		http.Error(w, "Çok fazla deneme", http.StatusTooManyRequests)
		return
	}

	// Validate credentials
	if req.Email == adminEmail && utils.ComparePassword(adminHash, req.Password) {
		// Log success
		audit.Log(r.Context(), audit.Event{
			Action:    "ADMIN_LOGIN_SUCCESS",
			Email:     req.Email,
			IP:        clientIP,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"success": true,
			},
		})

		// Set secure session cookie
		setSessionCookie(w)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	// Log failure
	audit.Log(r.Context(), audit.Event{
		Action:    "ADMIN_LOGIN_FAILED",
		Email:     req.Email,
		IP:        clientIP,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"success": false,
		},
	})

	blockClient(clientIP)

	http.Error(w, "Yanlış e-posta veya şifre", http.StatusUnauthorized)
}

func setSessionCookie(w http.ResponseWriter) {
	expire := time.Now().Add(24 * time.Hour)
	sessionID := generateSecureToken()
	cookie := &http.Cookie{
		Name:     "koschei_admin",
		Value:    sessionID,		Expires:  expire,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

func generateSecureToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Simple rate limiter (in prod, use Redis)
var blockedIPs = make(map[string]time.Time)

func isBlocked(ip string) bool {
	if t, found := blockedIPs[ip]; found {
		if time.Since(t) < 5*time.Minute {
			return true
		}
		delete(blockedIPs, ip)
	}
	return false
}

func blockClient(ip string) {
	blockedIPs[ip] = time.Now()
}
