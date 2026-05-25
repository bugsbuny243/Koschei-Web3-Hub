package http

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"koschei/api/internal/handlers"
)

func NewServer(db *sql.DB, dbInitError string, adminPassword string, corsOrigin string, staticDir string) http.Handler {
	h := &handlers.Handler{DB: db, DBInitError: dbInitError, AdminPassword: adminPassword, Limiter: handlers.NewLimiter()}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/version", method("GET", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"app":    "koschei",
			"status": "ok",
			"build":  "phase3-db-final-010",
		})
	}))
	mux.HandleFunc("/api/auth/register", requiresDB(h, method("POST", h.Register)))
	mux.HandleFunc("/api/auth/login", requiresDB(h, method("POST", h.Login)))
	mux.HandleFunc("/api/me", requiresDB(h, handlers.RequireAuth(method("GET", h.Me))))
	mux.HandleFunc("/api/plans", requiresDB(h, method("GET", h.Plans)))
	mux.HandleFunc("/api/billing/manual-payment-request", requiresDB(h, method("POST", h.ManualPaymentRequest)))
	mux.HandleFunc("/api/credits/me", requiresDB(h, handlers.RequireAuth(method("GET", h.Credits))))
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if !h.RequireDB(w) {
			return
		}
		if !h.OwnerAuth(w, r) {
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
	mux.HandleFunc("/api/runtime/projects", requiresDB(h, handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.CreateRuntimeProject(w, r)
			return
		}
		if r.Method == http.MethodGet {
			h.ListRuntimeProjects(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	mux.HandleFunc("/api/runtime/projects/", requiresDB(h, handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/artifacts") {
			h.RuntimeArtifactsRoute(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/artifacts/generate") {
			h.RuntimeArtifactsRoute(w, r)
			return
		}
		if r.Method == http.MethodGet {
			h.GetRuntimeProject(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	mux.HandleFunc("/api/runtime/tasks", requiresDB(h, handlers.RequireAuth(method("GET", h.ListRuntimeTasks))))
	mux.HandleFunc("/api/runtime/tasks/", requiresDB(h, handlers.RequireAuth(method("GET", h.GetRuntimeTask))))
	mux.HandleFunc("/api/runtime/logs/", requiresDB(h, handlers.RequireAuth(method("GET", h.GetRuntimeLogs))))
	mux.HandleFunc("/api/runtime/route", requiresDB(h, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if !h.OwnerAuth(w, r) {
			return
		}
		h.RuntimeRoute(w, r)
	}))
	mux.HandleFunc("/api/ai/generate", requiresDB(h, handlers.RequireAuth(method("POST", h.AIGenerate))))
	mux.HandleFunc("/api/ai/jobs", requiresDB(h, handlers.RequireAuth(method("GET", h.AIJobs))))
	mux.HandleFunc("/api/ai/image", requiresDB(h, handlers.RequireAuth(method("POST", h.AIImageGenerate))))
	mux.HandleFunc("/api/ai/audio", requiresDB(h, handlers.RequireAuth(method("POST", h.AIAudioGenerate))))
	mux.HandleFunc("/api/artifacts/", requiresDB(h, handlers.RequireAuth(h.ArtifactRoute)))
	mux.HandleFunc("/api/owner/payment-requests", requiresDB(h, method("GET", h.OwnerPaymentRequests)))
	mux.HandleFunc("/api/owner/activate-plan", requiresDB(h, method("POST", h.OwnerActivatePlan)))
	mux.HandleFunc("/api/owner/grant-credits", requiresDB(h, method("POST", h.OwnerGrantCredits)))
	mux.HandleFunc("/api/owner/db-health", requiresDB(h, method("GET", h.OwnerDBHealth)))
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

	return securityHeaders(cors(apiReadiness(db, mux), corsOrigin))
}

func apiReadiness(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/version" && db == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "database unavailable", "details": "database connection is not initialized"})
			return
		}
		next.ServeHTTP(w, r)
	})
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-admin-password, Authorization")
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
