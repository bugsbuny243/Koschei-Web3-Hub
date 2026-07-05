package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
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

func WithReadDB(db *sql.DB) Option {
	return func(c *serverConfig) { c.dbRead = db }
}

func WithCache(value cache.Cache) Option {
	return func(c *serverConfig) {
		if value != nil {
			c.cache = value
		}
	}
}

func WithSolanaRPC(rpc *web3.SolanaRPC) Option {
	return func(c *serverConfig) { c.solanaRPC = rpc }
}

func WithJobStore(store *jobs.Store) Option {
	return func(c *serverConfig) { c.jobStore = store }
}

func WithJobQueue(queue jobs.Queue) Option {
	return func(c *serverConfig) { c.jobQueue = queue }
}

func NewServer(db *sql.DB, dbInitError string, adminPassword string, corsOrigin string, staticDir string, opts ...Option) http.Handler {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		if missing := handlers.MissingProductionAuthEnv(); len(missing) > 0 {
			panic("production auth env missing: " + strings.Join(missing, ", "))
		}
	}
	config := serverConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	cacheValue := config.cache
	if cacheValue == nil {
		cacheValue = cache.NewMemoryCache()
	}
	h := &handlers.Handler{DB: db, DBRead: config.dbRead, AdminPassword: adminPassword, Limiter: handlers.NewRateLimiter(), DBInitError: dbInitError, Cache: cacheValue, SolanaRPC: config.solanaRPC, JobStore: config.jobStore, JobQueue: config.jobQueue}
	if h.DBRead == nil {
		h.DBRead = db
	}
	mux := http.NewServeMux()
	premium := func(next http.HandlerFunc) http.HandlerFunc {
		return handlers.RequireAuth(h.RequireActiveEntitlement(next))
	}

	registerCoreRoutes(mux, h)
	registerOwnerRoutes(mux, h, staticDir)
	registerRuntimeRoutes(mux, h)
	registerCustomerProductRoutes(mux, h, premium)
	registerB2BRoutes(mux, h)
	registerWeb3Routes(mux, h, premium)
	registerStaticFiles(mux, staticDir)

	return securityHeaders(cors(apiReadiness(db, mux), corsOrigin))
}

func registerCoreRoutes(mux *http.ServeMux, h *handlers.Handler) {
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

	mux.HandleFunc("/api/me", handlers.RequireAuth(method("GET", h.Me)))
	mux.HandleFunc("/api/me/package", handlers.RequireAuth(method("GET", h.MePackage)))
	mux.HandleFunc("/api/member/summary", requiresDB(h, handlers.RequireAuth(method("GET", h.MemberSummary))))
	mux.HandleFunc("/api/payments/request", requiresDB(h, handlers.RequireAuth(method("POST", h.PaymentRequest))))
	mux.HandleFunc("/api/shopier/webhook", requiresDB(h, method("POST", h.ShopierWebhook)))

	mux.HandleFunc("/api/arvis/preflight", method("POST", h.ARVISPreflight))
	mux.HandleFunc("/api/public/impact", method("GET", h.PublicImpact))
	mux.HandleFunc("/api/public/metrics", method("GET", h.GetPublicMetrics))
	mux.HandleFunc("/api/public/tool-prices", requiresDB(h, method("GET", h.ToolPrices)))
	mux.HandleFunc("/api/agent/health", requiresDB(h, method("GET", h.AgentTool)))
	mux.HandleFunc("/api/agent/wallet-score", requiresDB(h, method("POST", h.AgentTool)))
	mux.HandleFunc("/api/agent/risk-summary", requiresDB(h, method("POST", h.AgentTool)))
	mux.HandleFunc("/api/agent/metadata-template", requiresDB(h, method("POST", h.AgentTool)))
	mux.HandleFunc("/api/agent/chain-health", requiresDB(h, method("POST", h.AgentTool)))
}

func registerOwnerRoutes(mux *http.ServeMux, h *handlers.Handler, staticDir string) {
	mux.HandleFunc("/api/owner/login", method("POST", h.OwnerLoginAudited))
	mux.HandleFunc("/api/owner/logout", ownerOnly(h, method("POST", h.OwnerLogout)))
	mux.HandleFunc("/api/owner/command-center", ownerOnly(h, method("GET", h.OwnerCommandCenterStatus)))
	mux.HandleFunc("/api/owner/feedback", requiresDB(h, ownerOnly(h, h.OwnerFeedback)))
	mux.HandleFunc("/api/owner/users", requiresDB(h, ownerOnly(h, method("GET", h.OwnerUsersV2))))
	mux.HandleFunc("/api/owner/credits/add", requiresDB(h, ownerOnly(h, method("POST", h.OwnerAddCredits))))
	mux.HandleFunc("/api/owner/users/ban", requiresDB(h, ownerOnly(h, method("POST", h.OwnerBanUser))))
	mux.HandleFunc("/api/owner/users/remove", requiresDB(h, ownerOnly(h, method("POST", h.OwnerRemoveUser))))
	mux.HandleFunc("/api/owner/payment-requests", requiresDB(h, ownerOnly(h, method("GET", h.OwnerPaymentRequests))))
	mux.HandleFunc("/api/owner/payment-health", requiresDB(h, ownerOnly(h, method("GET", h.OwnerPaymentHealth))))
	mux.HandleFunc("/api/owner/payments/approve", requiresDB(h, ownerOnly(h, method("POST", h.OwnerApprovePayment))))
	mux.HandleFunc("/api/owner/payments/reject", requiresDB(h, ownerOnly(h, method("POST", h.OwnerRejectPayment))))
	mux.HandleFunc("/api/owner/command", requiresDB(h, ownerOnly(h, method("POST", h.OwnerCommand))))
	mux.HandleFunc("/api/owner/brain", requiresDB(h, ownerOnly(h, method("POST", h.OwnerBrain))))
	mux.HandleFunc("/api/owner/chat", requiresDB(h, ownerOnly(h, h.OwnerChat)))
	mux.HandleFunc("/api/owner/health", requiresDB(h, ownerOnly(h, method("GET", h.OwnerHealth))))
	mux.HandleFunc("/api/owner/status", requiresDB(h, ownerOnly(h, method("GET", h.OwnerStatus))))
}
