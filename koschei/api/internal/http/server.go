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
func WithCache(value cache.Cache) Option {
	return func(c *serverConfig) {
		if value != nil {
			c.cache = value
		}
	}
}
func WithSolanaRPC(rpc *web3.SolanaRPC) Option { return func(c *serverConfig) { c.solanaRPC = rpc } }
func WithJobStore(store *jobs.Store) Option    { return func(c *serverConfig) { c.jobStore = store } }
func WithJobQueue(queue jobs.Queue) Option     { return func(c *serverConfig) { c.jobQueue = queue } }

func NewServer(db *sql.DB, dbInitError string, adminPassword string, corsOrigin string, staticDir string, opts ...Option) http.Handler {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		if missing := handlers.MissingProductionAuthEnv(); len(missing) > 0 {
			panic("production auth env missing: " + strings.Join(missing, ", "))
		}
	}
	config := serverConfig{cache: cache.NewNoop()}
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	if config.dbRead == nil {
		config.dbRead = db
	}
	if config.solanaRPC == nil {
		config.solanaRPC = web3.NewSolanaRPC(config.cache)
	}
	h := &handlers.Handler{DB: db, DBRead: config.dbRead, AdminPassword: adminPassword, Limiter: handlers.NewLimiter(), DBInitError: dbInitError, Cache: config.cache, SolanaRPC: config.solanaRPC, JobStore: config.jobStore, JobQueue: config.jobQueue}
	mux := http.NewServeMux()
	koschAccess := func(next http.HandlerFunc) http.HandlerFunc {
		return handlers.RequireAuth(h.RequireActiveEntitlement(next))
	}
	apiKey := func(next http.HandlerFunc) http.HandlerFunc {
		return h.APIKeyAuth(h.RequireAPIKeyKOSCH(h.APIRateLimit(next)))
	}
	registerCoreRoutes(mux, h, koschAccess)
	registerAccountRoutes(mux, h, koschAccess)
	registerOwnerRoutes(mux, h, staticDir)
	registerProductRoutes(mux, h, koschAccess)
	registerDeveloperAPIRoutes(mux, h, apiKey)
	registerWatchlistRoutes(mux, h, koschAccess)
	registerStatic(mux, staticDir)
	return securityHeaders(cors(apiReadiness(db, mux), corsOrigin))
}

func registerCoreRoutes(mux *http.ServeMux, h *handlers.Handler, koschAccess func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/config", method("GET", h.Config))
	mux.HandleFunc("/api/auth/provision", method("POST", h.Provision))
	mux.HandleFunc("/api/web3/health", method("GET", h.Web3Health))
	mux.HandleFunc("/api/web3/health/logs", requiresDB(h, handlers.RequireAuth(method("GET", h.Web3HealthLogs))))
	mux.HandleFunc("/api/analytics/event", method("POST", h.AnalyticsEvent))
	mux.HandleFunc("/ads.txt", method("GET", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("google.com, pub-6081394144742471, DIRECT, f08c47fec0942fa0"))
	}))
	mux.HandleFunc("/robots.txt", method("GET", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\nSitemap: https://tradepigloball.co/sitemap.xml"))
	}))
	mux.HandleFunc("/api/version", method("GET", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"app": "koschei-engine", "status": "ok", "access": "free-core-kosch-premium"})
	}))
	mux.HandleFunc("/api/auth/register", method("POST", h.Register))
	mux.HandleFunc("/api/auth/login", method("POST", h.Login))
	mux.HandleFunc("/api/auth/neon-login", method("GET", h.NeonLogin))
	mux.HandleFunc("/api/auth/neon-register", method("GET", h.NeonRegister))
	mux.HandleFunc("/api/auth/neon-callback", method("GET", h.NeonCallback))
	mux.HandleFunc("/api/me", handlers.RequireAuth(method("GET", h.Me)))
	mux.HandleFunc("/api/arvis/preflight", method("POST", h.ARVISPreflight))
	mux.HandleFunc("/api/public/impact", method("GET", h.PublicImpact))
	mux.HandleFunc("/api/public/metrics", method("GET", h.GetPublicMetrics))
	mux.HandleFunc("/api/agent/health", requiresDB(h, method("GET", h.AgentTool)))
	mux.HandleFunc("/api/agent/wallet-score", requiresDB(h, koschAccess(method("POST", h.AgentTool))))
	mux.HandleFunc("/api/agent/risk-summary", requiresDB(h, koschAccess(method("POST", h.AgentTool))))
	mux.HandleFunc("/api/agent/metadata-template", requiresDB(h, koschAccess(method("POST", h.AgentTool))))
	mux.HandleFunc("/api/agent/chain-health", requiresDB(h, koschAccess(method("POST", h.AgentTool))))
}

func registerAccountRoutes(mux *http.ServeMux, h *handlers.Handler, koschAccess func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/api/account/api-keys", requiresDB(h, koschAccess(h.APIKeysCollection)))
	mux.HandleFunc("/api/account/api-keys/", requiresDB(h, koschAccess(method("POST", h.RevokeAPIKey))))
}

func registerOwnerRoutes(mux *http.ServeMux, h *handlers.Handler, staticDir string) {
	mux.HandleFunc("/api/owner/login", method("POST", h.OwnerLoginAudited))
	mux.HandleFunc("/api/owner/logout", ownerOnly(h, method("POST", h.OwnerLogout)))
	mux.HandleFunc("/api/owner/command-center", ownerOnly(h, method("GET", h.OwnerCommandCenterStatus)))
	mux.HandleFunc("/api/owner/operations", ownerOnly(h, method("GET", h.OwnerOperationsStatus)))
	mux.HandleFunc("/api/owner/arvis", requiresDB(h, ownerOnly(h, method("GET", h.OwnerRadarOverview))))
	mux.HandleFunc("/api/owner/arvis/scan", requiresDB(h, ownerOnly(h, method("POST", h.OwnerRadarScan))))
	mux.HandleFunc("/api/owner/radar/sources", requiresDB(h, ownerOnly(h, h.OwnerRadarSources)))
	mux.HandleFunc("/api/owner/kosch-access", requiresDB(h, ownerOnly(h, method("GET", h.OwnerKOSCHAccess))))
	mux.HandleFunc("/api/owner/security-events", requiresDB(h, ownerOnly(h, method("GET", h.OwnerSecurityEvents))))
	mux.HandleFunc("/api/owner/route-map", ownerOnly(h, method("GET", ownerRouteMap)))
	mux.HandleFunc("/api/owner/feedback", requiresDB(h, ownerOnly(h, h.OwnerFeedback)))
	mux.HandleFunc("/api/owner/users", requiresDB(h, ownerOnly(h, method("GET", h.OwnerUsersV2))))
	mux.HandleFunc("/api/owner/users/ban", requiresDB(h, ownerOnly(h, method("POST", h.OwnerBanUser))))
	mux.HandleFunc("/api/owner/users/remove", requiresDB(h, ownerOnly(h, method("POST", h.OwnerRemoveUser))))
	mux.HandleFunc("/api/owner/command", requiresDB(h, ownerOnly(h, method("POST", h.OwnerCommand))))
	mux.HandleFunc("/api/owner/brain", requiresDB(h, ownerOnly(h, method("POST", h.OwnerBrain))))
	mux.HandleFunc("/api/owner/chat", requiresDB(h, ownerOnly(h, h.OwnerChat)))
	mux.HandleFunc("/api/owner/health", requiresDB(h, ownerOnly(h, method("GET", h.OwnerHealth))))
	mux.HandleFunc("/api/owner/status", requiresDB(h, ownerOnly(h, method("GET", h.OwnerStatus))))
	mux.HandleFunc("/owner", ownerPageHandler(staticDir))
	mux.HandleFunc("/owner.html", ownerPageHandler(staticDir))
}

func registerProductRoutes(mux *http.ServeMux, h *handlers.Handler, koschAccess func(http.HandlerFunc) http.HandlerFunc) {
	// Free core: no account or KOSCH balance required. This is intentionally
	// limited to deterministic read-only token fundamentals and public preflight.
	mux.HandleFunc("/api/token/scan", method("POST", h.TokenScan))

	// Premium: KOSCH unlocks deeper history, graph, exposure and automation.
	mux.HandleFunc("/api/v1/token/extensions", requiresDB(h, koschAccess(method("POST", h.TokenScan))))
	mux.HandleFunc("/api/v1/address-poisoning/check", requiresDB(h, koschAccess(method("POST", h.AddressPoisoningCheck))))
	mux.HandleFunc("/api/v1/risk/badge", method("GET", h.SecurityRiskBadge))
	mux.HandleFunc("/api/v1/radar/feed", requiresDB(h, koschAccess(method("GET", h.SecurityRadarFeed))))
	mux.HandleFunc("/api/v1/radar/check", requiresDB(h, koschAccess(method("POST", h.SecurityRadarCheck))))
	mux.HandleFunc("/api/v1/radar/detail", requiresDB(h, koschAccess(method("GET", h.SecurityRadarDetail))))
	mux.HandleFunc("/api/v1/radar/graph", requiresDB(h, koschAccess(method("GET", h.SecurityRadarGraph))))
	mux.HandleFunc("/api/v1/radar/exposure", requiresDB(h, koschAccess(method("GET", h.SecurityRadarExposureReport))))
}

func registerDeveloperAPIRoutes(mux *http.ServeMux, h *handlers.Handler, apiKey func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/api/v1/scan/token", requiresDB(h, apiKey(method("POST", h.B2BTokenScan))))
	mux.HandleFunc("/api/v1/usage", requiresDB(h, apiKey(method("GET", h.APIUsage))))
	mux.HandleFunc("/api/v1/shield/preflight", requiresDB(h, apiKey(method("POST", h.ShieldPreflight))))
	mux.HandleFunc("/api/v1/shield/transaction", requiresDB(h, apiKey(method("POST", h.ShieldPreflight))))
	mux.HandleFunc("/api/v1/shield/address-poisoning", requiresDB(h, apiKey(method("POST", h.AddressPoisoningCheck))))
}

func registerStatic(mux *http.ServeMux, staticDir string) {
	if staticDir == "" {
		return
	}
	info, err := os.Stat(staticDir)
	if err != nil || !info.IsDir() {
		log.Printf("warning: static directory unavailable at %q: %v", staticDir, err)
		return
	}
	fileServer := http.FileServer(http.Dir(staticDir))
	indexPath := filepath.Join(staticDir, "index.html")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/" {
			http.ServeFile(w, r, indexPath)
			return
		}
		clean := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		candidate := filepath.Join(staticDir, clean)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		if info, err := os.Stat(candidate + ".html"); err == nil && !info.IsDir() {
			http.ServeFile(w, r, candidate+".html")
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}
