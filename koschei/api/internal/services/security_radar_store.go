package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type SecurityRadarStore struct {
	DB *sql.DB
}

type SecurityRadarEventRecord struct {
	ModuleID      string
	Target        string
	TargetType    string
	Network       string
	Signature     string
	SourceAddress string
	EventType     string
	Slot          int64
	BlockTime     *time.Time
	Signals       map[string]any
	RawSummary    map[string]any
	Source        string
}

type SecurityRadarVerdictRecord struct {
	ID             string         `json:"id,omitempty"`
	EventID        string         `json:"event_id,omitempty"`
	ModuleID       string         `json:"module_id"`
	Target         string         `json:"target"`
	TargetType     string         `json:"target_type"`
	Network        string         `json:"network"`
	Grade          string         `json:"grade"`
	RiskIndex      int            `json:"risk_index"`
	RiskLevel      string         `json:"risk_level"`
	Verdict        string         `json:"verdict"`
	Recommendation string         `json:"recommendation"`
	Evidence       []string       `json:"evidence"`
	Signals        map[string]any `json:"signals"`
	RuleVersion    string         `json:"rule_version"`
	Signed         bool           `json:"signed"`
	Signature      string         `json:"signature"`
	Source         string         `json:"source,omitempty"`
	EventType      string         `json:"event_type,omitempty"`
	Provider       string         `json:"provider,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

func NewSecurityRadarStore(db *sql.DB) *SecurityRadarStore {
	return &SecurityRadarStore{DB: db}
}

func (s *SecurityRadarStore) MarkSignatureSeen(ctx context.Context, moduleID, signature, sourceAddress, network string) (bool, error) {
	if s == nil || s.DB == nil {
		return false, nil
	}
	moduleID = strings.TrimSpace(moduleID)
	signature = strings.TrimSpace(signature)
	sourceAddress = strings.TrimSpace(sourceAddress)
	network = normalizeRadarNetwork(network)
	if moduleID == "" || signature == "" {
		return false, nil
	}
	var id string
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO security_radar_seen_signatures (module_id, signature, source_address, source_target, network, seen_at, created_at)
		VALUES ($1,$2,NULLIF($3,''),NULLIF($3,''),$4,now(),now())
		ON CONFLICT (signature, module_id, network) DO NOTHING
		RETURNING id::text`, moduleID, signature, sourceAddress, network).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *SecurityRadarStore) InsertEvent(ctx context.Context, event SecurityRadarEventRecord) (string, error) {
	if s == nil || s.DB == nil {
		return "", nil
	}
	event.ModuleID = strings.TrimSpace(event.ModuleID)
	event.Target = strings.TrimSpace(event.Target)
	event.TargetType = firstSecurityRadarString(event.TargetType, "unknown")
	event.Network = normalizeRadarNetwork(event.Network)
	event.EventType = firstSecurityRadarString(event.EventType, "solana_signature")
	event.Source = firstSecurityRadarString(event.Source, "alchemy_polling")
	if event.ModuleID == "" || event.Target == "" {
		return "", nil
	}
	signals, _ := json.Marshal(nonNilMap(event.Signals))
	rawSummary, _ := json.Marshal(nonNilMap(event.RawSummary))
	var id string
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO security_radar_events (module_id,target,target_type,network,signature,source_address,event_type,slot,block_time,signals,raw_summary,source,created_at,updated_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,NULLIF($8,0),$9,$10::jsonb,$11::jsonb,$12,now(),now())
		RETURNING id::text`, event.ModuleID, event.Target, event.TargetType, event.Network, event.Signature, event.SourceAddress, event.EventType, event.Slot, event.BlockTime, string(signals), string(rawSummary), event.Source).Scan(&id)
	return id, err
}

func (s *SecurityRadarStore) InsertVerdict(ctx context.Context, verdict SecurityRadarVerdictRecord) (string, error) {
	if s == nil || s.DB == nil {
		return "", nil
	}
	verdict.ModuleID = strings.TrimSpace(verdict.ModuleID)
	verdict.Target = strings.TrimSpace(verdict.Target)
	verdict.TargetType = firstSecurityRadarString(verdict.TargetType, "unknown")
	verdict.Network = normalizeRadarNetwork(verdict.Network)
	verdict.RuleVersion = firstSecurityRadarString(verdict.RuleVersion, SecurityRadarRuleVersion)
	verdict.Source = firstSecurityRadarString(verdict.Source, "alchemy_polling")
	if verdict.Provider == "" {
		verdict.Provider = verdict.Source
	}
	if verdict.ModuleID == "" || verdict.Target == "" {
		return "", nil
	}
	signals := nonNilMap(verdict.Signals)
	if shouldApplySBX1HiddenSignals(verdict) {
		hidden := buildSBX1HiddenSignalPack(ctx, verdict)
		applyHiddenRiskAdjustment(&verdict, hidden.RiskAdjustment)
		signals["sbx1_hidden_signal_pack"] = hidden.Signals
		signals["sbx1_hidden_risk_adjustment"] = hidden.RiskAdjustment
		signals["sbx1_hidden_customer_surface"] = false
	}
	evidence, _ := json.Marshal(nonNilEvidence(verdict.Evidence))
	if verdict.EventType != "" {
		signals["event_type"] = verdict.EventType
	}
	if verdict.Provider != "" {
		signals["provider"] = verdict.Provider
	}
	signalsRaw, _ := json.Marshal(signals)
	var id string
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO security_radar_verdicts (event_id,module_id,target,target_type,network,grade,risk_index,risk_level,verdict,recommendation,evidence,signals,rule_version,signed,signature,source,created_at,updated_at)
		VALUES (NULLIF($1,'')::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12::jsonb,$13,$14,NULLIF($15,''),$16,now(),now())
		ON CONFLICT (signature,module_id) WHERE signature IS NOT NULL DO UPDATE SET
			event_id=COALESCE(EXCLUDED.event_id, security_radar_verdicts.event_id),
			target=EXCLUDED.target,
			target_type=EXCLUDED.target_type,
			network=EXCLUDED.network,
			grade=EXCLUDED.grade,
			risk_index=EXCLUDED.risk_index,
			risk_level=EXCLUDED.risk_level,
			verdict=EXCLUDED.verdict,
			recommendation=EXCLUDED.recommendation,
			evidence=EXCLUDED.evidence,
			signals=EXCLUDED.signals,
			rule_version=EXCLUDED.rule_version,
			signed=EXCLUDED.signed,
			source=EXCLUDED.source,
			updated_at=now()
		RETURNING id::text`, verdict.EventID, verdict.ModuleID, verdict.Target, verdict.TargetType, verdict.Network, verdict.Grade, verdict.RiskIndex, verdict.RiskLevel, verdict.Verdict, verdict.Recommendation, string(evidence), string(signalsRaw), verdict.RuleVersion, verdict.Signed, verdict.Signature, verdict.Source).Scan(&id)
	return id, err
}

func (s *SecurityRadarStore) LatestVerdicts(ctx context.Context, limit int) ([]SecurityRadarVerdictRecord, error) {
	if s == nil || s.DB == nil {
		return []SecurityRadarVerdictRecord{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT v.id::text, COALESCE(v.event_id::text,''), v.module_id, v.target, v.target_type, v.network, v.grade, v.risk_index, v.risk_level, v.verdict, v.recommendation, v.evidence, v.signals, v.rule_version, v.signed, COALESCE(v.signature,''), COALESCE(v.source,''), COALESCE(e.event_type,''), v.created_at
		FROM security_radar_verdicts v
		LEFT JOIN security_radar_events e ON e.id = v.event_id
		WHERE v.module_id <> 'walletless_claim_shield'
		ORDER BY v.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SecurityRadarVerdictRecord{}
	for rows.Next() {
		var item SecurityRadarVerdictRecord
		var evidenceRaw, signalsRaw []byte
		if err := rows.Scan(&item.ID, &item.EventID, &item.ModuleID, &item.Target, &item.TargetType, &item.Network, &item.Grade, &item.RiskIndex, &item.RiskLevel, &item.Verdict, &item.Recommendation, &evidenceRaw, &signalsRaw, &item.RuleVersion, &item.Signed, &item.Signature, &item.Source, &item.EventType, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(evidenceRaw, &item.Evidence)
		_ = json.Unmarshal(signalsRaw, &item.Signals)
		if item.Evidence == nil {
			item.Evidence = []string{}
		}
		if item.Signals == nil {
			item.Signals = map[string]any{}
		}
		if provider, ok := item.Signals["provider"].(string); ok && provider != "" {
			item.Provider = provider
		} else if item.Source == "pumpportal" {
			item.Provider = "alchemy+pumpportal"
		} else if item.Source != "" {
			item.Provider = item.Source
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeRadarNetwork(network string) string {
	if strings.TrimSpace(network) == "" {
		return "solana-mainnet"
	}
	return strings.TrimSpace(network)
}

func firstSecurityRadarString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func nonNilMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

func nonNilEvidence(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
