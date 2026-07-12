package services

import (
	"context"
	"database/sql"
	"encoding/json"
)

// LatestPumpHighVolumeReportsExact is the owner-panel read path for automatic
// Pump reports. Solana addresses are case-sensitive base58 values, so exact
// equality is both correct and allows the existing (module_id,target,...) index
// to avoid repeatedly scanning every final verdict for each target.
func (s *SecurityRadarStore) LatestPumpHighVolumeReportsExact(ctx context.Context, limit int) ([]PumpHighVolumeOwnerItem, error) {
	if s == nil || s.DB == nil {
		return []PumpHighVolumeOwnerItem{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
		WITH latest_events AS (
			SELECT DISTINCT ON (e.target) e.id::text, e.target, e.signals, e.created_at
			FROM security_radar_events e
			WHERE e.event_type=$1 AND e.source=$2 AND btrim(e.target)<>''
			ORDER BY e.target, e.created_at DESC, e.id DESC
		)
		SELECT e.id,e.target,e.signals,e.created_at,
		       v.risk_index,v.risk_level,v.verdict,v.created_at
		FROM latest_events e
		LEFT JOIN LATERAL (
			SELECT risk_index,risk_level,verdict,created_at
			FROM security_radar_verdicts v
			WHERE v.target=e.target AND v.module_id='final_verdict_engine'
			  AND v.signed=true AND v.source=$2
			ORDER BY v.created_at DESC,v.id DESC LIMIT 1
		) v ON true
		ORDER BY COALESCE((e.signals->>'volume_24h_usd')::numeric,0) DESC,e.created_at DESC
		LIMIT $3`, pumpHighVolumeEventType, pumpHighVolumeSource, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []PumpHighVolumeOwnerItem{}
	for rows.Next() {
		var item PumpHighVolumeOwnerItem
		var signalsRaw []byte
		var risk sql.NullInt64
		var level, verdict sql.NullString
		var reportAt sql.NullTime
		if err := rows.Scan(&item.EventID, &item.Target, &signalsRaw, &item.ObservedAt, &risk, &level, &verdict, &reportAt); err != nil {
			return nil, err
		}
		item.Signals = map[string]any{}
		_ = json.Unmarshal(signalsRaw, &item.Signals)
		item.Name = pumpSignalString(item.Signals, "token_name", "name")
		item.Symbol = pumpSignalString(item.Signals, "token_symbol", "symbol")
		item.Creator = pumpSignalString(item.Signals, "creator_wallet", "creator")
		item.Volume24hUSD = pumpSignalFloat(item.Signals, "volume_24h_usd")
		item.ThresholdUSD = pumpSignalFloat(item.Signals, "volume_threshold_usd")
		item.PairCount = int(pumpSignalFloat(item.Signals, "volume_pair_count"))
		item.LiquidityUSD = pumpSignalFloat(item.Signals, "liquidity_usd")
		item.MarketCapUSD = pumpSignalFloat(item.Signals, "market_cap_usd")
		item.VolumeProvider = pumpSignalString(item.Signals, "volume_provider")
		item.ReportStatus = "queued"
		if pumpSignalBool(item.Signals, "auto_scan_attempted") {
			item.ReportStatus = "evidence_pending"
		}
		if risk.Valid {
			value := int(risk.Int64)
			item.RiskIndex = &value
			item.RiskLevel = level.String
			item.Verdict = verdict.String
			item.ReportStatus = "completed"
		}
		if reportAt.Valid {
			value := reportAt.Time.UTC()
			item.ReportAt = &value
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
