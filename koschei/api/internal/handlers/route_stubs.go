package handlers

import (
	"net/http"
)

func notImplemented(w http.ResponseWriter, name string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "not_implemented", "handler": name})
}

// Custom email/password registration is disabled.
// Users register via Neon Auth directly from the frontend (koschei-auth.js).
// This endpoint is kept for backward compatibility but does nothing,
// preventing unauthenticated writes to app_user_profiles without a real
// Neon Auth identity (auth_subject) or password storage.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error":   "custom_auth_disabled",
		"message": "Use Neon Auth email sign-up from the frontend.",
	})
}

// Şimdilik diğer rotalar kapalı kalmaya devam ediyor, sırayla açarız
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error":   "custom_auth_disabled",
		"message": "Use Neon Auth email sign-in from the frontend.",
	})
}
func (h *Handler) StartOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "StartOTPLogin")
}
func (h *Handler) VerifyOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "VerifyOTPLogin")
}
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) { notImplemented(w, "Me") }

// AdminUsers requires the admin password (x-admin-password header),
// matching every other /api/admin/* endpoint. Previously this only
// checked for the presence of a "koschei_admin" cookie with no value
// validation, allowing anyone who set that cookie to read all user
// emails, roles, plans and credit balances.
func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	UsersHandler(w, r)
}

func (h *Handler) AdminUserAction(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	notImplemented(w, "AdminUserAction")
}
func (h *Handler) AdminSettings(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	notImplemented(w, "AdminSettings")
}
func (h *Handler) AIGenerate(w http.ResponseWriter, r *http.Request) { notImplemented(w, "AIGenerate") }
func (h *Handler) AIJobs(w http.ResponseWriter, r *http.Request)     { notImplemented(w, "AIJobs") }
