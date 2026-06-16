package http

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
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
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		if missing := handlers.MissingProductionAuthEnv(); len(missing) > 0 {
			log.Fatalf("CRITICAL: missing required production auth env vars: %s", strings.Join(missing, ", "))
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
	premium := func(next http.HandlerFunc) http.HandlerFunc {
		return handlers.RequireAuth(h.RequireActiveEntitlement(next))
	}
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
	mux.HandleFunc("/api/me", handlers.RequireAuth(method("GET", h.Me)))
	mux.HandleFunc("/api/me/package", handlers.RequireAuth(method("GET", h.MePackage)))
	mux.HandleFunc("/api/member/summary", requiresDB(h, handlers.RequireAuth(method("GET", h.MemberSummary))))
	mux.HandleFunc("/api/payments/request", requiresDB(h, handlers.RequireAuth(method("POST", h.PaymentRequest))))
	mux.HandleFunc("/api/shopier/webhook", requiresDB(h, method("POST", h.ShopierWebhook)))
	mux.HandleFunc("/api/owner/login", method("POST", h.OwnerLogin))
	mux.HandleFunc("/api/owner/users", requiresDB(h, ownerOnly(h, method("GET", h.OwnerUsersV2))))
	mux.HandleFunc("/api/owner/credits/add", requiresDB(h, ownerOnly(h, method("POST", h.OwnerAddCredits))))
	mux.HandleFunc("/api/owner/users/ban", requiresDB(h, ownerOnly(h, method("POST", h.OwnerBanUser))))
	mux.HandleFunc("/api/owner/users/remove", requiresDB(h, ownerOnly(h, method("POST", h.OwnerRemoveUser))))
	mux.HandleFunc("/api/owner/payment-requests", requiresDB(h, ownerOnly(h, method("GET", h.OwnerPaymentRequests))))
	mux.HandleFunc("/api/owner/payments/approve", requiresDB(h, ownerOnly(h, method("POST", h.OwnerApprovePayment))))
	mux.HandleFunc("/api/owner/payments/reject", requiresDB(h, ownerOnly(h, method("POST", h.OwnerRejectPayment))))
	mux.HandleFunc("/api/owner/command", requiresDB(h, ownerOnly(h, method("POST", h.OwnerCommand))))
	mux.HandleFunc("/api/owner/brain", requiresDB(h, ownerOnly(h, method("POST", h.OwnerBrain))))
	mux.HandleFunc("/api/owner/health", requiresDB(h, ownerOnly(h, method("GET", h.OwnerHealth))))
	mux.HandleFunc("/api/owner/status", requiresDB(h, ownerOnly(h, method("GET", h.OwnerStatus))))
	mux.HandleFunc("/api/owner/radar/summary", requiresDB(h, ownerOnly(h, method("GET", h.OwnerRadarSummary))))
	mux.HandleFunc("/api/owner/radar/sources", requiresDB(h, ownerOnly(h, h.OwnerRadarSources)))
	mux.HandleFunc("/api/owner/radar/sources/disable", requiresDB(h, ownerOnly(h, method("POST", h.OwnerDisableRadarSource))))
	mux.HandleFunc("/api/owner/emergency", ownerOnly(h, method("POST", h.OwnerEmergencyControl)))
	mux.HandleFunc("/api/owner/grants", requiresDB(h, ownerOnly(h, method("GET", h.OwnerGrants))))
	mux.HandleFunc("/api/owner/dao-guardian", requiresDB(h, ownerOnly(h, method("GET", h.OwnerDAOGuardianSummary))))
	mux.HandleFunc("/owner", ownerPageHandler(staticDir))
	mux.HandleFunc("/owner.html", ownerPageHandler(staticDir))
	mux.HandleFunc("/jarvis", redirectToDashboard)
	mux.HandleFunc("/jarvis.html", redirectToDashboard)
	registerLegacyDashboardRedirects(mux)
	mux.HandleFunc("/api/public/impact", method("GET", h.PublicImpact))
	mux.HandleFunc("/api/public/metrics", method("GET", h.GetPublicMetrics))
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
	mux.HandleFunc("/api/risk/scan", requiresDB(h, premium(method("POST", h.ScanRisk))))
	mux.HandleFunc("/api/mev/analyze", requiresDB(h, handlers.RequireAuth(method("POST", h.AnalyzeMEV))))
	mux.HandleFunc("/api/liquidity/analyze", requiresDB(h, handlers.RequireAuth(method("POST", h.LiquidityDrainAnalyze))))
	mux.HandleFunc("/api/dao/proposal-risk", requiresDB(h, handlers.RequireAuth(method("POST", h.DAOGuardianAnalyze))))
	mux.HandleFunc("/api/wallet/score", requiresDB(h, premium(method("POST", h.WalletScore))))
	mux.HandleFunc("/api/token/scan", requiresDB(h, premium(method("POST", h.TokenScan))))
	mux.HandleFunc("/api/account/api-keys", requiresDB(h, handlers.RequireAuth(h.APIKeysCollection)))
	mux.HandleFunc("/api/account/api-keys/", requiresDB(h, handlers.RequireAuth(method("POST", h.RevokeAPIKey))))
	mux.HandleFunc("/api/v1/scan/token", requiresDB(h, h.APIKeyAuth(h.CheckB2BQuota(method("POST", h.B2BTokenScan)))))
	mux.HandleFunc("/api/v1/unified/analyze", handlers.RequireAuth(method("POST", h.UnifiedIntelligenceHandler)))
	mux.HandleFunc("/api/v1/radar/feed", requiresDB(h, handlers.RequireAuth(h.RequireActiveEntitlement(method("GET", h.SecurityRadarFeed)))))
	mux.HandleFunc("/api/v1/radar/check", requiresDB(h, handlers.RequireAuth(h.RequireActiveEntitlement(method("POST", h.SecurityRadarCheck)))))
	mux.HandleFunc("/api/v1/risk/badge", method("GET", h.SecurityRiskBadge))
	mux.HandleFunc("/api/v1/mev/analyze", requiresDB(h, handlers.RequireAuth(h.APIKeyAuth(h.APIRateLimit(method("POST", h.MEVAnalyze))))))
	mux.HandleFunc("/api/v1/mev/protected-send", requiresDB(h, handlers.RequireAuth(h.APIKeyAuth(h.APIRateLimit(method("POST", h.ProtectedSend))))))
	mux.HandleFunc("/api/v1/liquidity/analyze", requiresDB(h, handlers.RequireAuth(h.APIKeyAuth(h.APIRateLimit(method("POST", h.LiquidityDrainAnalyze))))))
	mux.HandleFunc("/api/v1/radar/emergency", requiresDB(h, handlers.RequireAuth(h.APIKeyAuth(h.APIRateLimit(method("POST", h.EmergencyAlert))))))
	mux.HandleFunc("/api/v1/dao/proposal-risk", requiresDB(h, h.APIKeyAuth(h.APIRateLimit(method("POST", h.DAOGuardianAnalyze)))))
	mux.HandleFunc("/api/v1/smart-money/snapshot", requiresDB(h, h.APIKeyAuth(h.APIRateLimit(method("GET", h.SmartMoneySnapshot)))))
	mux.HandleFunc("/api/v1/usage", requiresDB(h, h.APIKeyAuth(method("GET", h.APIUsage))))
	mux.HandleFunc("/api/paddle/checkout", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateCheckout))))
	mux.HandleFunc("/api/paddle/status", handlers.RequireAuth(method("GET", h.PaddleStatus)))
	mux.HandleFunc("/api/v1/paddle/checkout", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateCheckout))))
	mux.HandleFunc("/api/v1/b2b/checkout", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateCheckout))))
	mux.HandleFunc("/api/paddle/webhook", requiresDB(h, method("POST", h.HandleWebhook)))
	mux.HandleFunc("/api/v1/paddle/webhook", requiresDB(h, method("POST", h.HandleWebhook)))
	mux.HandleFunc("/api/v1/b2b/usage", requiresDB(h, h.APIKeyAuth(method("GET", h.B2BUsage))))
	mux.HandleFunc("/api/tx/decode", requiresDB(h, premium(method("POST", h.TXDecode))))
	mux.HandleFunc("/api/tx/mev-warning", requiresDB(h, handlers.RequireAuth(method("POST", h.TXDecoderMEVWarning))))
	mux.HandleFunc("/api/jobs/token-scan", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateWeb3Job))))
	mux.HandleFunc("/api/jobs/wallet-score", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateWeb3Job))))
	mux.HandleFunc("/api/jobs/tx-decode", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateWeb3Job))))
	mux.HandleFunc("/api/jobs/", requiresDB(h, handlers.RequireAuth(method("GET", h.GetWeb3Job))))
	mux.HandleFunc("/api/portfolio/track", requiresDB(h, premium(method("POST", h.PortfolioTrack))))
	mux.HandleFunc("/api/smart-money", requiresDB(h, handlers.RequireAuth(method("GET", h.SmartMoney))))
	mux.HandleFunc("/ws/smart-money", method("GET", h.SmartMoneyStream))
	mux.HandleFunc("/api/airdrop/check", requiresDB(h, handlers.RequireAuth(method("POST", h.AirdropCheck))))
	mux.HandleFunc("/api/rug-radar/feed", method("GET", h.RugRadarFeed))
	mux.HandleFunc("/api/rug-radar/submit", requiresDB(h, handlers.RequireAuth(method("POST", h.RugRadarSubmit))))
	mux.HandleFunc("/api/program/scan", requiresDB(h, handlers.RequireAuth(method("POST", h.ProgramScan))))
	mux.HandleFunc("/api/project-radar/scan", requiresDB(h, premium(method("POST", h.ProjectRadar))))
	mux.HandleFunc("/api/ai/generate", requiresDB(h, handlers.RequireAuth(method("POST", h.AIGenerate))))
	mux.HandleFunc("/api/ai/jobs", requiresDB(h, handlers.RequireAuth(method("GET", h.AIJobs))))
	mux.HandleFunc("/api/v1/games", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateGameProject))))
	mux.HandleFunc("/api/v1/build/android", requiresDB(h, handlers.RequireAuth(method("POST", h.BuildAndroid))))
	mux.HandleFunc("/api/web3/sources", requiresDB(h, handlers.RequireAuth(h.Web3Sources)))
	mux.HandleFunc("/api/web3/sources/", requiresDB(h, handlers.RequireAuth(h.Web3Source)))
	mux.HandleFunc("/api/web3/events", requiresDB(h, handlers.RequireAuth(method("GET", h.Web3Events))))
	mux.HandleFunc("/api/web3/intelligence-graph", requiresDB(h, premium(h.IntelligenceGraph)))
	mux.HandleFunc("/api/web3/intelligence-graph/build", requiresDB(h, premium(method("POST", h.IntelligenceGraph))))
	mux.HandleFunc("/api/web3/risk-v2", requiresDB(h, premium(method("POST", h.RiskV2))))
	mux.HandleFunc("/api/web3/tx-decode-pro", requiresDB(h, premium(method("POST", h.TXDecodePro))))
	mux.HandleFunc("/api/web3/cross-chain-risk", requiresDB(h, premium(method("POST", h.CrossChainRisk))))
	mux.HandleFunc("/api/web3/sybil-check", requiresDB(h, premium(method("POST", h.SybilCheck))))
	mux.HandleFunc("/api/web3/funding-assistant/generate", requiresDB(h, premium(method("POST", h.FundingAssistant))))
	mux.HandleFunc("/api/web3/project-radar", requiresDB(h, premium(method("POST", h.ProjectRadar))))
	mux.HandleFunc("/api/grants/readiness", requiresDB(h, premium(method("POST", h.FundingAssistant))))
	mux.HandleFunc("/api/graph/build", requiresDB(h, premium(method("POST", h.IntelligenceGraph))))
	mux.HandleFunc("/api/sybil/check", requiresDB(h, premium(method("POST", h.SybilCheck))))
	mux.HandleFunc("/api/artifacts/", requiresDB(h, handlers.RequireAuth(h.ArtifactRoute)))

	if staticDir != "" {
		if info, err := os.Stat(staticDir); err != nil || !info.IsDir() {
			log.Printf("warning: static directory unavailable at %q: %v", staticDir, err)
		} else {
			static := http.FileServer(http.Dir(staticDir))
			indexPath := filepath.Join(staticDir, "index.html")
			cleanRoutes := publicCleanRoutes(staticDir)
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
					http.NotFound(w, r)
					return
				}
				if r.URL.Path == "/" {
					http.ServeFile(w, r, indexPath)
					return
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

const robotsTXTBody = "User-agent: *\nAllow: /\nSitemap: https://tradepigloball.co/sitemap.xml"

func publicCleanRoutes(staticDir string) map[string]string {
	routes := map[string]string{
		"/api-docs": "/docs-api.html",
		"/docs/api": "/docs-api.html",
		"/docs/sdk": "/docs-sdk.html",
		"/sdk":      "/docs-sdk.html",
		"/tools":    "/dashboard.html",
	}

	entries, err := os.ReadDir(staticDir)
	if err != nil {
		return routes
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".html") || name == "index.html" {
			continue
		}
		slug := strings.TrimSuffix(name, ".html")
		routes["/"+slug] = "/" + name
	}
	return routes
}

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
