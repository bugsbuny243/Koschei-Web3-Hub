package http

import (
	"database/sql"
	"net/http"
	"strings"

	"koschei/api/internal/handlers"
)

func NewServer(db *sql.DB, adminPassword string, corsOrigin string) http.Handler {
	h := &handlers.Handler{DB: db, AdminPassword: adminPassword}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/plans", method("GET", h.Plans))
	mux.HandleFunc("/api/billing/manual-payment-request", method("POST", h.ManualPaymentRequest))
	mux.HandleFunc("/api/credits", method("GET", h.Credits))
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet { h.GetJobs(w, r); return }
		if r.Method == http.MethodPost { h.CreateJob(w, r); return }
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/runtime/route", method("POST", h.RuntimeRoute))
	mux.HandleFunc("/api/owner/payment-requests", method("GET", h.OwnerPaymentRequests))
	mux.HandleFunc("/api/owner/activate-plan", method("POST", h.OwnerActivatePlan))
	mux.HandleFunc("/api/owner/grant-credits", method("POST", h.OwnerGrantCredits))
	mux.HandleFunc("/api/owner/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/status") { h.OwnerUpdateJobStatus(w, r); return }
		http.NotFound(w, r)
	})
	return cors(mux, corsOrigin)
}

func method(m string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != m { w.WriteHeader(http.StatusMethodNotAllowed); return }
		next(w, r)
	}
}

func cors(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" { w.Header().Set("Access-Control-Allow-Origin", origin) }
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-admin-password")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next.ServeHTTP(w, r)
	})
}
