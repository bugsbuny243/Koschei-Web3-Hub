package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

const publicImpactCacheKey = "public-impact:v2"

var publicImpactCacheTTL = 60 * time.Second

type publicImpactMetrics struct {
	OK                           bool                        `json:"ok"`
	Statement                    string                      `json:"statement"`
	VerificationNote             string                      `json:"verification_note"`
	NoCustody                    string                      `json:"no_custody"`
	TotalSavedUSD                float64                     `json:"total_saved_usd"`
	MEVEstimatedLossPreventedUSD float64                     `json:"mev_estimated_loss_prevented_usd"`
	LiquidityLossPreventedUSD    float64                     `json:"liquidity_loss_prevented_usd"`
	RugPullsPrevented            int64                       `json:"rug_pulls_prevented"`
	ActiveProtectedWallets       int64                       `json:"active_protected_wallets"`
	Last24HBiggestPrevention     *publicImpactPreventionLog  `json:"last_24h_biggest_prevention,omitempty"`
	Last24HPreventedUSD          float64                     `json:"last_24h_prevented_usd"`
	Last7DPreventedUSD           float64                     `json:"last_7d_prevented_usd"`
	MEVEventsCount               int64                       `json:"mev_events_count"`
	LiquidityAlertsCount         int64                       `json:"liquidity_alerts_count"`
	SystemActiveUsers            int64                       `json:"system_active_users"`
	GeneratedOutputsCount        int64                       `json:"generated_outputs_count"`
	ModulesLiveCount             int                         `json:"modules_live_count"`
	SupportedNetworksCount       int64                       `json:"supported_networks_count"`
	ChainChecksCount             int64                       `json:"chain_checks_count"`
	WatchlistSourceCount         int64                       `json:"watchlist_source_count"`
	Web3EventCount               int64                       `json:"web3_event_count"`
	LiveModules                  []map[string]string         `json:"live_modules"`
	RecentLogs                   []publicImpactPreventionLog `json:"recent_logs"`
	PublicRoadmap                []string                    `json:"public_roadmap"`
	UpdatedAt                    time.Time                   `json:"updated_at"`
	CacheTTLSeconds              int                         `json:"cache_ttl_seconds"`
}

type publicImpactPreventionLog struct {
	Source              string    `json:"source"`
	AmountUSD           float64   `json:"amount_usd"`
	RiskScore           int       `json:"risk_score"`
	RiskLevel           string    `json:"risk_level"`
	AnonymizedSubject   string    `json:"anonymized_subject"`
	TransactionHint     string    `json:"transaction_hint,omitempty"`
	OccurredAt          time.Time `json:"occurred_at"`
	VerifiabilityStatus string    `json:"verifiability_status"`
}

// PublicImpact returns grant-facing, read-only public-good impact metrics.
// It deliberately uses the read replica handle (DBRead) when available and only
// executes SELECT statements against public impact tables.
func (h *Handler) GetPublicMetrics(w http.ResponseWriter, r *http.Request) {
	h.PublicImpact(w, r)
}

func (h *Handler) PublicImpact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var cached publicImpactMetrics
	if h.Cache != nil {
		if ok, err := h.Cache.GetJSON(ctx, publicImpactCacheKey, &cached); err == nil && ok {
			w.Header().Set("Cache-Control", "public, max-age=60")
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}

	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	metrics, err := buildPublicImpactMetrics(ctx, db)
	if err != nil {
		log.Printf("public impact metrics failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "impact metrics unavailable"})
		return
	}
	if h.Cache != nil {
		_ = h.Cache.SetJSON(ctx, publicImpactCacheKey, metrics, publicImpactCacheTTL)
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, metrics)
}

func buildPublicImpactMetrics(ctx context.Context, db *sql.DB) (publicImpactMetrics, error) {
	if db == nil {
		return publicImpactMetrics{}, errors.New("database handle is nil")
	}
	mods, err := publicImpactModules(ctx, db)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	mevSaved, err := safeImpactFloat(ctx, db, `SELECT COALESCE(SUM(estimated_loss_usd),0)::float FROM mev_protection_events`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	liquiditySaved, err := safeImpactFloat(ctx, db, `SELECT COALESCE(SUM(loss_prevented_usd),0)::float FROM liquidity_drain_alerts`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	last24hSaved, err := safeImpactFloat(ctx, db, `
		SELECT COALESCE(SUM(amount_usd),0)::float
		FROM (
			SELECT estimated_loss_usd AS amount_usd FROM mev_protection_events WHERE created_at >= now() - interval '24 hours'
			UNION ALL
			SELECT loss_prevented_usd AS amount_usd FROM liquidity_drain_alerts WHERE created_at >= now() - interval '24 hours'
		) impact_amounts`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	last7dSaved, err := safeImpactFloat(ctx, db, `
		SELECT COALESCE(SUM(amount_usd),0)::float
		FROM (
			SELECT estimated_loss_usd AS amount_usd FROM mev_protection_events WHERE created_at >= now() - interval '7 days'
			UNION ALL
			SELECT loss_prevented_usd AS amount_usd FROM liquidity_drain_alerts WHERE created_at >= now() - interval '7 days'
		) impact_amounts`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	activeWallets, err := safeImpactCount(ctx, db, `
		SELECT GREATEST(
			COALESCE((SELECT COUNT(DISTINCT lower(user_wallet)) FROM mev_protection_events WHERE user_wallet <> ''),0),
			COALESCE((SELECT MAX(active_users) FROM system_analytics),0)
		)::bigint`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	systemActiveUsers, err := safeImpactCount(ctx, db, `SELECT COALESCE(MAX(active_users),0)::bigint FROM system_analytics`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	biggest, err := latestBiggestPrevention(ctx, db)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	recentLogs, err := recentImpactPreventionLogs(ctx, db)
	if err != nil {
		return publicImpactMetrics{}, err
	}

	count := func(query string) (int64, error) { return safeImpactCount(ctx, db, query) }
	mevEvents, err := count(`SELECT COUNT(*) FROM mev_protection_events`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	liquidityAlerts, err := count(`SELECT COUNT(*) FROM liquidity_drain_alerts`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	rugPulls, err := count(`SELECT COUNT(*) FROM liquidity_drain_alerts WHERE risk_score >= 45 OR upper(severity) IN ('YÜKSEK','KRİTİK','HIGH','CRITICAL')`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	generated, err := count(`SELECT COUNT(*) FROM web3_outputs`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	networks, err := count(`SELECT COUNT(DISTINCT network) FROM web3_event_sources WHERE network IS NOT NULL`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	chainChecks, err := count(`SELECT COUNT(*) FROM chain_health_logs`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	watchlistSources, err := count(`SELECT COUNT(*) FROM web3_event_sources`)
	if err != nil {
		return publicImpactMetrics{}, err
	}
	web3Events, err := count(`SELECT COUNT(*) FROM web3_events`)
	if err != nil {
		return publicImpactMetrics{}, err
	}

	return publicImpactMetrics{
		OK:                           true,
		Statement:                    "Koschei Public Shield is a no-custody public-good risk prevention layer for Web3 builders and users.",
		VerificationNote:             "These metrics are backed by read-only database aggregates from blockchain-verifiable event records.",
		NoCustody:                    "No private keys. No seed phrases. No custody. Read-only intelligence.",
		TotalSavedUSD:                roundUSD(mevSaved + liquiditySaved),
		MEVEstimatedLossPreventedUSD: roundUSD(mevSaved),
		LiquidityLossPreventedUSD:    roundUSD(liquiditySaved),
		RugPullsPrevented:            rugPulls,
		ActiveProtectedWallets:       activeWallets,
		Last24HBiggestPrevention:     biggest,
		Last24HPreventedUSD:          roundUSD(last24hSaved),
		Last7DPreventedUSD:           roundUSD(last7dSaved),
		MEVEventsCount:               mevEvents,
		LiquidityAlertsCount:         liquidityAlerts,
		SystemActiveUsers:            systemActiveUsers,
		GeneratedOutputsCount:        generated,
		ModulesLiveCount:             len(mods),
		SupportedNetworksCount:       networks,
		ChainChecksCount:             chainChecks,
		WatchlistSourceCount:         watchlistSources,
		Web3EventCount:               web3Events,
		LiveModules:                  mods,
		RecentLogs:                   recentLogs,
		PublicRoadmap:                []string{"Publish verifiable public-good impact snapshots", "Open-source MEV Shield and Liquidity Radar safety modules", "Add community-maintained risk feeds for Solana and Ethereum"},
		UpdatedAt:                    time.Now().UTC(),
		CacheTTLSeconds:              int(publicImpactCacheTTL.Seconds()),
	}, nil
}

func publicImpactModules(ctx context.Context, db *sql.DB) ([]map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT title, COALESCE(description,''), COALESCE(category,'') FROM koschei_modules WHERE status='active' ORDER BY title`)
	if isUndefinedImpactSource(err) {
		return []map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	mods := []map[string]string{}
	for rows.Next() {
		var title, description, category string
		if err := rows.Scan(&title, &description, &category); err != nil {
			return nil, err
		}
		mods = append(mods, map[string]string{"title": title, "description": description, "category": category})
	}
	return mods, rows.Err()
}

func safeImpactFloat(ctx context.Context, db *sql.DB, query string) (float64, error) {
	var value float64
	err := db.QueryRowContext(ctx, query).Scan(&value)
	if isUndefinedImpactSource(err) {
		return 0, nil
	}
	return value, err
}

func latestBiggestPrevention(ctx context.Context, db *sql.DB) (*publicImpactPreventionLog, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT source, subject, tx_hint, amount_usd::float, risk_score, risk_level, created_at
		FROM (
			SELECT 'MEV Shield' AS source, user_wallet AS subject, tx_signature AS tx_hint, estimated_loss_usd AS amount_usd, risk_score, risk_level, created_at
			FROM mev_protection_events
			WHERE created_at >= now() - interval '24 hours'
			UNION ALL
			SELECT 'Liquidity Radar' AS source, COALESCE(NULLIF(pool_address,''), token_mint) AS subject, '' AS tx_hint, loss_prevented_usd AS amount_usd, risk_score, severity AS risk_level, created_at
			FROM liquidity_drain_alerts
			WHERE created_at >= now() - interval '24 hours'
		) recent_preventions
		ORDER BY amount_usd DESC, created_at DESC
		LIMIT 1`)
	if isUndefinedImpactSource(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var source, subject, txHint, level string
	var amount float64
	var score int
	var occurredAt time.Time
	if err := rows.Scan(&source, &subject, &txHint, &amount, &score, &level, &occurredAt); err != nil {
		return nil, err
	}
	return &publicImpactPreventionLog{Source: source, AmountUSD: roundUSD(amount), RiskScore: score, RiskLevel: level, AnonymizedSubject: anonymizeImpactSubject(subject), TransactionHint: anonymizeImpactSubject(txHint), OccurredAt: occurredAt, VerifiabilityStatus: "Blockchain-verifiable event; public view is anonymized for user safety."}, rows.Err()
}

func recentImpactPreventionLogs(ctx context.Context, db *sql.DB) ([]publicImpactPreventionLog, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT source, subject, tx_hint, amount_usd::float, risk_score, risk_level, created_at
		FROM (
			SELECT 'MEV Shield' AS source, user_wallet AS subject, tx_signature AS tx_hint, estimated_loss_usd AS amount_usd, risk_score, risk_level, created_at
			FROM mev_protection_events
			UNION ALL
			SELECT 'Liquidity Radar' AS source, COALESCE(NULLIF(pool_address,''), token_mint) AS subject, '' AS tx_hint, loss_prevented_usd AS amount_usd, risk_score, severity AS risk_level, created_at
			FROM liquidity_drain_alerts
		) recent_preventions
		ORDER BY created_at DESC
		LIMIT 10`)
	if isUndefinedImpactSource(err) {
		return []publicImpactPreventionLog{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []publicImpactPreventionLog{}
	for rows.Next() {
		var source, subject, txHint, level string
		var amount float64
		var score int
		var occurredAt time.Time
		if err := rows.Scan(&source, &subject, &txHint, &amount, &score, &level, &occurredAt); err != nil {
			return nil, err
		}
		logs = append(logs, publicImpactPreventionLog{Source: source, AmountUSD: roundUSD(amount), RiskScore: score, RiskLevel: level, AnonymizedSubject: anonymizeImpactSubject(subject), TransactionHint: anonymizeImpactSubject(txHint), OccurredAt: occurredAt, VerifiabilityStatus: "Blockchain-verifiable event; public view is anonymized for user safety."})
	}
	return logs, rows.Err()
}

func isUndefinedImpactSource(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined_table") || strings.Contains(msg, "undefined_column")
}

func anonymizeImpactSubject(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "anonymous"
	}
	if len(value) <= 10 {
		return value[:1] + "…" + value[len(value)-1:]
	}
	return value[:6] + "…" + value[len(value)-4:]
}

func roundUSD(value float64) float64 {
	return float64(int64(value*100+0.5)) / 100
}
