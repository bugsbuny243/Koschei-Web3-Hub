package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type protectedValuePoint struct {
	Date                   string  `json:"date"`
	MEVSavedUSD            float64 `json:"mev_saved_usd"`
	LiquidityProtectedUSD  float64 `json:"liquidity_protected_usd"`
	GovernanceProtectedUSD float64 `json:"governance_protected_usd"`
	TotalValueProtectedUSD float64 `json:"total_value_protected_usd"`
}

type impactTweetDraft struct {
	Date      string `json:"date"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at,omitempty"`
}

type publicMEVProtectionEvent struct {
	ID               string         `json:"id"`
	UserWallet       string         `json:"user_wallet"`
	TXSignature      string         `json:"tx_signature"`
	EstimatedLossUSD float64        `json:"estimated_loss_usd"`
	MEVSavedUSD      float64        `json:"mev_saved_usd"`
	JitoTipUsed      bool           `json:"jito_tip_used"`
	RiskScore        int            `json:"risk_score"`
	RiskLevel        string         `json:"risk_level"`
	Route            string         `json:"route"`
	RawPayload       map[string]any `json:"raw_payload"`
	CreatedAt        time.Time      `json:"created_at"`
}

func protectedImpactMetrics(ctx context.Context, db *sql.DB) (map[string]any, error) {
	if db == nil {
		return map[string]any{"total_value_protected_usd": 0, "series": []protectedValuePoint{}, "tweet_draft": draftForImpact(time.Now().UTC(), 0)}, nil
	}
	series := make([]protectedValuePoint, 30)
	index := map[string]int{}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	for i := 29; i >= 0; i-- {
		day := today.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		idx := 29 - i
		series[idx] = protectedValuePoint{Date: key}
		index[key] = idx
	}
	if err := addProtectedSeries(ctx, db, index, series, `SELECT date_trunc('day', created_at)::date::text, COALESCE(sum(mev_saved_usd),0)::float8 FROM mev_protection_events WHERE created_at >= now() - interval '30 days' GROUP BY 1`, func(p *protectedValuePoint, v float64) { p.MEVSavedUSD = v }); err != nil {
		return nil, err
	}
	if err := addProtectedSeries(ctx, db, index, series, `SELECT date_trunc('day', created_at)::date::text, COALESCE(sum(loss_prevented_usd),0)::float8 FROM liquidity_drain_alerts WHERE created_at >= now() - interval '30 days' GROUP BY 1`, func(p *protectedValuePoint, v float64) { p.LiquidityProtectedUSD = v }); err != nil {
		return nil, err
	}
	if err := addProtectedSeries(ctx, db, index, series, `SELECT date_trunc('day', created_at)::date::text, COALESCE(sum(estimated_outflow_usd),0)::float8 FROM proposal_risks WHERE risk_score >= 70 AND created_at >= now() - interval '30 days' GROUP BY 1`, func(p *protectedValuePoint, v float64) { p.GovernanceProtectedUSD = v }); err != nil {
		return nil, err
	}
	total := 0.0
	for i := range series {
		series[i].TotalValueProtectedUSD = roundMoney(series[i].MEVSavedUSD + series[i].LiquidityProtectedUSD + series[i].GovernanceProtectedUSD)
		total += series[i].TotalValueProtectedUSD
	}
	draft := ensureDailyImpactTweetDraft(ctx, db, total)
	return map[string]any{"total_value_protected_usd": roundMoney(total), "series": series, "tweet_draft": draft}, nil
}

func addProtectedSeries(ctx context.Context, db *sql.DB, index map[string]int, series []protectedValuePoint, query string, set func(*protectedValuePoint, float64)) error {
	rows, err := db.QueryContext(ctx, query)
	if isMissingRelation(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var value float64
		if err := rows.Scan(&day, &value); err != nil {
			return err
		}
		if idx, ok := index[day]; ok {
			set(&series[idx], roundMoney(value))
		}
	}
	return rows.Err()
}

func ensureDailyImpactTweetDraft(ctx context.Context, db *sql.DB, total float64) impactTweetDraft {
	now := time.Now().UTC()
	draft := draftForImpact(now, total)
	if db == nil {
		return draft
	}
	var text string
	var created time.Time
	err := db.QueryRowContext(ctx, `INSERT INTO impact_tweet_drafts (draft_date, draft_text, metrics_snapshot, created_at) VALUES (CURRENT_DATE, $1, $2::jsonb, now()) ON CONFLICT (draft_date) DO UPDATE SET draft_text = EXCLUDED.draft_text, metrics_snapshot = EXCLUDED.metrics_snapshot RETURNING draft_text, created_at`, draft.Text, fmt.Sprintf(`{"total_value_protected_usd":%.2f}`, total)).Scan(&text, &created)
	if err != nil {
		return draft
	}
	draft.Text = text
	draft.CreatedAt = created.UTC().Format(time.RFC3339)
	return draft
}

func draftForImpact(now time.Time, total float64) impactTweetDraft {
	date := now.UTC().Format("2006-01-02")
	text := fmt.Sprintf("Daily Koschei public-good impact (%s): $%.2f Total Value Protected across MEV Shield, Liquidity Radar and DAO Guardian signals. No custody. No private keys. Ecosystem safety first.", date, roundMoney(total))
	return impactTweetDraft{Date: date, Text: text}
}

func roundMoney(v float64) float64 { return float64(int(v*100+0.5)) / 100 }

func isMissingRelation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined_table")
}

func (h *Handler) PublicMEVProtectionEvents(w http.ResponseWriter, r *http.Request) {
	if h.DBRead == nil && h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	rows, err := db.QueryContext(r.Context(), `SELECT id::text, user_wallet, tx_signature, estimated_loss_usd::float8, COALESCE(mev_saved_usd, estimated_loss_usd)::float8, jito_tip_used, risk_score, risk_level, route, COALESCE(raw_payload, '{}'::jsonb), created_at FROM mev_protection_events ORDER BY created_at DESC`)
	if isMissingRelation(err) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": 0, "events": []publicMEVProtectionEvent{}})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "events_unavailable"})
		return
	}
	defer rows.Close()
	events := make([]publicMEVProtectionEvent, 0)
	for rows.Next() {
		var event publicMEVProtectionEvent
		var raw []byte
		if err := rows.Scan(&event.ID, &event.UserWallet, &event.TXSignature, &event.EstimatedLossUSD, &event.MEVSavedUSD, &event.JitoTipUsed, &event.RiskScore, &event.RiskLevel, &event.Route, &raw, &event.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "events_scan_failed"})
			return
		}
		if err := json.Unmarshal(raw, &event.RawPayload); err != nil {
			event.RawPayload = map[string]any{}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "events_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": len(events), "events": events})
}
