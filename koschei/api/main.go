package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"koschei/api/internal/cache"
	appdb "koschei/api/internal/db"
	"koschei/api/internal/handlers"
	apihttp "koschei/api/internal/http"
	"koschei/api/internal/jobs"
	"koschei/api/internal/services"
	"koschei/api/internal/web3"
)

const (
	httpReadHeaderTimeout = 5 * time.Second
	httpReadTimeout       = 15 * time.Second
	httpWriteTimeout      = 10 * time.Minute
	httpIdleTimeout       = 60 * time.Second
	httpShutdownTimeout   = 15 * time.Second
)

func main() {
	log.Printf("koschei api starting")
	if missing := services.MissingProductionSecurityEnv(); len(missing) > 0 {
		log.Fatalf("CRITICAL: missing required production security env vars: %s", strings.Join(missing, ", "))
	}

	appCtx, stopApp := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopApp()

	// The Web3 runtime remains stateless by default. Durable application
	// persistence is opt-in through APP_DATABASE_URL so deployments that need
	// account history, watchlists, jobs, webhooks and owner DB-backed operations
	// can restore those capabilities without silently reusing legacy DATABASE_URL.
	appDB, appReadDB, dbInitError, err := buildApplicationStores()
	if err != nil {
		log.Fatalf("CRITICAL: configured application persistence is unavailable: %v", err)
	}
	if appDB != nil {
		defer appDB.Close()
		if appReadDB != nil && appReadDB != appDB {
			defer appReadDB.Close()
		}
		log.Printf("application persistence connected: DB-backed customer and owner capabilities available")
	} else {
		log.Printf("stateless Web3 runtime enabled: APP_DATABASE_URL is not set")
	}

	entitlementDB, err := buildEntitlementStore()
	if err != nil {
		log.Fatalf("CRITICAL: configured entitlement ledger is unavailable: %v", err)
	}
	if entitlementDB != nil {
		defer entitlementDB.Close()
		log.Printf("entitlement ledger connected: paid-access enforcement available")
	} else {
		log.Printf("ENTITLEMENT_DATABASE_URL is not set: paid customer operations remain fail-closed")
	}

	appCache := buildCache()
	defer appCache.Close()
	solanaRPC := web3.NewSolanaRPC(appCache)
	log.Printf("solana rpc primary=%s fallback=%s",
		web3.RPCProviderHost(solanaRPC.URL("solana-mainnet")),
		web3.RPCProviderHost(web3.SolanaRPCFallbackURL("solana-mainnet")),
	)

	jobStore := jobs.NewStore(appDB)
	jobQueue := jobs.Queue(jobs.NoopQueue{})
	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		jobQueue = jobs.NewNATSQueue(natsURL, os.Getenv("NATS_SUBJECT_PREFIX"))
	}
	defer jobQueue.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if handlers.ConfiguredNeonAuthJWKSURL() == "" {
		log.Printf("NEON_AUTH_JWKS_URL is not set")
	}
	staticDir := resolveStaticDir(os.Getenv("STATIC_DIR"))
	log.Printf("static public path: %s", staticDir)
	handler := englishPublicHTML(apihttp.MountFabric(apihttp.NewServer(
		appDB,
		dbInitError,
		os.Getenv("ADMIN_PASSWORD"),
		firstEnv("CORS_ORIGIN", "CORS_ALLOWED_ORIGIN"),
		staticDir,
		apihttp.WithReadDB(appReadDB),
		apihttp.WithEntitlementDB(entitlementDB),
		apihttp.WithCache(appCache),
		apihttp.WithSolanaRPC(solanaRPC),
		apihttp.WithJobStore(jobStore),
		apihttp.WithJobQueue(jobQueue),
	)))
	server := newHTTPServer(port, handler)

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("api listening on %s", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	case <-appCtx.Done():
		log.Printf("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
			_ = server.Close()
		}
		select {
		case err := <-serverErrors:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("http server stopped with error: %v", err)
			}
		case <-shutdownCtx.Done():
			log.Printf("http server shutdown deadline reached")
		}
	}
}

func buildApplicationStores() (*sql.DB, *sql.DB, string, error) {
	writeURL := strings.TrimSpace(os.Getenv("APP_DATABASE_URL"))
	if writeURL == "" {
		return nil, nil, "application persistence disabled: APP_DATABASE_URL is not configured", nil
	}

	writeDB, err := appdb.Connect(writeURL)
	if err != nil {
		return nil, nil, "application persistence unavailable", err
	}

	readURL := strings.TrimSpace(os.Getenv("APP_DATABASE_READ_URL"))
	if readURL == "" || readURL == writeURL {
		return writeDB, writeDB, "", nil
	}
	readDB, err := appdb.ConnectReplica(readURL)
	if err != nil {
		_ = writeDB.Close()
		return nil, nil, "application read persistence unavailable", err
	}
	return writeDB, readDB, "", nil
}

func buildEntitlementStore() (*sql.DB, error) {
	databaseURL := strings.TrimSpace(os.Getenv("ENTITLEMENT_DATABASE_URL"))
	if databaseURL == "" {
		return nil, nil
	}
	return appdb.ConnectEntitlementStore(databaseURL)
}

func newHTTPServer(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + strings.TrimSpace(port),
		Handler:           handler,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func resolveStaticDir(configured string) string {
	if configured != "" {
		return configured
	}
	for _, candidate := range []string{"public", filepath.Join("/app", "public"), filepath.Join("koschei", "api", "public")} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join("koschei", "api", "public")
}

func buildCache() cache.Cache {
	if os.Getenv("CACHE_ENABLED") == "false" {
		return cache.NewNoop()
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return cache.NewMemory()
	}
	redisCache, err := cache.NewRedis(redisURL, os.Getenv("REDIS_TLS") == "true")
	if err != nil {
		log.Printf("redis cache unavailable, using in-memory cache: %v", err)
		return cache.NewMemory()
	}
	return redisCache
}
