package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const smartMoneyEvidenceFilter = `(
	source IN ('pumpportal','alchemy_polling','arvis_stream')
	OR signals @> '{"real_onchain_evidence":true}'::jsonb
	OR signals @> '{"source_verified_program_event":true}'::jsonb
)`

type smartMoneySummary struct {
	Observations       int64      `json:"observations"`
	UniqueTargets      int64      `json:"unique_targets"`
	UniqueTransactions int64      `json:"unique_transactions"`
	HighRiskSignals    int64      `json:"high_risk_signals"`
	ObservedWallets    int64      `json:"observed_wallets"`
	LatestEventAt      *time.Time `json:"latest_event_at,omitempty"`
}

type smartMoneyWallet struct {
	Address       string    `json:"address"`
	Observations  int64     `json:"observations"`
	UniqueTokens  int64     `json:"unique_tokens"`
	BuyEvents     int64     `json:"buy_events"`
	SellEvents    int64     `json:"sell_events"`
	ActivityBias  string    `json:"activity_bias"`
	MaxRiskIndex  int       `json:"max_risk_index"`
	AverageRisk   float64   `json:"average_risk_index"`
	EvidenceFeeds string    `json:"evidence_feeds"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

type smartMoneyFundingAccount struct {
	Address       string    `json:"address"`
	Observations  int64     `json:"observations"`
	UniqueTargets int64     `json:"unique_targets"`
	MaxRiskIndex  int       `json:"max_risk_index"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

type smartMoneyAlert struct {
	Target         string         `json:"target"`
	ModuleID       string         `json:"module_id"`
	RiskIndex      int            `json:"risk_index"`
	RiskLevel      string         `json:"risk_level"`
	Verdict        string         `json:"verdict"`
	Recommendation string         `json:"recommendation"`
	Metrics        map[string]any `json:"metrics"`
	CreatedAt      time.Time      `json:"created_at"`
}

type smartMoneyModuleQuality struct {
	ModuleID        string     `json:"module_id"`
	Observations    int64      `json:"observations"`
	VerifiedOnchain int64      `json:"verified_onchain"`
	LatestEventAt   *time.Time `json:"latest_event_at,omitempty"`
}

// SmartMoneyStream is retained for a future continuous client. The production
// page currently polls the verified snapshot endpoint instead of holding open a
// connection.
func (h *Handler) SmartMoneyStream(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "websocket_upgrader_pending", "message": "Use the verified snapshot endpoint."})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: {\"snapshot\":\"/api/smart-money/snapshot\",\"ts\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// SmartMoneySnapshot aggregates verified ARVIS activity. It deliberately calls
// wallets "observed" rather than profitable, institutional or whale accounts:
// the underlying evidence proves activity and relationships, not future returns.
func (h *Handler) SmartMoneySnapshot(w http.ResponseWriter, r *http.Request) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database_unavailable"})
		return
	}

	windowLabel, windowDuration := parseSmartMoneyWindow(r.URL.Query().Get("window"))
	limit := parseSmartMoneyLimit(r.URL.Query().Get("limit"))
	since := time.Now().UTC().Add(-windowDuration)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	summary, err := loadSmartMoneySummary(ctx, db, since)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "snapshot_unavailable"})
		return
	}
	wallets, err := loadSmartMoneyWallets(ctx, db, since, limit)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "wallet_activity_unavailable"})
		return
	}
	funding, err := loadSmartMoneyFundingAccounts(ctx, db, since, limit)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "funding_activity_unavailable"})
		return
	}
	alerts, err := loadSmartMoneyAlerts(ctx, db, since, limit)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "alert_activity_unavailable"})
		return
	}
	quality, err := loadSmartMoneyQuality(ctx, db, since)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "quality_summary_unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"data_available": summary.Observations > 0,
		"mode":           "verified_observed_activity",
		"generated_at":   time.Now().UTC(),
		"window":         windowLabel,
		"source":         "arvis_security_radar_events",
		"summary":        summary,
		"active_wallets": wallets,
		"funding_accounts": funding,
		"alerts":          alerts,
		"data_quality":    quality,
		"disclaimer":      "Rankings represent verified observed activity and risk evidence, not profitability, ownership, investment quality or future performance.",
	})
}

func parseSmartMoneyWindow(raw string) (string, time.Duration) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "15m":
		return "15m", 15 * time.Minute
	case "6h":
		return "6h", 6 * time.Hour
	case "24h":
		return "24h", 24 * time.Hour
	default:
		return "1h", time.Hour
	}
}

func parseSmartMoneyLimit(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 12
	}
	if value > 50 {
		return 50
	}
	return value
}

func smartMoneyActivityBias(buys, sells int64) string {
	switch {
	case buys > sells*2 && buys >= 3:
		return "buy_dominant"
	case sells > buys*2 && sells >= 3:
		return "sell_dominant"
	default:
		return "mixed"
	}
}

func loadSmartMoneySummary(ctx context.Context, db *sql.DB, since time.Time) (smartMoneySummary, error) {
	var out smartMoneySummary
	var latest sql.NullTime
	query := `SELECT
		count(*)::bigint,
		count(DISTINCT target)::bigint,
		count(DISTINCT NULLIF(signature,''))::bigint,
		count(*) FILTER (WHERE risk_index >= 70 OR lower(COALESCE(verdict,'')) IN ('block','withhold'))::bigint,
		count(DISTINCT NULLIF(signals->>'trader',''))::bigint,
		max(created_at)
	FROM security_radar_events
	WHERE created_at >= $1
	  AND module_id IN ('pump_sybil_radar','funding_cluster_detector','holder_concentration','liquidity_movement','sniper_timing_detector')
	  AND ` + smartMoneyEvidenceFilter
	err := db.QueryRowContext(ctx, query, since).Scan(&out.Observations, &out.UniqueTargets, &out.UniqueTransactions, &out.HighRiskSignals, &out.ObservedWallets, &latest)
	if latest.Valid {
		out.LatestEventAt = &latest.Time
	}
	return out, err
}

func loadSmartMoneyWallets(ctx context.Context, db *sql.DB, since time.Time, limit int) ([]smartMoneyWallet, error) {
	rows, err := db.QueryContext(ctx, `SELECT
		signals->>'trader' AS address,
		count(DISTINCT NULLIF(signature,''))::bigint AS observations,
		count(DISTINCT NULLIF(signals->>'mint',''))::bigint AS unique_tokens,
		count(*) FILTER (WHERE lower(COALESCE(signals->>'tx_type','')) LIKE '%buy%')::bigint AS buy_events,
		count(*) FILTER (WHERE lower(COALESCE(signals->>'tx_type','')) LIKE '%sell%')::bigint AS sell_events,
		COALESCE(max(risk_index),0),
		COALESCE(avg(risk_index),0)::float8,
		string_agg(DISTINCT source, ',' ORDER BY source),
		min(created_at),
		max(created_at)
	FROM security_radar_events
	WHERE created_at >= $1
	  AND module_id='pump_sybil_radar'
	  AND NULLIF(signals->>'trader','') IS NOT NULL
	  AND NULLIF(signature,'') IS NOT NULL
	  AND `+smartMoneyEvidenceFilter+`
	GROUP BY signals->>'trader'
	ORDER BY observations DESC, max(created_at) DESC
	LIMIT $2`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]smartMoneyWallet, 0, limit)
	for rows.Next() {
		var item smartMoneyWallet
		if err := rows.Scan(&item.Address, &item.Observations, &item.UniqueTokens, &item.BuyEvents, &item.SellEvents, &item.MaxRiskIndex, &item.AverageRisk, &item.EvidenceFeeds, &item.FirstSeenAt, &item.LastSeenAt); err != nil {
			return nil, err
		}
		item.ActivityBias = smartMoneyActivityBias(item.BuyEvents, item.SellEvents)
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadSmartMoneyFundingAccounts(ctx context.Context, db *sql.DB, since time.Time, limit int) ([]smartMoneyFundingAccount, error) {
	rows, err := db.QueryContext(ctx, `SELECT
		account,
		count(*)::bigint,
		count(DISTINCT target)::bigint,
		COALESCE(max(risk_index),0),
		max(created_at)
	FROM security_radar_events
	CROSS JOIN LATERAL jsonb_array_elements_text(signals->'funding_accounts') AS account
	WHERE created_at >= $1
	  AND module_id='funding_cluster_detector'
	  AND signals @> '{"real_onchain_evidence":true}'::jsonb
	  AND NULLIF(account,'') IS NOT NULL
	GROUP BY account
	ORDER BY count(*) DESC, max(created_at) DESC
	LIMIT $2`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]smartMoneyFundingAccount, 0, limit)
	for rows.Next() {
		var item smartMoneyFundingAccount
		if err := rows.Scan(&item.Address, &item.Observations, &item.UniqueTargets, &item.MaxRiskIndex, &item.LastSeenAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadSmartMoneyAlerts(ctx context.Context, db *sql.DB, since time.Time, limit int) ([]smartMoneyAlert, error) {
	rows, err := db.QueryContext(ctx, `SELECT
		target,
		module_id,
		risk_index,
		COALESCE(risk_level,''),
		COALESCE(verdict,''),
		COALESCE(recommendation,''),
		COALESCE(signals->>'largest_holder_percentage',''),
		COALESCE(signals->>'top_10_holder_percentage',''),
		COALESCE(signals->>'recent_signature_count',''),
		COALESCE(signals->>'failed_signature_count',''),
		COALESCE(signals->>'funding_account_count',''),
		COALESCE(signals->>'token_mint_count',''),
		created_at
	FROM security_radar_events
	WHERE created_at >= $1
	  AND module_id IN ('funding_cluster_detector','holder_concentration','liquidity_movement','sniper_timing_detector')
	  AND signals @> '{"real_onchain_evidence":true}'::jsonb
	  AND (risk_index >= 35 OR lower(COALESCE(verdict,'')) IN ('warn','block','withhold'))
	ORDER BY risk_index DESC, created_at DESC
	LIMIT $2`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]smartMoneyAlert, 0, limit)
	for rows.Next() {
		var item smartMoneyAlert
		var largest, top10, recent, failed, fundingCount, mintCount string
		if err := rows.Scan(&item.Target, &item.ModuleID, &item.RiskIndex, &item.RiskLevel, &item.Verdict, &item.Recommendation, &largest, &top10, &recent, &failed, &fundingCount, &mintCount, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Metrics = map[string]any{}
		addSmartMoneyMetric(item.Metrics, "largest_holder_percentage", largest)
		addSmartMoneyMetric(item.Metrics, "top_10_holder_percentage", top10)
		addSmartMoneyMetric(item.Metrics, "recent_signature_count", recent)
		addSmartMoneyMetric(item.Metrics, "failed_signature_count", failed)
		addSmartMoneyMetric(item.Metrics, "funding_account_count", fundingCount)
		addSmartMoneyMetric(item.Metrics, "token_mint_count", mintCount)
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadSmartMoneyQuality(ctx context.Context, db *sql.DB, since time.Time) ([]smartMoneyModuleQuality, error) {
	rows, err := db.QueryContext(ctx, `SELECT
		module_id,
		count(*)::bigint,
		count(*) FILTER (WHERE signals @> '{"real_onchain_evidence":true}'::jsonb)::bigint,
		max(created_at)
	FROM security_radar_events
	WHERE created_at >= $1
	  AND module_id IN ('pump_sybil_radar','funding_cluster_detector','holder_concentration','liquidity_movement','sniper_timing_detector')
	  AND `+smartMoneyEvidenceFilter+`
	GROUP BY module_id
	ORDER BY count(*) DESC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []smartMoneyModuleQuality{}
	for rows.Next() {
		var item smartMoneyModuleQuality
		var latest sql.NullTime
		if err := rows.Scan(&item.ModuleID, &item.Observations, &item.VerifiedOnchain, &latest); err != nil {
			return nil, err
		}
		if latest.Valid {
			item.LatestEventAt = &latest.Time
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func addSmartMoneyMetric(dst map[string]any, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if numeric, err := strconv.ParseFloat(value, 64); err == nil {
		dst[key] = numeric
		return
	}
	dst[key] = value
}
