package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/services"
)

const (
	arvisHealthCacheTTL = 15 * time.Second
	publicHealthTimeout = 2 * time.Second
	evidenceHealthURL   = "/health?evidence=refresh"
)

// Collection must not hold the snapshot lock or queue concurrent public probes.
var arvisHealthRefresh sync.Mutex

var arvisHealthCache = struct {
	sync.RWMutex
	data      map[string]any
	expiresAt time.Time
}{}

// Health is the public liveness endpoint used by Railway and external
// transport monitors. Koschei Web3 is intentionally stateless for application
// blockchain/radar/evidence data, so PostgreSQL is not a readiness dependency.
// Customer indicators explicitly request a bounded observation refresh; plain
// /health never collects evidence or waits for the collection lock.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	arvis := cachedArvisHealthSnapshot()
	if r.Method == http.MethodGet && r.URL.Query().Get("evidence") == "refresh" {
		ctx, cancel := context.WithTimeout(r.Context(), publicHealthTimeout)
		defer cancel()
		arvis = h.cachedArvisHealth(ctx)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"persistence": "stateless",
		"database":    "not_used",
		"service":     "koschei-web3",
		"arvis":       arvis,
	})
}

func cachedArvisHealthSnapshot() map[string]any {
	arvisHealthCache.RLock()
	defer arvisHealthCache.RUnlock()
	if arvisHealthCache.data == nil {
		return map[string]any{
			"pipeline_status": "live_provider_mode",
			"details_url":     evidenceHealthURL,
			"cached":          false,
		}
	}
	out := make(map[string]any, len(arvisHealthCache.data)+3)
	for key, value := range arvisHealthCache.data {
		out[key] = value
	}
	out["cached"] = true
	out["cache_expires_at"] = arvisHealthCache.expiresAt.UTC().Format(time.RFC3339)
	out["details_url"] = evidenceHealthURL
	// A cached observation is not evidence of current pipeline availability once
	// its collection TTL has elapsed. Keep liveness cheap; do not collect here.
	if !time.Now().Before(arvisHealthCache.expiresAt) {
		out["pipeline_status"] = "stale"
	}
	return out
}

func (h *Handler) cachedArvisHealth(ctx context.Context) map[string]any {
	arvisHealthCache.RLock()
	fresh := arvisHealthCache.data != nil && time.Now().Before(arvisHealthCache.expiresAt)
	arvisHealthCache.RUnlock()
	if fresh || !arvisHealthRefresh.TryLock() {
		return cachedArvisHealthSnapshot()
	}
	defer arvisHealthRefresh.Unlock()
	arvisHealthCache.RLock()
	fresh = arvisHealthCache.data != nil && time.Now().Before(arvisHealthCache.expiresAt)
	arvisHealthCache.RUnlock()
	if fresh {
		return cachedArvisHealthSnapshot()
	}

	stats := h.securityRadarStreamStats(ctx)
	data := map[string]any{
		"pipeline_status":         measuredArvisHealthStatus(stats),
		"architecture_arm_count":  stats["architecture_arm_count"],
		"runtime_engines":         stats["runtime_engines"],
		"raw_stream_events":       stats["raw_stream_events"],
		"recognized_events":       stats["recognized_events"],
		"enriched_mints":          stats["enriched_mints"],
		"visible_verdicts":        stats["visible_verdicts"],
		"signed_verdicts_total":   stats["signed_verdicts_total"],
		"processing_active":       stats["processing_active"],
		"processing_completed":    stats["processing_completed"],
		"processing_insufficient": stats["processing_insufficient"],
		"processing_failed":       stats["processing_failed"],
		"last_stream_event_at":    stats["last_stream_event_at"],
		"last_processed_at":       stats["last_processed_at"],
		"runtime_window_minutes":  stats["runtime_window_minutes"],
		"sources":                 h.arvisSourceHealth(ctx),
		"failures":                h.arvisFailureHealth(ctx),
		"cached_for_seconds":      int(arvisHealthCacheTTL.Seconds()),
	}
	if ctx.Err() != nil {
		// A cancelled or incomplete refresh must not publish a healthy snapshot.
		return map[string]any{"pipeline_status": "unavailable", "cached": false, "details_url": evidenceHealthURL}
	}
	arvisHealthCache.Lock()
	arvisHealthCache.data = data
	arvisHealthCache.expiresAt = time.Now().Add(arvisHealthCacheTTL)
	arvisHealthCache.Unlock()
	return cachedArvisHealthSnapshot()
}

func measuredArvisHealthStatus(stats map[string]any) string {
	status, _ := stats["pipeline_status"].(string)
	if status == "" {
		return "unverified"
	}
	if status != "healthy" {
		return status
	}
	// The collector omits failed SQL observations. Missing failure/lease metrics
	// cannot count as zero when deciding whether a pipeline is healthy.
	for _, key := range []string{
		"raw_stream_events", "enriched_mints", "processing_active",
		"processing_stale_active", "processing_completed",
		"processing_completed_recent", "processing_failed_recent",
		"last_stream_event_at", "last_processed_at",
	} {
		if _, observed := stats[key]; !observed {
			return "unverified"
		}
	}
	return status
}

func resetArvisHealthCache() {
	arvisHealthCache.Lock()
	arvisHealthCache.data = nil
	arvisHealthCache.expiresAt = time.Time{}
	arvisHealthCache.Unlock()
}

func (h *Handler) arvisSourceHealth(ctx context.Context) map[string]any {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		return map[string]any{}
	}
	return map[string]any{
		"pump":    arvisModuleSourceHealth(ctx, db, services.ModulePumpSybilRadar),
		"raydium": arvisModuleSourceHealth(ctx, db, services.ModuleRaydiumPoolGuardian),
	}
}

func (h *Handler) arvisFailureHealth(ctx context.Context) map[string]any {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	out := map[string]any{
		"retryable":         int64(0),
		"exhausted":         int64(0),
		"recent_15_minutes": int64(0),
		"latest_error_code": "",
		"latest_error_at":   "",
		"reason_counts":     map[string]int64{},
	}
	if db == nil {
		return out
	}

	var retryable, exhausted, recent int64
	var latestAt string
	if err := db.QueryRowContext(ctx, `
		SELECT
			count(*) FILTER (WHERE status='failed' AND attempts < 3),
			count(*) FILTER (WHERE status='exhausted' OR (status='failed' AND attempts >= 3)),
			count(*) FILTER (WHERE status='failed' AND attempts < 3 AND updated_at > now() - interval '15 minutes'),
			COALESCE(max(updated_at) FILTER (WHERE status IN ('failed','exhausted'))::text,'')
		FROM arvis_stream_processing
	`).Scan(&retryable, &exhausted, &recent, &latestAt); err == nil {
		out["retryable"] = retryable
		out["exhausted"] = exhausted
		out["recent_15_minutes"] = recent
		out["latest_error_at"] = latestAt
	}

	rows, err := db.QueryContext(ctx, `
		SELECT COALESCE(last_error,''), updated_at::text
		FROM arvis_stream_processing
		WHERE status IN ('failed','exhausted')
		ORDER BY updated_at DESC
		LIMIT 200
	`)
	if err != nil {
		return out
	}
	defer rows.Close()
	counts := map[string]int64{}
	latestCode := ""
	for rows.Next() {
		var raw, updatedAt string
		if rows.Scan(&raw, &updatedAt) != nil {
			continue
		}
		code := classifyArvisFailure(raw)
		counts[code]++
		if latestCode == "" {
			latestCode = code
		}
	}
	out["reason_counts"] = counts
	out["latest_error_code"] = latestCode
	return out
}

func classifyArvisFailure(raw string) string {
	text := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case text == "":
		return "unknown"
	case strings.Contains(text, "duplicate key") || strings.Contains(text, "23505") || strings.Contains(text, "unique constraint"):
		return "duplicate_write"
	case strings.Contains(text, "foreign key") || strings.Contains(text, "23503"):
		return "foreign_key"
	case strings.Contains(text, "null value") || strings.Contains(text, "23502") || strings.Contains(text, "check constraint") || strings.Contains(text, "23514"):
		return "schema_constraint"
	case strings.Contains(text, "does not exist") || strings.Contains(text, "undefined table") || strings.Contains(text, "undefined column") || strings.Contains(text, "42p01") || strings.Contains(text, "42703"):
		return "missing_schema"
	case strings.Contains(text, "invalid input syntax for type json") || strings.Contains(text, "unsupported value") || strings.Contains(text, "json"):
		return "json_encoding"
	case strings.Contains(text, "deadline exceeded") || strings.Contains(text, "timeout") || strings.Contains(text, "timed out"):
		return "timeout"
	case strings.Contains(text, "connection") || strings.Contains(text, "network") || strings.Contains(text, "broken pipe") || strings.Contains(text, "eof"):
		return "database_or_network"
	case strings.Contains(text, "insert arm verdict"):
		return "verdict_insert"
	case strings.Contains(text, "insert arm event"):
		return "event_insert"
	case strings.Contains(text, "check existing arm verdict"):
		return "idempotency_check"
	default:
		return "processing_error"
	}
}

func arvisModuleSourceHealth(ctx context.Context, db *sql.DB, moduleID string) map[string]any {
	out := map[string]any{
		"module_id":     moduleID,
		"events":        int64(0),
		"recent":        int64(0),
		"enriched":      int64(0),
		"last_event_at": "",
	}
	if db == nil || moduleID == "" {
		return out
	}
	count := func(key, query string) {
		var value int64
		if err := db.QueryRowContext(ctx, query, moduleID).Scan(&value); err == nil {
			out[key] = value
		}
	}
	count("events", `SELECT count(*) FROM security_radar_stream_events WHERE module_id=$1`)
	count("recent", `SELECT count(*) FROM security_radar_stream_events WHERE module_id=$1 AND created_at > now() - interval '15 minutes'`)
	count("enriched", `SELECT count(*) FROM security_radar_stream_events WHERE module_id=$1 AND evidence_quality='transaction_enriched_mint'`)
	var lastEvent string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(created_at)::text,'') FROM security_radar_stream_events WHERE module_id=$1`, moduleID).Scan(&lastEvent); err == nil {
		out["last_event_at"] = lastEvent
	}
	return out
}
