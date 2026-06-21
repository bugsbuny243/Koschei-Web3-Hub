package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/services"
)

func (h *Handler) OwnerCommandCenterStatus(w http.ResponseWriter, r *http.Request) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}

	paddle := services.LoadPaddleConfigFromEnv()
	arvis := h.securityRadarStreamStats(r.Context())
	arvis["sources"] = h.arvisSourceHealth(r.Context())
	arvis["failures"] = h.arvisFailureHealth(r.Context())

	servicesMap := map[string]any{
		"database": map[string]any{"status": serviceStatus(db != nil, "connected", "unavailable")},
		"neon_auth": map[string]any{"status": serviceStatus(envSet("NEON_AUTH_JWKS_URL"), "configured", "missing")},
		"solana_rpc": map[string]any{"status": serviceStatus(envSet("SOLANA_RPC_URL") || envSet("ALCHEMY_SOLANA_RPC_URL") || envSet("HELIUS_SOLANA_RPC_URL") || envSet("QUICKNODE_SOLANA_RPC_URL") || envSet("ALCHEMY_API_KEY"), "configured", "missing")},
		"paddle": paddle.PublicStatus(),
		"shopier": map[string]any{"status": serviceStatus(envSet("SHOPIER_WEBHOOK_SECRET"), "configured", "manual")},
		"security_radar": map[string]any{"status": firstMapString(arvis, "pipeline_status"), "mode": firstOwnerEnv("KOSCHEI_SOLANA_WATCH_MODE", "stream")},
	}

	summary := h.ownerBusinessSummary(r.Context(), db)
	trends := map[string]any{
		"users_7d":  ownerDailyUserTrend(r.Context(), db),
		"orders_7d": ownerDailyOrderTrend(r.Context(), db),
	}
	actions := ownerActionQueue(summary, servicesMap, arvis)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"generated_at": time.Now().UTC(),
		"summary":      summary,
		"services":     servicesMap,
		"arvis":        arvis,
		"trends":       trends,
		"action_queue": actions,
	})
}

func (h *Handler) ownerBusinessSummary(ctx context.Context, db *sql.DB) map[string]any {
	summary := map[string]any{
		"total_users":                  int64(0),
		"active_users":                 int64(0),
		"banned_users":                 int64(0),
		"new_users_24h":                int64(0),
		"new_users_7d":                 int64(0),
		"active_entitlements":          int64(0),
		"expiring_entitlements_7d":     int64(0),
		"pending_payments":             int64(0),
		"paid_orders_30d":              int64(0),
		"revenue_try_cents_30d":        int64(0),
		"open_feedback":                int64(0),
		"security_feedback":            int64(0),
		"radar_findings_24h":           int64(0),
		"risk_cards_24h":               int64(0),
		"monitor_cards_24h":            int64(0),
		"security_events_24h":          int64(0),
		"critical_security_events_24h": int64(0),
		"failed_jobs_24h":              int64(0),
	}
	if db == nil {
		return summary
	}

	if ownerTableExists(ctx, db, "app_user_profiles") {
		summary["total_users"] = ownerCount(ctx, db, `SELECT count(*) FROM app_user_profiles`)
		summary["active_users"] = ownerCount(ctx, db, `SELECT count(*) FROM app_user_profiles WHERE COALESCE(status,'active')='active'`)
		summary["banned_users"] = ownerCount(ctx, db, `SELECT count(*) FROM app_user_profiles WHERE COALESCE(status,'active')='banned'`)
		summary["new_users_24h"] = ownerCount(ctx, db, `SELECT count(*) FROM app_user_profiles WHERE created_at >= now() - interval '24 hours'`)
		summary["new_users_7d"] = ownerCount(ctx, db, `SELECT count(*) FROM app_user_profiles WHERE created_at >= now() - interval '7 days'`)
	}
	if ownerTableExists(ctx, db, "entitlements") {
		summary["active_entitlements"] = ownerCount(ctx, db, `SELECT count(*) FROM entitlements WHERE status='active' AND (expires_at IS NULL OR expires_at > now())`)
		summary["expiring_entitlements_7d"] = ownerCount(ctx, db, `SELECT count(*) FROM entitlements WHERE status='active' AND expires_at > now() AND expires_at <= now() + interval '7 days'`)
	}
	if ownerTableExists(ctx, db, "payment_requests") {
		summary["pending_payments"] = ownerCount(ctx, db, `SELECT count(*) FROM payment_requests WHERE status='pending'`)
	}
	if ownerTableExists(ctx, db, "orders") {
		var paidOrders, revenue int64
		_ = db.QueryRowContext(ctx, `
			SELECT count(*), COALESCE(sum(amount_try_cents),0)
			FROM orders
			WHERE created_at >= now() - interval '30 days'
			  AND lower(COALESCE(status,'')) IN ('paid','completed','success','active')
		`).Scan(&paidOrders, &revenue)
		summary["paid_orders_30d"] = paidOrders
		summary["revenue_try_cents_30d"] = revenue
	}
	if ownerTableExists(ctx, db, "customer_feedback") {
		summary["open_feedback"] = ownerCount(ctx, db, `SELECT count(*) FROM customer_feedback WHERE status IN ('new','reviewing','planned')`)
		summary["security_feedback"] = ownerCount(ctx, db, `SELECT count(*) FROM customer_feedback WHERE category='security' AND status IN ('new','reviewing')`)
	}
	if ownerTableExists(ctx, db, "security_radar_verdicts") {
		summary["radar_findings_24h"] = ownerCount(ctx, db, `SELECT count(*) FROM security_radar_verdicts WHERE created_at >= now() - interval '24 hours' AND module_id='final_verdict_engine' AND signed=true`)
		summary["risk_cards_24h"] = ownerCount(ctx, db, `SELECT count(*) FROM security_radar_verdicts WHERE created_at >= now() - interval '24 hours' AND module_id='final_verdict_engine' AND signed=true AND lower(COALESCE(risk_level,'')) IN ('high','critical')`)
		summary["monitor_cards_24h"] = ownerCount(ctx, db, `SELECT count(*) FROM security_radar_verdicts WHERE created_at >= now() - interval '24 hours' AND module_id='final_verdict_engine' AND signed=true AND lower(COALESCE(risk_level,'')) NOT IN ('high','critical')`)
	}
	if ownerTableExists(ctx, db, "security_audit_events") {
		summary["security_events_24h"] = ownerCount(ctx, db, `SELECT count(*) FROM security_audit_events WHERE created_at >= now() - interval '24 hours'`)
		summary["critical_security_events_24h"] = ownerCount(ctx, db, `SELECT count(*) FROM security_audit_events WHERE created_at >= now() - interval '24 hours' AND lower(COALESCE(severity,'')) IN ('critical','fatal','high','error')`)
	}
	failed := int64(0)
	if ownerTableExists(ctx, db, "web3_jobs") {
		failed += ownerCount(ctx, db, `SELECT count(*) FROM web3_jobs WHERE updated_at >= now() - interval '24 hours' AND lower(COALESCE(status,'')) IN ('failed','error')`)
	}
	if ownerTableExists(ctx, db, "generation_jobs") {
		failed += ownerCount(ctx, db, `SELECT count(*) FROM generation_jobs WHERE updated_at >= now() - interval '24 hours' AND lower(COALESCE(status,'')) IN ('failed','error')`)
	}
	summary["failed_jobs_24h"] = failed
	return summary
}

func ownerCount(ctx context.Context, db *sql.DB, query string, args ...any) int64 {
	if db == nil {
		return 0
	}
	var count int64
	_ = db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count
}

func ownerDailyUserTrend(ctx context.Context, db *sql.DB) []map[string]any {
	out := make([]map[string]any, 0, 7)
	if db == nil || !ownerTableExists(ctx, db, "app_user_profiles") {
		return out
	}
	rows, err := db.QueryContext(ctx, `
		WITH days AS (SELECT generate_series(current_date - interval '6 days', current_date, interval '1 day')::date AS day)
		SELECT day::text, count(p.id)
		FROM days
		LEFT JOIN app_user_profiles p ON p.created_at >= day AND p.created_at < day + interval '1 day'
		GROUP BY day ORDER BY day
	`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var count int64
		if rows.Scan(&day, &count) == nil {
			out = append(out, map[string]any{"date": day, "count": count})
		}
	}
	return out
}

func ownerDailyOrderTrend(ctx context.Context, db *sql.DB) []map[string]any {
	out := make([]map[string]any, 0, 7)
	if db == nil || !ownerTableExists(ctx, db, "orders") {
		return out
	}
	rows, err := db.QueryContext(ctx, `
		WITH days AS (SELECT generate_series(current_date - interval '6 days', current_date, interval '1 day')::date AS day)
		SELECT day::text,
		       count(o.id) FILTER (WHERE lower(COALESCE(o.status,'')) IN ('paid','completed','success','active')),
		       COALESCE(sum(o.amount_try_cents) FILTER (WHERE lower(COALESCE(o.status,'')) IN ('paid','completed','success','active')),0)
		FROM days
		LEFT JOIN orders o ON o.created_at >= day AND o.created_at < day + interval '1 day'
		GROUP BY day ORDER BY day
	`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var count, revenue int64
		if rows.Scan(&day, &count, &revenue) == nil {
			out = append(out, map[string]any{"date": day, "count": count, "revenue_try_cents": revenue})
		}
	}
	return out
}

func ownerActionQueue(summary map[string]any, serviceMap map[string]any, arvis map[string]any) []map[string]any {
	actions := []map[string]any{}
	add := func(priority, kind, title, detail, tab string, count int64) {
		if count <= 0 {
			return
		}
		actions = append(actions, map[string]any{"priority": priority, "kind": kind, "title": title, "detail": detail, "target_tab": tab, "count": count})
	}
	add("critical", "arvis_failure", "Arvıs işlem hataları", "Son dönemde başarısız veya yeniden denenmesi gereken radar işleri var.", "arvis", mapInt64(arvis, "processing_failed_recent"))
	add("critical", "security_feedback", "Güvenlik geri bildirimleri", "Müşteriler güvenlik kategorisinde yeni bildirim gönderdi.", "feedback", mapInt64(summary, "security_feedback"))
	add("high", "security_events", "Kritik güvenlik olayları", "Son 24 saatte yüksek önem seviyesinde güvenlik olayı oluştu.", "security", mapInt64(summary, "critical_security_events_24h"))
	add("high", "pending_payment", "Bekleyen ödeme onayları", "Shopier ödeme talepleri owner incelemesi bekliyor.", "revenue", mapInt64(summary, "pending_payments"))
	add("medium", "feedback", "Yanıt bekleyen müşteri geri bildirimleri", "Yeni, incelenen veya planlanan geri bildirim kayıtları var.", "feedback", mapInt64(summary, "open_feedback"))
	add("medium", "expiring_entitlement", "Yakında bitecek paketler", "Önümüzdeki 7 gün içinde süresi dolacak aktif paketler var.", "customers", mapInt64(summary, "expiring_entitlements_7d"))
	add("medium", "failed_jobs", "Başarısız işler", "Son 24 saatte başarısız uygulama işleri oluştu.", "system", mapInt64(summary, "failed_jobs_24h"))

	for name, raw := range serviceMap {
		entry, _ := raw.(map[string]any)
		status := strings.ToLower(firstMapString(entry, "status"))
		if status == "missing" || status == "unavailable" || status == "error" {
			actions = append(actions, map[string]any{"priority": "medium", "kind": "service", "title": name + " yapılandırması", "detail": "Servis eksik veya kullanılamıyor: " + status, "target_tab": "system", "count": int64(1)})
		}
	}
	return actions
}

func mapInt64(values map[string]any, key string) int64 {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	default:
		return 0
	}
}

func firstMapString(values map[string]any, key string) string {
	if values == nil {
		return "unknown"
	}
	if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return "unknown"
}

func dbCount(r *http.Request, db *sql.DB, query string) int64 {
	return ownerCount(r.Context(), db, query)
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
	if truthyOwnerEnv(os.Getenv("KOSCHEI_AUTO_RADAR_ENABLED")) || truthyOwnerEnv(os.Getenv("RADAR_STREAM_ENABLED")) {
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
