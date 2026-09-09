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
	"koschei/api/internal/runtimecfg"
	"koschei/api/internal/web3"
)

type serverConfig struct {
	dbRead        *sql.DB
	entitlementDB *sql.DB
	cache         cache.Cache
	solanaRPC     *web3.SolanaRPC
	jobStore      *jobs.Store
	jobQueue      jobs.Queue
}

type Option func(*serverConfig)

type routeGate func(http.HandlerFunc) http.HandlerFunc
type tierRouteGate func(string, http.HandlerFunc) http.HandlerFunc

func WithReadDB(db *sql.DB) Option { return func(c *serverConfig) { c.dbRead = db } }
func WithEntitlementDB(db *sql.DB) Option {
	return func(c *serverConfig) { c.entitlementDB = db }
}
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
	if config.entitlementDB == nil {
		config.entitlementDB = db
	}
	if config.solanaRPC == nil {
		config.solanaRPC = web3.NewSolanaRPC(config.cache)
	}
	h := &handlers.Handler{DB: db, DBRead: config.dbRead, EntitlementDB: config.entitlementDB, AdminPassword: adminPassword, Limiter: handlers.NewLimiter(), DBInitError: dbInitError, Cache: config.cache, SolanaRPC: config.solanaRPC, JobStore: config.jobStore, JobQueue: config.jobQueue, CourtClient: handlers.NewCourtNarrativeClientFromEnv()}
	mux := http.NewServeMux()

	planTierAccess := func(plan string, next http.HandlerFunc) http.HandlerFunc {
		return handlers.RequireAuth(h.RequirePlanTier(plan, next))
	}
	planTier := func(plan string, next http.HandlerFunc) http.HandlerFunc {
		return handlers.RequireAuth(h.RequirePlanTier(plan, h.EnforcePlanOutput(next)))
	}
	planAccess := func(next http.HandlerFunc) http.HandlerFunc { return planTier("professional", next) }
	apiKeyProfessional := func(next http.HandlerFunc) http.HandlerFunc {
		return h.APIKeyAuth(h.RequireAPIKeyPlanTier("professional", h.APIRateLimit(next)))
	}
	// Developer APIs already have API-key monthly/RPM accounting. Do not charge
	// the customer entitlement output ledger a second time for the same request.
	apiKeyProfessionalMetered := func(next http.HandlerFunc) http.HandlerFunc {
		return h.APIKeyAuth(h.RequireAPIKeyPlanTier("professional", h.APIRateLimit(next)))
	}

	registerCoreRoutes(mux, h, planAccess)
	registerAccountRoutes(mux, h, planTierAccess)
	registerOwnerRoutes(mux, h, staticDir)
	registerDefenseOSRoutes(mux, h)
	registerProductRoutes(mux, h, planTier, planTierAccess)
	registerDeveloperAPIRoutes(mux, h, apiKeyProfessional, apiKeyProfessionalMetered)
	registerDossierRoutes(mux, h)
	registerBillingRoutes(mux, h)
	registerWatchlistRoutes(mux, h, func(next http.HandlerFunc) http.HandlerFunc { return planTier("professional", next) }, func(next http.HandlerFunc) http.HandlerFunc { return planTierAccess("professional", next) })
	registerStatic(mux, staticDir)

	return securityHeaders(cors(apiReadiness(db, mux), corsOrigin))
}

func registerCoreRoutes(mux *http.ServeMux, h *handlers.Handler, planAccess routeGate) {
	mux.HandleFunc("/health", method("GET", h.Health))
	mux.HandleFunc("/api/version", method("GET", h.APIVersion))
	mux.HandleFunc("/api/config", method("GET", h.PublicConfig))
	mux.HandleFunc("/api/me", handlers.RequireAuth(method("GET", h.Me)))
	mux.HandleFunc("/api/auth/register", method("POST", h.Register))
	mux.HandleFunc("/api/auth/login", method("POST", h.Login))
	mux.HandleFunc("/api/auth/provision", handlers.RequireAuth(method("POST", h.Provision)))
	mux.HandleFunc("/api/auth/neon-login", method("POST", h.NeonLogin))
	mux.HandleFunc("/api/auth/neon-register", method("POST", h.NeonRegister))
	mux.HandleFunc("/api/auth/neon-callback", method("POST", h.NeonCallback))
	mux.HandleFunc("/api/public/impact", method("GET", h.PublicImpact))
	mux.HandleFunc("/api/public/metrics", method("GET", h.PublicMetrics))
	mux.HandleFunc("/api/public/cases", method("GET", h.PublicCases))
	mux.HandleFunc("/api/public/token/status", method("GET", h.PublicTokenStatus))
	mux.HandleFunc("/api/public/token/readiness", method("GET", h.PublicTokenReadiness))
	mux.HandleFunc("/api/web3/health", method("GET", h.Web3Health))
	mux.HandleFunc("/api/analytics/event", method("POST", h.AnalyticsEvent))
	mux.HandleFunc("/api/premium/access", handlers.RequireAuth(method("GET", h.PremiumAccessStatus)))
	mux.HandleFunc("/api/premium/output", handlers.RequireAuth(planAccess(method("POST", h.PremiumOutput))))
}

func registerAccountRoutes(mux *http.ServeMux, h *handlers.Handler, planTierAccess tierRouteGate) {
	mux.HandleFunc("/api/account", handlers.RequireAuth(method("GET", h.Account)))
	mux.HandleFunc("/api/account/profile", handlers.RequireAuth(h.AccountProfile))
	mux.HandleFunc("/api/account/wallet", handlers.RequireAuth(h.AccountWallet))
	mux.HandleFunc("/api/account/plan", handlers.RequireAuth(method("GET", h.AccountPlan)))
	mux.HandleFunc("/api/account/history", planTierAccess("professional", method("GET", h.CustomerInvestigationHistory)))
	mux.HandleFunc("/api/account/dossiers", planTierAccess("professional", method("GET", h.CustomerDossiers)))
	mux.HandleFunc("/api/account/api-keys", planTierAccess("professional", h.APIKeysCollection))
	mux.HandleFunc("/api/account/api-keys/", planTierAccess("professional", method("POST", h.RevokeAPIKey)))
}

func registerOwnerRoutes(mux *http.ServeMux, h *handlers.Handler, staticDir string) {
	mux.HandleFunc("/owner", ownerPageHandler(staticDir))
	mux.HandleFunc("/api/owner/login", method("POST", h.OwnerLogin))
	mux.HandleFunc("/api/owner/logout", method("POST", h.OwnerLogout))
	mux.HandleFunc("/api/owner/command-center", ownerOnly(h, method("GET", h.OwnerCommandCenter)))
	mux.HandleFunc("/api/owner/operations", ownerOnly(h, method("GET", h.OwnerOperations)))
	mux.HandleFunc("/api/owner/arvis", ownerOnly(h, method("GET", h.OwnerArvis)))
	mux.HandleFunc("/api/owner/arvis/scan", ownerOnly(h, method("POST", h.OwnerArvisScan)))
	mux.HandleFunc("/api/owner/radar/unified", ownerOnly(h, method("POST", h.OwnerRadarUnified)))
	mux.HandleFunc("/api/owner/web3/shield/preflight", ownerOnly(h, method("POST", h.ShieldPreflight)))
	mux.HandleFunc("/api/owner/web3/transaction-guard", ownerOnly(h, method("POST", h.TransactionGuardV2Configured)))
	mux.HandleFunc("/api/owner/web3/transaction-guard/state-recheck", ownerOnly(h, method("POST", h.TransactionGuardStateRecheck)))
	mux.HandleFunc("/api/owner/web3/address-poisoning/check", ownerOnly(h, method("POST", h.AddressPoisoningCheck)))
	mux.HandleFunc("/api/owner/web3/defense-validation", ownerOnly(h, method("POST", h.DefenseValidationV1)))
	mux.HandleFunc("/api/owner/web3/execution-assurance/safe/verify", ownerOnly(h, method("POST", h.SafeExecutionAssuranceV1)))
	mux.HandleFunc("/api/owner/radar/jobs", ownerOnly(h, requiresDB(h, method("POST", h.CreateWeb3Job))))
}

func registerDefenseOSRoutes(mux *http.ServeMux, h *handlers.Handler) {
	mux.HandleFunc("/api/defense-os/health", method("GET", h.DefenseOSHealth))
	mux.HandleFunc("/api/defense-os/evidence", method("GET", h.DefenseOSEvidence))
}

func registerProductRoutes(mux *http.ServeMux, h *handlers.Handler, planTier tierRouteGate, planTierAccess tierRouteGate) {
	solana := func(next http.HandlerFunc) http.HandlerFunc { return requireRuntimeFeature(featureSolana, next) }
	risk := func(next http.HandlerFunc) http.HandlerFunc { return requireRuntimeFeature(featureRiskScanner, next) }
	mux.HandleFunc("/api/arvis/preflight", solana(method("POST", h.ARVISPreflight)))
	mux.HandleFunc("/api/token/scan", solana(risk(method("POST", h.TokenScan))))
	mux.HandleFunc("/api/v1/risk/badge", solana(badge(method("GET", h.SecurityRiskBadge))))
	mux.HandleFunc("/api/v1/token/extensions", solana(risk(requiresDB(h, planTier("professional", method("POST", h.TokenScan))))))
	mux.HandleFunc("/api/v1/address-poisoning/check", solana(requiresDB(h, planTier("professional", method("POST", h.AddressPoisoningCheck)))))
	mux.HandleFunc("/api/v1/radar/check", solana(requiresDB(h, planTier("professional", method("POST", h.SecurityRadarCheckWithAlerts)))))
	mux.HandleFunc("/api/v1/radar/jobs", solana(requiresDB(h, planTier("professional", method("POST", h.CreateWeb3Job)))))
	mux.HandleFunc("/api/v1/radar/jobs/", solana(requiresDB(h, handlers.RequireAuth(method("GET", h.GetWeb3Job)))))
	mux.HandleFunc("/api/jobs/token-scan", solana(risk(requiresDB(h, planTier("professional", method("POST", h.CreateWeb3Job))))))
	mux.HandleFunc("/api/jobs/", solana(requiresDB(h, handlers.RequireAuth(method("GET", h.GetWeb3Job)))))
	mux.HandleFunc("/api/v1/radar/detail", solana(requiresDB(h, planTier("professional", method("GET", h.SecurityRadarDetailV3)))))
	mux.HandleFunc("/api/customer/web3/transaction-preflight", solana(requiresDB(h, planTier("professional", method("POST", h.TransactionGuardV2Configured)))))
	mux.HandleFunc("/api/customer/web3/transaction-state-recheck", solana(requiresDB(h, planTierAccess("professional", customerStateRecheckRateLimit(h.DB, method("POST", h.TransactionGuardStateRecheck))))))
	mux.HandleFunc("/api/v1/radar/feed", solana(requiresDB(h, planTier("professional", method("GET", h.SecurityRadarFeed)))))
	mux.HandleFunc("/api/v1/radar/creator-intelligence", solana(requiresDB(h, planTier("professional", method("GET", h.OwnerCreatorIntelligence)))))
	mux.HandleFunc("/api/v1/radar/actor-intelligence", solana(requiresDB(h, planTier("professional", method("GET", h.OwnerActorSecurityIntelligence)))))
	mux.HandleFunc("/api/v1/radar/graph", solana(requiresDB(h, planTier("professional", method("GET", h.SecurityRadarGraph)))))
	mux.HandleFunc("/api/v1/radar/exposure", solana(requiresDB(h, planTier("professional", method("GET", h.SecurityRadarExposureReport)))))
	mux.HandleFunc("/api/v1/radar/court", solana(requiresDB(h, planTier("professional", method("POST", h.SecurityRadarCourt)))))
}

func registerDeveloperAPIRoutes(mux *http.ServeMux, h *handlers.Handler, professional routeGate, professionalMetered routeGate) {
	solana := func(next http.HandlerFunc) http.HandlerFunc { return requireRuntimeFeature(featureSolana, next) }
	risk := func(next http.HandlerFunc) http.HandlerFunc { return requireRuntimeFeature(featureRiskScanner, next) }
	mux.HandleFunc("/api/v1/scan/token", solana(risk(requiresDB(h, professionalMetered(method("POST", h.B2BTokenScan))))))
	mux.HandleFunc("/api/v1/usage", requiresDB(h, professional(method("GET", h.APIUsage))))
	mux.HandleFunc("/api/v1/shield/preflight", solana(requiresDB(h, professionalMetered(method("POST", h.ShieldPreflight)))))
	mux.HandleFunc("/api/v1/shield/transaction", solana(requiresDB(h, professionalMetered(method("POST", h.TransactionGuardV2Configured)))))
	mux.HandleFunc("/api/v1/shield/state-recheck", solana(requiresDB(h, professional(method("POST", h.TransactionGuardStateRecheck)))))
	mux.HandleFunc("/api/v1/shield/address-poisoning", solana(requiresDB(h, professionalMetered(method("POST", h.AddressPoisoningCheck)))))
	mux.HandleFunc("/api/v1/defense/validation", requiresDB(h, professionalMetered(method("POST", h.DefenseValidationV1))))
	mux.HandleFunc("/api/v1/execution-assurance/safe/verify", requiresDB(h, professionalMetered(method("POST", h.SafeExecutionAssuranceV1))))
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
	registerStaticAliases(mux, staticDir)
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
		if (r.URL.Path == "/launches" || r.URL.Path == "/launches.html") && !runtimeFeatureEnabled(featureLaunchPageBuilder) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "code": "feature_disabled", "feature": string(featureLaunchPageBuilder)})
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
