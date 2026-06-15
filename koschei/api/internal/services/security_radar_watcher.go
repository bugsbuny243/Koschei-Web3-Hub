package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/web3"
)

type SecurityRadarWatcher struct {
	DB       *sql.DB
	RPC      *web3.SolanaRPC
	Network  string
	Interval time.Duration
	Limit    int
	Sources  []SecurityRadarSource
}

type SecurityRadarSource struct {
	ModuleID   string
	Name       string
	Target     string
	TargetType string
}

type radarSignatureInfo struct {
	Signature string      `json:"signature"`
	Slot      uint64      `json:"slot"`
	Err       interface{} `json:"err"`
	BlockTime *int64      `json:"blockTime"`
}

func SecurityRadarAutoEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("KOSCHEI_AUTO_RADAR_ENABLED")), "1") || strings.EqualFold(strings.TrimSpace(os.Getenv("KOSCHEI_AUTO_RADAR_ENABLED")), "true")
}

func StartSecurityRadarWatcher(ctx context.Context, db *sql.DB, rpc *web3.SolanaRPC) func() {
	if db == nil || rpc == nil || !SecurityRadarAutoEnabled() {
		return func() {}
	}
	watcher := NewSecurityRadarWatcher(db, rpc)
	ctx, cancel := context.WithCancel(ctx)
	go watcher.Run(ctx)
	return cancel
}

func NewSecurityRadarWatcher(db *sql.DB, rpc *web3.SolanaRPC) *SecurityRadarWatcher {
	interval := 10 * time.Second
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_RADAR_POLL_SECONDS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			interval = time.Duration(v) * time.Second
		}
	}
	network := strings.TrimSpace(os.Getenv("KOSCHEI_SECURITY_NETWORK"))
	if network == "" {
		network = "solana-mainnet"
	}
	limit := 20
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_RADAR_SIGNATURE_LIMIT")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	return &SecurityRadarWatcher{DB: db, RPC: rpc, Network: network, Interval: interval, Limit: limit, Sources: defaultSecurityRadarSources()}
}

func defaultSecurityRadarSources() []SecurityRadarSource {
	pumpProgram := envWithDefault("6EF8rrecthR5Dk3SJLDEsy7M1H7MxuR6cWt98YVYgKED", "KOSCHEI_PUMPFUN_PROGRAM_ID", "PUMPFUN_PROGRAM_ID")
	raydiumProgram := envWithDefault("675kPX9MHTjS2zt1qfr1NY5Wwrzj4mWjU7VtXv9syS2", "KOSCHEI_RAYDIUM_PROGRAM_ID", "RAYDIUM_PROGRAM_ID")
	sources := []SecurityRadarSource{
		{ModuleID: ModulePumpSybilRadar, Name: "Pump.fun Sybil Radar", Target: pumpProgram, TargetType: "program"},
		{ModuleID: ModuleRaydiumPoolGuardian, Name: "Raydium Pool Guardian", Target: raydiumProgram, TargetType: "program"},
	}
	for _, target := range splitEnvList(firstEnvValue("KOSCHEI_CLAIM_PROGRAM_IDS", "KOSCHEI_CLAIM_TARGETS")) {
		sources = append(sources, SecurityRadarSource{ModuleID: ModuleWalletlessClaimShield, Name: "Walletless Claim Shield", Target: target, TargetType: "program"})
	}
	return sources
}

func (w *SecurityRadarWatcher) Run(ctx context.Context) {
	if w == nil || w.DB == nil || w.RPC == nil {
		return
	}
	log.Printf("security radar watcher started provider=%s mode=%s interval=%s", SecurityRadarProvider, SecurityRadarWatchMode, w.Interval)
	w.pollOnce(ctx)
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("security radar watcher stopped")
			return
		case <-ticker.C:
			w.pollOnce(ctx)
		}
	}
}

func (w *SecurityRadarWatcher) pollOnce(ctx context.Context) {
	for _, source := range w.Sources {
		if strings.TrimSpace(source.Target) == "" {
			continue
		}
		if err := w.pollSource(ctx, source); err != nil {
			log.Printf("security radar source poll failed module=%s target=%s err=%v", source.ModuleID, source.Target, err)
		}
	}
}

func (w *SecurityRadarWatcher) pollSource(ctx context.Context, source SecurityRadarSource) error {
	pollCtx, cancel := context.WithTimeout(ctx, minDuration(w.Interval, 12*time.Second))
	defer cancel()
	var sigs []radarSignatureInfo
	if err := w.RPC.Call(pollCtx, w.Network, "getSignaturesForAddress", []any{source.Target, map[string]any{"limit": w.Limit}}, &sigs, 30*time.Second); err != nil {
		return err
	}
	_ = w.upsertSource(ctx, source)
	for i := len(sigs) - 1; i >= 0; i-- {
		sig := strings.TrimSpace(sigs[i].Signature)
		if sig == "" {
			continue
		}
		inserted, err := w.markSeen(ctx, source, sig, sigs[i].Slot)
		if err != nil || !inserted {
			continue
		}
		bundle := AnalyzeSecurityRadars(SecurityRadarRequest{Target: sig, Network: w.Network, Mode: "alchemy_polling"})
		verdict := moduleVerdict(bundle, source.ModuleID)
		if verdict.Signals == nil {
			verdict.Signals = map[string]any{}
		}
		verdict.Signals["transaction_loaded"] = w.fetchTransaction(ctx, sig)
		_ = w.storeEvent(ctx, source, sig, sigs[i], verdict)
		_ = w.storeVerdict(ctx, source, sig, verdict)
		_ = w.updateCheckpoint(ctx, source, sig, sigs[i].Slot)
	}
	return nil
}

func (w *SecurityRadarWatcher) fetchTransaction(ctx context.Context, signature string) bool {
	if w == nil || w.RPC == nil || strings.TrimSpace(signature) == "" {
		return false
	}
	txCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var tx json.RawMessage
	err := w.RPC.Call(txCtx, w.Network, "getTransaction", []any{signature, map[string]any{"encoding": "jsonParsed", "maxSupportedTransactionVersion": 0}}, &tx, 24*time.Hour)
	return err == nil && len(tx) > 0
}

func moduleVerdict(bundle SecurityRadarBundle, moduleID string) SecurityRadarVerdict {
	switch moduleID {
	case ModuleRaydiumPoolGuardian:
		return bundle.RaydiumPoolGuardian
	case ModuleWalletlessClaimShield:
		return bundle.WalletlessClaimShield
	default:
		return bundle.PumpSybilRadar
	}
}

func (w *SecurityRadarWatcher) markSeen(ctx context.Context, source SecurityRadarSource, signature string, slot uint64) (bool, error) {
	var id string
	err := w.DB.QueryRowContext(ctx, `INSERT INTO security_radar_seen_signatures (module_id, source_target, signature, slot, created_at) VALUES ($1,$2,$3,$4,now()) ON CONFLICT (module_id, signature) DO NOTHING RETURNING id::text`, source.ModuleID, source.Target, signature, slot).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (w *SecurityRadarWatcher) storeEvent(ctx context.Context, source SecurityRadarSource, signature string, sigInfo radarSignatureInfo, verdict SecurityRadarVerdict) error {
	evidence, _ := json.Marshal(verdict.Evidence)
	signals, _ := json.Marshal(verdict.Signals)
	_, err := w.DB.ExecContext(ctx, `INSERT INTO security_radar_events (target, target_type, module_id, source, signature, risk_index, risk_level, grade, verdict, recommendation, evidence, signals, rule_version, slot, created_at, updated_at) VALUES ($1,$2,$3,'alchemy_polling',$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12,$13,now(),now()) ON CONFLICT DO NOTHING`, source.Target, source.TargetType, source.ModuleID, signature, verdict.RiskIndex, verdict.RiskLevel, verdict.Grade, verdict.Verdict, verdict.Recommendation, string(evidence), string(signals), verdict.RuleVersion, sigInfo.Slot)
	return err
}

func (w *SecurityRadarWatcher) storeVerdict(ctx context.Context, source SecurityRadarSource, signature string, verdict SecurityRadarVerdict) error {
	evidence, _ := json.Marshal(verdict.Evidence)
	signals, _ := json.Marshal(verdict.Signals)
	_, err := w.DB.ExecContext(ctx, `INSERT INTO security_radar_verdicts (target, target_type, module_id, signature, risk_index, risk_level, grade, verdict, recommendation, evidence, signals, rule_version, source, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12,'alchemy_polling',now(),now()) ON CONFLICT (signature,module_id) DO UPDATE SET risk_index=EXCLUDED.risk_index, risk_level=EXCLUDED.risk_level, grade=EXCLUDED.grade, verdict=EXCLUDED.verdict, recommendation=EXCLUDED.recommendation, evidence=EXCLUDED.evidence, signals=EXCLUDED.signals, updated_at=now()`, source.Target, source.TargetType, source.ModuleID, verdict.Signature, verdict.RiskIndex, verdict.RiskLevel, verdict.Grade, verdict.Verdict, verdict.Recommendation, string(evidence), string(signals), verdict.RuleVersion)
	return err
}

func (w *SecurityRadarWatcher) upsertSource(ctx context.Context, source SecurityRadarSource) error {
	_, err := w.DB.ExecContext(ctx, `INSERT INTO security_radar_sources (module_id, name, target, target_type, provider, watch_mode, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,now(),now()) ON CONFLICT (module_id, target) DO UPDATE SET name=EXCLUDED.name, provider=EXCLUDED.provider, watch_mode=EXCLUDED.watch_mode, updated_at=now()`, source.ModuleID, source.Name, source.Target, source.TargetType, SecurityRadarProvider, SecurityRadarWatchMode)
	return err
}

func (w *SecurityRadarWatcher) updateCheckpoint(ctx context.Context, source SecurityRadarSource, signature string, slot uint64) error {
	_, err := w.DB.ExecContext(ctx, `UPDATE security_radar_sources SET last_seen_signature=$1, last_seen_slot=$2, updated_at=now() WHERE module_id=$3 AND target=$4`, signature, slot, source.ModuleID, source.Target)
	return err
}

func splitEnvList(raw string) []string {
	out := []string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func firstEnvValue(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func envWithDefault(defaultValue string, keys ...string) string {
	if value := firstEnvValue(keys...); value != "" {
		return value
	}
	return defaultValue
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
