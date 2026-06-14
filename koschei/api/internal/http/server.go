package http

import (
	"database/sql"
	"log"
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

func WithReadDB(db *sql.DB) Option { return func(c *serverConfig) { c.dbRead = db } }
func WithCache(c cache.Cache) Option { return func(cfg *serverConfig) { if c != nil { cfg.cache = c } } }
func WithSolanaRPC(rpc *web3.SolanaRPC) Option { return func(c *serverConfig) { c.solanaRPC = rpc } }
func WithJobStore(store *jobs.Store) Option { return func(c *serverConfig) { c.jobStore = store } }
func WithJobQueue(queue jobs.Queue) Option { return func(c *serverConfig) { c.jobQueue = queue } }

func NewServer(db *sql.DB, dbInitError string, adminPassword string, corsOrigin string, staticDir string, opts ...Option) http.Handler {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		if missing := handlers.MissingProductionAuthEnv(); len(missing) > 0 {
			log.Fatalf("CRITICAL: missing required production auth env vars: %s", strings.Join(missing, ", "))
		}
	}
	cfg := serverConfig{cache: cache.NewNoop()}
	for _, opt := range opts { opt(&cfg) }
	if cfg.dbRead == nil { cfg.dbRead = db }
	if cfg.solanaRPC == nil { cfg.solanaRPC = web3.NewSolanaRPC(cfg.cache) }
	h := &handlers.Handler{DB: db, DBRead: cfg.dbRead, DBInitError: dbInitError, AdminPassword: adminPassword, Limiter: handlers.NewLimiter(), Cache: cfg.cache, SolanaRPC: cfg.solanaRPC, JobStore: cfg.jobStore, JobQueue: cfg.jobQueue}
	mux := http.NewServeMux()

	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/ads.txt", method("GET", adsTXT))
	mux.HandleFunc("/robots.txt", method("GET", robotsTXT))
	mux.HandleFunc("/api/version", method("GET", versionJSON))
	mux.HandleFunc("/api/config", method("GET", h.Config))
	mux.HandleFunc("/api/analytics/event", method("POST", h.AnalyticsEvent))
	mux.HandleFunc("/api/auth/register", method("POST", h.Register))
	mux.HandleFunc("/api/auth/login", method("POST", h.Login))
	mux.HandleFunc("/api/auth/provision", method("POST", h.Provision))
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
	mux.HandleFunc("/api/owner/users", requiresDB(h, ownerOnly(h, method("GET", h.OwnerUsers))))
	mux.HandleFunc("/api/owner/payment-requests", requiresDB(h, ownerOnly(h, method("GET", h.OwnerPaymentRequests))))
	mux.HandleFunc("/api/owner/payments/approve", requiresDB(h, ownerOnly(h, method("POST", h.OwnerApprovePayment))))
	mux.HandleFunc("/api/owner/payments/reject", requiresDB(h, ownerOnly(h, method("POST", h.OwnerRejectPayment))))
	mux.HandleFunc("/api/owner/health", requiresDB(h, ownerOnly(h, method("GET", h.OwnerHealth))))
	mux.HandleFunc("/api/owner/status", requiresDB(h, ownerOnly(h, method("GET", h.OwnerStatus))))
	mux.HandleFunc("/api/owner/command", requiresDB(h, ownerOnly(h, method("POST", h.OwnerCommand))))
	mux.HandleFunc("/owner", ownerPageHandler(staticDir))
	mux.HandleFunc("/owner.html", ownerPageHandler(staticDir))

	mux.HandleFunc("/api/web3/health", method("GET", h.Web3Health))
	mux.HandleFunc("/api/web3/health/logs", requiresDB(h, handlers.RequireAuth(method("GET", h.Web3HealthLogs))))
	mux.HandleFunc("/api/v1/unified/analyze", handlers.RequireAuth(method("POST", h.UnifiedIntelligenceHandler)))
	mux.HandleFunc("/api/paddle/status", method("GET", h.PaddleStatus))
	mux.HandleFunc("/api/paddle/checkout", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateCheckout))))
	mux.HandleFunc("/api/v1/paddle/checkout", requiresDB(h, handlers.RequireAuth(method("POST", h.CreateCheckout))))
	mux.HandleFunc("/api/paddle/webhook", requiresDB(h, method("POST", h.HandleWebhook)))
	mux.HandleFunc("/api/v1/paddle/webhook", requiresDB(h, method("POST", h.HandleWebhook)))

	mux.HandleFunc("/jarvis", redirectToDashboard)
	mux.HandleFunc("/jarvis.html", redirectToDashboard)
	registerLegacyDashboardRedirects(mux)
	installStaticRoutes(mux, staticDir)
	return securityHeaders(cors(apiReadiness(db, mux), corsOrigin))
}
