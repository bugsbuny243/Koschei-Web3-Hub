package handlers

import "net/http"

func notImplemented(w http.ResponseWriter, name string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "not_implemented", "handler": name})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) { notImplemented(w, "Register") }
func (h *Handler) Login(w http.ResponseWriter, r *http.Request)    { notImplemented(w, "Login") }
func (h *Handler) StartOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "StartOTPLogin")
}
func (h *Handler) VerifyOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "VerifyOTPLogin")
}
func (h *Handler) Me(w http.ResponseWriter, r *http.Request)         { notImplemented(w, "Me") }
func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) { UsersHandler(w, r) }
func (h *Handler) AdminUserAction(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "AdminUserAction")
}
func (h *Handler) AdminSettings(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "AdminSettings")
}
func (h *Handler) AIGenerate(w http.ResponseWriter, r *http.Request) { notImplemented(w, "AIGenerate") }
func (h *Handler) AIJobs(w http.ResponseWriter, r *http.Request)     { notImplemented(w, "AIJobs") }
