package http

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"koschei/api/internal/handlers"
)

func NewServer(db *sql.DB, adminPassword string, corsOrigin string, staticDir string) http.Handler {
	h := &handlers.Handler{DB: db, AdminPassword: adminPassword, Limiter: handlers.NewLimiter()}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/plans", requiresDB(h, method("GET", h.Plans)))
	mux.HandleFunc("/api/billing/manual-payment-request", requiresDB(h, method("POST", h.ManualPaymentRequest)))
	mux.HandleFunc("/api/credits", requiresDB(h, method("GET", h.Credits)))
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if !h.RequireDB(w) {
			return
		}
		if r.Method == http.MethodGet {
			h.GetJobs(w, r)
			return
		}
		if r.Method == http.MethodPost {
			h.CreateJob(w, r)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/runtime/projects", func(w http.ResponseWriter, r *http.Request) {
		if !h.RequireDB(w) {
			return
		}
		if r.Method == http.MethodPost {
			h.CreateRuntimeProject(w, r)
			return
		}
		if r.Method == http.MethodGet {
			h.ListRuntimeProjects(w, r)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/runtime/projects/", requiresDB(h, method("GET", h.GetRuntimeProject)))
	mux.HandleFunc("/api/runtime/tasks", requiresDB(h, method("GET", h.ListRuntimeTasks)))
	mux.HandleFunc("/api/runtime/tasks/", requiresDB(h, method("GET", h.GetRuntimeTask)))
	mux.HandleFunc("/api/runtime/logs/", requiresDB(h, method("GET", h.GetRuntimeLogs)))
	mux.HandleFunc("/api/runtime/route", requiresDB(h, method("POST", h.RuntimeRoute)))
	mux.HandleFunc("/api/owner/payment-requests", requiresDB(h, method("GET", h.OwnerPaymentRequests)))
	mux.HandleFunc("/api/owner/activate-plan", requiresDB(h, method("POST", h.OwnerActivatePlan)))
	mux.HandleFunc("/api/owner/grant-credits", requiresDB(h, method("POST", h.OwnerGrantCredits)))
	mux.HandleFunc("/api/owner/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if !h.RequireDB(w) {
			return
		}
		if r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/status") {
			h.OwnerUpdateJobStatus(w, r)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/owner/runtime/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if !h.RequireDB(w) {
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/retry") {
			h.OwnerRetryRuntimeTask(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel") {
			h.OwnerCancelRuntimeTask(w, r)
			return
		}
		if r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/status") {
			h.OwnerUpdateRuntimeTaskStatus(w, r)
			return
		}
		http.NotFound(w, r)
	})

	if staticDir != "" {
		if info, err := os.Stat(staticDir); err != nil || !info.IsDir() {
			log.Printf("warning: static directory unavailable at %q: %v", staticDir, err)
		} else {
			static := http.FileServer(http.Dir(staticDir))
			indexPath := filepath.Join(staticDir, "index.html")
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					http.NotFound(w, r)
					return
				}
				if r.Method != http.MethodGet && r.Method != http.MethodHead {
					http.NotFound(w, r)
					return
				}
				candidate := filepath.Join(staticDir, strings.TrimPrefix(filepath.Clean(r.URL.Path), "/"))
				if fileInfo, err := os.Stat(candidate); err == nil && !fileInfo.IsDir() {
					static.ServeHTTP(w, r)
					return
				}
				http.ServeFile(w, r, indexPath)
			})
		}
	} else {
		log.Printf("warning: STATIC_DIR is empty; frontend static files not enabled")
	}

	return securityHeaders(cors(mux, corsOrigin))
}

func requiresDB(h *handlers.Handler, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.RequireDB(w) {
			return
		}
		next(w, r)
	}
}

func method(m string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != m {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}
func cors(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-admin-password")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
