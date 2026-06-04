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
	mux.HandleFunc("/api/config", method("GET", h.Config))
	mux.HandleFunc("/api/debug/token", method("GET", h.DebugToken))
	mux.HandleFunc("/api/auth/provision", method("POST", h.Provision))
	mux.HandleFunc("/api/web3/health", method("GET", h.Web3Health))
	mux.HandleFunc("/api/web3/health/logs", requiresDB(h, handlers.RequireAuth(method("GET", h.Web3HealthLogs))))
	mux.HandleFunc("/api/analytics/event", method("POST", h.AnalyticsEvent))
	mux.HandleFunc("/api/version", method("GET", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"app": "koschei-engine", "status": "ok"})
	}))
	mux.HandleFunc("/api/auth/register", requiresDB(h, method("POST", h.Register)))
	mux.HandleFunc("/api/auth/login", requiresDB(h, method("POST", h.Login)))
	mux.HandleFunc("/api/auth/otp/start", method("POST", h.StartOTPLogin))
	mux.HandleFunc("/api/auth/otp/verify", method("POST", h.VerifyOTPLogin))
	mux.HandleFunc("/api/me", requiresDB(h, handlers.RequireAuth(method("GET", h.Me))))
	mux.HandleFunc("/api/member/summary", requiresDB(h, handlers.RequireAuth(method("GET", h.MemberSummary))))
	mux.HandleFunc("/api/payments/request", requiresDB(h, handlers.RequireAuth(method("POST", h.PaymentRequest))))
	mux.HandleFunc("/api/admin/payment-requests", requiresDB(h, method("GET", h.AdminPaymentRequests)))
	mux.HandleFunc("/api/admin/analytics/events", requiresDB(h, method("GET", h.AdminAnalyticsEvents)))
	mux.HandleFunc("/api/admin/grant-radar", requiresDB(h, h.GrantRadar))
	mux.HandleFunc("/api/admin/proof-of-impact", requiresDB(h, method("GET", h.ProofOfImpact)))
	mux.HandleFunc("/api/admin/payment-requests/approve", requiresDB(h, method("POST", h.ApprovePaymentRequest)))
	mux.HandleFunc("/api/admin/payment-requests/reject", requiresDB(h, method("POST", h.RejectPaymentRequest)))
	mux.HandleFunc("/api/admin/summary", requiresDB(h, method("GET", h.AdminSummary)))
	mux.HandleFunc("/api/admin/users", requiresDB(h, method("GET", func(w http.ResponseWriter, r *http.Request) { h.AdminTable(w, r, "users") })))
	mux.HandleFunc("/api/admin/payments", requiresDB(h, method("GET", func(w http.ResponseWriter, r *http.Request) { h.AdminTable(w, r, "payments") })))
	mux.HandleFunc("/api/admin/entitlements", requiresDB(h, method("GET", func(w http.ResponseWriter, r *http.Request) { h.AdminTable(w, r, "entitlements") })))
	mux.HandleFunc("/api/admin/outputs", requiresDB(h, method("GET", func(w http.ResponseWriter, r *http.Request) { h.AdminTable(w, r, "outputs") })))
	mux.HandleFunc("/api/admin/watchlist-sources", requiresDB(h, method("GET", func(w http.ResponseWriter, r *http.Request) { h.AdminTable(w, r, "watchlist-sources") })))
	mux.HandleFunc("/api/admin/web3-events", requiresDB(h, method("GET", func(w http.ResponseWriter, r *http.Request) { h.AdminTable(w, r, "web3-events") })))
	mux.HandleFunc("/api/admin/chain-health", requiresDB(h, method("GET", func(w http.ResponseWriter, r *http.Request) { h.AdminTable(w, r, "chain-health") })))
	mux.HandleFunc("/api/admin/analytics", requiresDB(h, method("GET", func(w http.ResponseWriter, r *http.Request) { h.AdminTable(w, r, "analytics") })))
	mux.HandleFunc("/api/admin/system-scan", requiresDB(h, method("GET", h.AdminSystemScan)))
	mux.HandleFunc("/api/admin/chat", requiresDB(h, method("POST", h.AdminChat)))
	mux.HandleFunc("/api/admin/modules", requiresDB(h, method("GET", h.AdminModules)))
	mux.HandleFunc("/api/admin/grant-autopilot", requiresDB(h, method("GET", h.GrantAutopilot)))
	mux.HandleFunc("/api/admin/grant-autopilot/generate", requiresDB(h, method("POST", h.GrantGenerate)))
	mux.HandleFunc("/api/admin/grant-autopilot/save", requiresDB(h, method("POST", h.GrantSave)))
	mux.HandleFunc("/api/admin/agent-api-keys", requiresDB(h, method("POST", h.AdminAgentKey)))
	mux.HandleFunc("/api/admin/tool-usage", requiresDB(h, method("GET", h.AdminToolUsage)))
	mux.HandleFunc("/api/public/impact", requiresDB(h, method("GET", h.PublicImpact)))
	mux.HandleFunc("/api/public/tool-prices", requiresDB(h, method("GET", h.ToolPrices)))
	mux.HandleFunc("/api/agent/health", requiresDB(h, method("GET", h.AgentTool)))
	mux.HandleFunc("/api/agent/wallet-score", requiresDB(h, method("POST", h.AgentTool)))
	mux.HandleFunc("/api/agent/risk-summary", requiresDB(h, method("POST", h.AgentTool)))
	mux.HandleFunc("/api/agent/metadata-template", requiresDB(h, method("POST", h.AgentTool)))
	mux.HandleFunc("/api/agent/chain-health", requiresDB(h, method("POST", h.AgentTool)))
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
	mux.HandleFunc("/api/metadata/generate", requiresDB(h, handlers.RequireAuth(method("POST", h.GenerateMetadata))))
	mux.HandleFunc("/api/risk/scan", requiresDB(h, handlers.RequireAuth(method("POST", h.ScanRisk))))
	mux.HandleFunc("/api/wallet/score", requiresDB(h, handlers.RequireAuth(method("POST", h.WalletScore))))
	mux.HandleFunc("/api/token/scan", requiresDB(h, handlers.RequireAuth(method("POST", h.TokenScan))))
	mux.HandleFunc("/api/portfolio/track", requiresDB(h, handlers.RequireAuth(method("POST", h.PortfolioTrack))))
	mux.HandleFunc("/api/smart-money", requiresDB(h, handlers.RequireAuth(method("GET", h.SmartMoney))))
	mux.HandleFunc("/api/airdrop/check", requiresDB(h, handlers.RequireAuth(method("POST", h.AirdropCheck))))
	mux.HandleFunc("/api/rug-radar/feed", method("GET", h.RugRadarFeed))
	mux.HandleFunc("/api/rug-radar/submit", requiresDB(h, handlers.RequireAuth(method("POST", h.RugRadarSubmit))))
	mux.HandleFunc("/api/program/scan", requiresDB(h, handlers.RequireAuth(method("POST", h.ProgramScan))))
	mux.HandleFunc("/api/ai/generate", requiresDB(h, handlers.RequireAuth(method("POST", h.AIGenerate))))
	mux.HandleFunc("/api/ai/jobs", requiresDB(h, handlers.RequireAuth(method("GET", h.AIJobs))))
	mux.HandleFunc("/api/v1/games", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateGameProject))))
	mux.HandleFunc("/api/v1/build/android", requiresDB(h, handlers.RequireAuth(method("POST", h.BuildAndroid))))
	mux.HandleFunc("/api/web3/sources", requiresDB(h, handlers.RequireAuth(h.Web3Sources)))
	mux.HandleFunc("/api/web3/sources/", requiresDB(h, handlers.RequireAuth(h.Web3Source)))
	mux.HandleFunc("/api/web3/events", requiresDB(h, handlers.RequireAuth(method("GET", h.Web3Events))))
	mux.HandleFunc("/api/web3/intelligence-graph", requiresDB(h, handlers.RequireAuth(h.IntelligenceGraph)))
	mux.HandleFunc("/api/web3/intelligence-graph/build", requiresDB(h, handlers.RequireAuth(method("POST", h.IntelligenceGraph))))
	mux.HandleFunc("/api/web3/risk-v2", requiresDB(h, handlers.RequireAuth(method("POST", h.RiskV2))))
	mux.HandleFunc("/api/web3/tx-decode-pro", requiresDB(h, handlers.RequireAuth(method("POST", h.TXDecodePro))))
	mux.HandleFunc("/api/web3/cross-chain-risk", requiresDB(h, handlers.RequireAuth(method("POST", h.CrossChainRisk))))
	mux.HandleFunc("/api/web3/sybil-check", requiresDB(h, handlers.RequireAuth(method("POST", h.SybilCheck))))
	mux.HandleFunc("/api/web3/funding-assistant/generate", requiresDB(h, handlers.RequireAuth(method("POST", h.FundingAssistant))))
	mux.HandleFunc("/api/artifacts/", requiresDB(h, handlers.RequireAuth(h.ArtifactRoute)))

	if staticDir != "" {
		if info, err := os.Stat(staticDir); err != nil || !info.IsDir() {
			log.Printf("warning: static directory unavailable at %q: %v", staticDir, err)
		} else {
			static := http.FileServer(http.Dir(staticDir))
			indexPath := filepath.Join(staticDir, "index.html")
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
					http.NotFound(w, r)
					return
				}
				cleanRoutes := map[string]string{
					"/hub":               "/hub.html",
					"/login":             "/login.html",
					"/register":          "/register.html",
					"/metadata":          "/metadata.html",
					"/risk":              "/risk.html",
					"/chains":            "/chains.html",
					"/watchlist":         "/watchlist.html",
					"/account":           "/account.html",
					"/pricing":           "/pricing.html",
					"/admin":             "/admin.html",
					"/admin-payments":    "/admin-payments.html",
					"/admin-analytics":   "/admin-analytics.html",
					"/dashboard":         "/dashboard.html",
					"/impact":            "/impact.html",
					"/docs":              "/docs.html",
					"/docs/api":          "/docs-api.html",
					"/docs/sdk":          "/docs-sdk.html",
					"/graph":             "/graph.html",
					"/risk-v2":           "/risk-v2.html",
					"/tx-decoder-pro":    "/tx-decoder-pro.html",
					"/cross-chain-risk":  "/cross-chain-risk.html",
					"/sybil-check":       "/sybil-check.html",
					"/funding-assistant": "/funding-assistant.html",
					"/agent-api":         "/agent-api.html",
					"/pay-per-tool":      "/pay-per-tool.html",
				}
				if staticPath, ok := cleanRoutes[r.URL.Path]; ok {
					r = r.Clone(r.Context())
					r.URL.Path = staticPath
				}
				candidate := filepath.Join(staticDir, strings.TrimPrefix(filepath.Clean(r.URL.Path), "/"))
				if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
					static.ServeHTTP(w, r)
					return
				}
				http.ServeFile(w, r, indexPath)
			})
		}
	}
	return securityHeaders(cors(apiReadiness(db, mux), corsOrigin))
}

func apiReadiness(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/version" && r.URL.Path != "/api/config" && r.URL.Path != "/api/auth/provision" && r.URL.Path != "/api/web3/health" && r.URL.Path != "/api/analytics/event" && db == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "database unavailable"})
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-admin-password, Authorization, X-Koschei-Source-Id")
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
