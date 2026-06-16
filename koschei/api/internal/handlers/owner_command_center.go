package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"strings"

	"koschei/api/internal/services"
)

func (h *Handler) OwnerCommandCenterStatus(w http.ResponseWriter, r *http.Request) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	paddle := services.LoadPaddleConfigFromEnv()
	servicesMap := map[string]any{
		"database": map[string]any{"status": serviceStatus(db != nil, "connected", "unavailable")},
		"neon_auth": map[string]any{"status": serviceStatus(envSet("NEON_AUTH_JWKS_URL"), "configured", "missing")},
		"alchemy": map[string]any{"status": serviceStatus(envSet("ALCHEMY_API_KEY") || envSet("SOLANA_RPC_URL"), "configured", "missing")},
		"pumpportal": map[string]any{"status": pumpPortalStatus()},
		"raydium_api": map[string]any{"status": serviceStatus(envSet("RAYDIUM_API_BASE"), "configured", "missing")},
		"paddle": paddle.PublicStatus(),
		"security_radar_worker": map[string]any{"status": radarWorkerStatus(), "mode": firstOwnerEnv("KOSCHEI_SOLANA_WATCH_MODE", "polling"), "provider": firstOwnerEnv("KOSCHEI_SECURITY_PROVIDER", "alchemy")},
	}
	summary := map[string]any{
		"total_users": dbCount(r, db, `SELECT count(*) FROM app_user_profiles`),
		"active_entitlements": dbCount(r, db, `SELECT count(*) FROM entitlements WHERE status='active' AND (expires_at IS NULL OR expires_at > now())`),
		"pending_payments": dbCount(r, db, `SELECT count(*) FROM payment_requests WHERE status='pending'`),
		"radar_sources": dbCount(r, db, `SELECT count(*) FROM security_radar_sources`),
		"radar_findings_24h": dbCount(r, db, `SELECT count(*) FROM security_radar_verdicts WHERE created_at >= now() - interval '24 hours'`),
		"security_events_24h": dbCount(r, db, `SELECT count(*) FROM security_audit_events WHERE created_at >= now() - interval '24 hours'`),
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "services": servicesMap, "summary": summary})
}

func dbCount(r *http.Request, db *sql.DB, query string) int64 {
	if db == nil {
		return 0
	}
	var n int64
	_ = db.QueryRowContext(r.Context(), query).Scan(&n)
	return n
}

func envSet(name string) bool {
	return strings.TrimSpace(os.Getenv(name)) != ""
}

func serviceStatus(ok bool, good string, bad string) string {
	if ok {
		return good
	}
	return bad
}

func pumpPortalStatus() string {
	if !truthyOwnerEnv(os.Getenv("PUMPPORTAL_ENABLED")) {
		return "missing"
	}
	if envSet("PUMPPORTAL_DATA_WS") || envSet("PUMPPORTAL_API_KEY") {
		return "configured"
	}
	return "partial"
}

func radarWorkerStatus() string {
	if truthyOwnerEnv(os.Getenv("KOSCHEI_AUTO_RADAR_ENABLED")) || truthyOwnerEnv(os.Getenv("PUMPPORTAL_ENABLED")) {
		return "active"
	}
	return "unknown"
}

func truthyOwnerEnv(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func firstOwnerEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return "unknown"
}
