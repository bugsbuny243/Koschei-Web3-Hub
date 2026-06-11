package http

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"koschei/api/internal/cache"
	"koschei/api/internal/handlers"
	"koschei/api/internal/jobs"
	"koschei/api/internal/web3"
)

type serverConfig struct {
	dbRead    *sql.DB
	cache     cache.Cache
	solanaRPC *web3.SolanaRPC
	jobStore  *jobs.Store
	jobQueue  jobs.Queue
}

type Option func(*serverConfig)

func WithReadDB(db *sql.DB) Option { return func(c *serverConfig) { c.dbRead = db } }
func WithCache(cache cache.Cache) Option {
	return func(c *serverConfig) {
		if cache != nil {
			c.cache = cache
		}
	}
}
func WithSolanaRPC(rpc *web3.SolanaRPC) Option { return func(c *serverConfig) { c.solanaRPC = rpc } }
func WithJobStore(store *jobs.Store) Option    { return func(c *serverConfig) { c.jobStore = store } }
func WithJobQueue(queue jobs.Queue) Option     { return func(c *serverConfig) { c.jobQueue = queue } }

func NewServer(db *sql.DB, dbInitError string, adminPassword string, corsOrigin string, staticDir string, opts ...Option) http.Handler {
	if os.Getenv("APP_ENV") == "production" {
		if strings.TrimSpace(handlers.ConfiguredNeonAuthJWKSURL()) == "" {
			log.Print("CRITICAL: NEON_AUTH_JWKS_URL must be set in production")
		}
		if strings.TrimSpace(handlers.ConfiguredNeonAuthIssuer()) == "" {
			log.Print("CRITICAL: NEON_AUTH_ISSUER should be set in production")
		}
	}
	cfg := serverConfig{cache: cache.NewNoop()}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.dbRead == nil {
		cfg.dbRead = db
	}
	if cfg.solanaRPC == nil {
		cfg.solanaRPC = web3.NewSolanaRPC(cfg.cache)
	}
	h := &handlers.Handler{DB: db, DBRead: cfg.dbRead, DBInitError: dbInitError, AdminPassword: adminPassword, Limiter: handlers.NewLimiter(), Cache: cfg.cache, SolanaRPC: cfg.solanaRPC, JobStore: cfg.jobStore, JobQueue: cfg.jobQueue}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/config", method("GET", h.Config))
	mux.HandleFunc("/api/auth/provision", method("POST", h.Provision))
	mux.HandleFunc("/api/web3/health", method("GET", h.Web3Health))
	mux.HandleFunc("/api/web3/health/logs", requiresDB(h, handlers.RequireAuth(method("GET", h.Web3HealthLogs))))
	mux.HandleFunc("/api/analytics/event", method("POST", h.AnalyticsEvent))
	mux.HandleFunc("/ads.txt", method("GET", adsTXT))
	mux.HandleFunc("/robots.txt", method("GET", robotsTXT))
	mux.HandleFunc("/api/version", method("GET", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"app": "koschei-engine", "status": "ok"})
	}))
	mux.HandleFunc("/api/auth/register", method("POST", h.Register))
	mux.HandleFunc("/api/auth/login", method("POST", h.Login))
	mux.HandleFunc("/api/auth/neon-login", method("GET", h.NeonLogin))
	mux.HandleFunc("/api/auth/neon-register", method("GET", h.NeonRegister))
	mux.HandleFunc("/api/auth/neon-callback", method("GET", h.NeonCallback))
	mux.HandleFunc("/api/auth/otp/start", method("POST", h.StartOTPLogin))
	mux.HandleFunc("/api/auth/otp/verify", method("POST", h.VerifyOTPLogin))
	mux.HandleFunc("/api/me", requiresDB(h, handlers.RequireAuth(method("GET", h.Me))))
	mux.HandleFunc("/api/member/summary", requiresDB(h, handlers.RequireAuth(method("GET", h.MemberSummary))))
	mux.HandleFunc("/api/payments/request", requiresDB(h, handlers.RequireAuth(method("POST", h.PaymentRequest))))
	mux.HandleFunc("/api/shopier/webhook", requiresDB(h, method("POST", h.ShopierWebhook)))
	mux.HandleFunc("/api/owner/users", requiresDB(h, method("GET", h.OwnerUsers)))
	mux.HandleFunc("/api/owner/credits/add", requiresDB(h, method("POST", h.OwnerAddCredits)))
	mux.HandleFunc("/api/owner/users/ban", requiresDB(h, method("POST", h.OwnerBanUser)))
	mux.HandleFunc("/api/owner/payment-requests", requiresDB(h, method("GET", h.OwnerPaymentRequests)))
	mux.HandleFunc("/api/owner/payments/approve", requiresDB(h, method("POST", h.OwnerApprovePayment)))
	mux.HandleFunc("/api/owner/payments/reject", requiresDB(h, method("POST", h.OwnerRejectPayment)))
	mux.HandleFunc("/api/owner/command", requiresDB(h, method("POST", h.OwnerCommand)))
	mux.HandleFunc("/api/owner/status", requiresDB(h, method("GET", h.OwnerStatus)))
	mux.HandleFunc("/api/admin/payment-requests", requiresDB(h, method("GET", h.AdminPaymentRequests)))
	mux.HandleFunc("/api/admin/analytics/events", requiresDB(h, method("GET", h.AdminAnalyticsEvents)))
	mux.HandleFunc("/api/admin/grant-radar", requiresDB(h, h.GrantRadar))
	mux.HandleFunc("/api/admin/proof-of-impact", requiresDB(h, method("GET", h.ProofOfImpact)))
	mux.HandleFunc("/api/admin/payment-requests/approve", requiresDB(h, method("POST", h.ApprovePaymentRequest)))
	mux.HandleFunc("/api/admin/payment-requests/reject", requiresDB(h, method("POST", h.RejectPaymentRequest)))
	mux.HandleFunc("/api/admin/summary", requiresDB(h, method("GET", h.AdminSummary)))
	mux.HandleFunc("/api/admin/users", requiresDB(h, method("GET", h.AdminUsers)))
	mux.HandleFunc("/api/admin/users/action", requiresDB(h, method("POST", h.AdminUserAction)))
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
	mux.HandleFunc("/api/admin/settings", requiresDB(h, method("GET", h.AdminSettings)))
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
	mux.HandleFunc("/api/account/api-keys", requiresDB(h, handlers.RequireAuth(h.APIKeysCollection)))
	mux.HandleFunc("/api/account/api-keys/", requiresDB(h, handlers.RequireAuth(method("POST", h.RevokeAPIKey))))
	mux.HandleFunc("/api/v1/scan/token", requiresDB(h, h.APIKeyAuth(h.APIRateLimit(method("POST", h.B2BTokenScan)))))
	mux.HandleFunc("/api/v1/usage", requiresDB(h, h.APIKeyAuth(method("GET", h.APIUsage))))
	mux.HandleFunc("/api/tx/decode", requiresDB(h, handlers.RequireAuth(method("POST", h.TXDecode))))
	mux.HandleFunc("/api/jobs/token-scan", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateWeb3Job))))
	mux.HandleFunc("/api/jobs/wallet-score", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateWeb3Job))))
	mux.HandleFunc("/api/jobs/tx-decode", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateWeb3Job))))
	mux.HandleFunc("/api/jobs/", requiresDB(h, handlers.RequireAuth(method("GET", h.GetWeb3Job))))
	mux.HandleFunc("/api/portfolio/track", requiresDB(h, handlers.RequireAuth(method("POST", h.PortfolioTrack))))
	mux.HandleFunc("/api/smart-money", requiresDB(h, handlers.RequireAuth(method("GET", h.SmartMoney))))
	mux.HandleFunc("/api/airdrop/check", requiresDB(h, handlers.RequireAuth(method("POST", h.AirdropCheck))))
	mux.HandleFunc("/api/rug-radar/feed", method("GET", h.RugRadarFeed))
	mux.HandleFunc("/api/rug-radar/submit", requiresDB(h, handlers.RequireAuth(method("POST", h.RugRadarSubmit))))
	mux.HandleFunc("/api/program/scan", requiresDB(h, handlers.RequireAuth(method("POST", h.ProgramScan))))
	mux.HandleFunc("/api/project-radar/scan", requiresDB(h, handlers.RequireAuth(method("POST", h.ProjectRadar))))
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
	mux.HandleFunc("/api/web3/project-radar", requiresDB(h, handlers.RequireAuth(method("POST", h.ProjectRadar))))
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
				if r.URL.Path == "/" {
					http.ServeFile(w, r, indexPath)
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
					"/support":           "/support.html",
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
					"/project-radar":     "/project-radar.html",
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

const adsTXTBody = "google.com, pub-6081394144742471, DIRECT, f08c47fec0942fa0"

const robotsTXTBody = "User-agent: *\nAllow: /\n\nSitemap: https://tradepigloball.co/sitemap.xml"

func adsTXT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Robots-Tag", "all")
	_, _ = w.Write([]byte(adsTXTBody))
}

func robotsTXT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(robotsTXTBody))
}

func apiReadiness(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/version" && r.URL.Path != "/api/config" && r.URL.Path != "/api/auth/register" && r.URL.Path != "/api/auth/login" && r.URL.Path != "/api/auth/provision" && r.URL.Path != "/api/auth/neon-login" && r.URL.Path != "/api/auth/neon-register" && r.URL.Path != "/api/auth/neon-callback" && r.URL.Path != "/api/web3/health" && r.URL.Path != "/api/analytics/event" && db == nil {
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-admin-password, Authorization, X-API-Key, X-Koschei-Source-Id, x-koschei-agent-key")
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
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy())
		if os.Getenv("APP_ENV") == "production" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}

func contentSecurityPolicy() string {
	connectSrc := "'self' https://www.google-analytics.com https://region1.google-analytics.com"
	if authOrigin := publicNeonAuthOrigin(); authOrigin != "" {
		connectSrc += " " + authOrigin
	}
	return "default-src 'self'; script-src 'self' 'unsafe-inline' https://pagead2.googlesyndication.com https://www.googletagmanager.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src " + connectSrc + "; frame-src https://googleads.g.doubleclick.net https://tpc.googlesyndication.com; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
}

func publicNeonAuthOrigin() string {
	raw := strings.TrimSpace(os.Getenv("EXPO_PUBLIC_NEON_AUTH_URL"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("NEON_AUTH_BASE_URL"))
	}
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
