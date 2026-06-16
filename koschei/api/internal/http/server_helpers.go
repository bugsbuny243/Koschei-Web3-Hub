package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"koschei/api/internal/handlers"
)

func apiReadiness(db *sql.DB, next http.Handler) http.Handler {
	setSecurityAuditDB(db)
	protected := bodyLimit(sensitiveRateLimit(next))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") && path != "/api/me" && path != "/api/me/package" && path != "/api/v1/unified/analyze" && path != "/api/v1/risk/badge" && path != "/api/version" && path != "/api/config" && path != "/api/auth/register" && path != "/api/auth/login" && path != "/api/auth/provision" && path != "/api/auth/neon-login" && path != "/api/auth/neon-register" && path != "/api/auth/neon-callback" && path != "/api/owner/login" && path != "/api/public/impact" && path != "/api/public/metrics" && path != "/api/web3/health" && path != "/api/analytics/event" && db == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "database unavailable"})
			return
		}
		protected.ServeHTTP(w, r)
	})
}

func registerLegacyDashboardRedirects(mux *http.ServeMux) {
	legacyDashboards := []string{
		"/airdrop-checker", "/cross-chain-risk", "/funding-assistant", "/grant", "/grant-writer", "/graph", "/hub", "/intelligence-graph", "/liquidity-radar", "/mev-shield", "/portfolio", "/program-scanner", "/project-radar", "/radar", "/risk", "/risk-v2", "/smart-money", "/solana-risk-scanner", "/solana-token-scanner", "/solana-tx-decoder", "/sybil-check", "/sybil-checker", "/token-scanner", "/tx-decoder", "/tx-decoder-pro", "/unified", "/wallet-score",
	}
	for _, route := range legacyDashboards {
		mux.HandleFunc(route, redirectToDashboard)
		mux.HandleFunc(route+".html", redirectToDashboard)
	}
}

func redirectToDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func ownerPageHandler(staticDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if staticDir != "" {
			ownerPath := filepath.Join(staticDir, "owner.html")
			if info, err := os.Stat(ownerPath); err == nil && !info.IsDir() {
				http.ServeFile(w, r, ownerPath)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Koschei Owner</title><style>body{margin:0;min-height:100vh;background:#070711;color:#f5f7fb;font-family:Inter,system-ui,sans-serif;display:grid;place-items:center}.card{max-width:720px;padding:32px;border:1px solid rgba(255,255,255,.14);border-radius:24px;background:linear-gradient(180deg,rgba(255,255,255,.08),rgba(255,255,255,.03));box-shadow:0 28px 80px rgba(0,0,0,.35)}h1{margin:0 0 12px;font-size:clamp(30px,5vw,54px)}p{color:#a7b0c2;line-height:1.65}.pill{display:inline-block;color:#00ffaa;border:1px solid rgba(0,255,170,.4);border-radius:999px;padding:8px 12px;font-weight:800}</style></head><body><main class="card"><span class="pill">Owner login</span><h1>Koschei Owner Dashboard</h1><p>Statik owner paneli bulunamadı. Owner API uçları korumalı kalır; giriş formunu sunmak için /owner sayfası kimlik doğrulaması istemeden servis edilir.</p></main></body></html>`))
	}
}

func ownerOnly(h *handlers.Handler, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.OwnerAuth(w, r) {
			return
		}
		next(w, r)
	}
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
	allowedOrigins := buildAllowedOrigins(origin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowed := allowedCORSOrigin(r.Header.Get("Origin"), allowedOrigins); allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-admin-password, Authorization, X-API-Key, X-Koschei-Source-Id, x-koschei-agent-key, X-Koschei-Secret, X-Owner-Secret, X-Owner-Wallet, X-CSRF-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func buildAllowedOrigins(configured string) map[string]struct{} {
	origins := map[string]struct{}{"https://tradepigloball.co": {}, "https://www.tradepigloball.co": {}, "http://tradepigloball.co": {}, "http://www.tradepigloball.co": {}}
	for _, item := range strings.Split(configured, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			origins[strings.TrimRight(item, "/")] = struct{}{}
		}
	}
	return origins
}
