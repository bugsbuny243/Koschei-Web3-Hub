package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

type PumpPortalRadarAdapter struct {
	Store *SecurityRadarStore
}

func NewPumpPortalRadarAdapter(store *SecurityRadarStore) *PumpPortalRadarAdapter {
	return &PumpPortalRadarAdapter{Store: store}
}

func (a *PumpPortalRadarAdapter) HandleEvent(ctx context.Context, ev PumpPortalEvent) error {
	if a == nil || a.Store == nil || a.Store.DB == nil {
		return nil
	}
	mint := resolvePumpPortalMint(ev)
	if mint == "" {
		return nil
	}
	signature := strings.TrimSpace(ev.Signature)
	creator := strings.TrimSpace(ev.Creator)
	if signature != "" {
		inserted, err := a.Store.MarkSignatureSeen(ctx, ModulePumpSybilRadar, signature, firstNonEmptyPumpPortal(creator, ev.Trader), "solana-mainnet")
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
	}

	eventType := pumpPortalEventType(ev)
	signals := pumpPortalSignals(ev, mint, signature, eventType)
	rawSummary := map[string]any{
		"source":      "pumpportal",
		"event_type":  eventType,
		"mint":        mint,
		"name":        ev.Name,
		"symbol":      ev.Symbol,
		"creator":     creator,
		"trader":      ev.Trader,
		"tx_type":     ev.TxType,
		"signature":   signature,
		"received_at": ev.ReceivedAt,
		"raw":         ev.Raw,
	}
	eventID, err := a.Store.InsertEvent(ctx, SecurityRadarEventRecord{
		ModuleID:      ModulePumpSybilRadar,
		Target:        mint,
		TargetType:    "token",
		Network:       "solana-mainnet",
		Signature:     signature,
		SourceAddress: firstNonEmptyPumpPortal(creator, ev.Trader),
		EventType:     eventType,
		Signals:       signals,
		RawSummary:    rawSummary,
		Source:        "pumpportal",
	})
	if err != nil {
		return err
	}

	// A PumpPortal discovery must become a complete ARVIS story, not an isolated
	// pump module row. The live-stream mode enables the verified Pump arm while
	// the same run also evaluates holder, authority, graph, timing and program
	// evidence. Signed arms and the final verdict share the source event.
	analysis := AnalyzeArvisRadars(SecurityRadarRequest{
		Target:  mint,
		Network: "solana-mainnet",
		Mode:    "live_stream:" + ModulePumpSybilRadar,
	})
	bundle := EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	arms := ArvisArmsFromBundle(bundle)
	if len(arms) == 0 {
		arms = analysis.Arms
	}

	var firstErr error
	inserted := 0
	for _, arm := range arms {
		if !arm.Signed || !pumpPortalArmVerified(arm) {
			continue
		}
		arm.Signals = mergePumpPortalSignals(arm.Signals, signals)
		arm.Signals["source_verified_program_event"] = true
		arm.Signals["source_module"] = ModulePumpSybilRadar
		arm.Signals["customer_detail_visible"] = true
		if arm.ModuleID == ModuleFinalVerdictEngine {
			arm.Signals["warning_label"] = pumpPortalWarningLabel(arm.RiskIndex)
			arm.Signals["warning_scope"] = "token and source-reported creator/deployer wallet; evidence-based risk signal, not an identity accusation"
		}
		arm.Evidence = append(arm.Evidence,
			"PumpPortal reported a Pump token discovery event for this mint.",
		)
		if isLikelySolanaAddress(creator) {
			arm.Evidence = append(arm.Evidence, fmt.Sprintf("Source-reported creator/deployer wallet: %s.", creator))
		}
		if _, err := a.Store.InsertVerdict(ctx, SecurityRadarVerdictRecord{
			EventID:        eventID,
			ModuleID:       arm.ModuleID,
			Target:         arm.Target,
			TargetType:     "token",
			Network:        arm.Network,
			Grade:          arm.Grade,
			RiskIndex:      arm.RiskIndex,
			RiskLevel:      arm.RiskLevel,
			Verdict:        arm.Verdict,
			Recommendation: arm.Recommendation,
			Evidence:       arm.Evidence,
			Signals:        arm.Signals,
			RuleVersion:    arm.RuleVersion,
			Signed:         arm.Signed,
			Signature:      firstNonEmptyPumpPortal(arm.Signature, signature),
			Source:         "pumpportal",
			EventType:      eventType,
			Provider:       "alchemy+pumpportal",
		}); err != nil && firstErr == nil {
			firstErr = err
		} else if err == nil {
			inserted++
		}
	}
	if inserted == 0 && firstErr == nil {
		log.Printf("pumpportal radar discovery stored without signed ARVIS verdict: mint=%s", mint)
	}
	return firstErr
}

func pumpPortalSignals(ev PumpPortalEvent, mint, signature, eventType string) map[string]any {
	creator := strings.TrimSpace(ev.Creator)
	signals := map[string]any{
		"source":                     "pumpportal",
		"provider":                   "pumpportal",
		"event_type":                 eventType,
		"launch_platform":            "pump.fun",
		"mint":                       mint,
		"name":                       ev.Name,
		"token_name":                 ev.Name,
		"symbol":                     ev.Symbol,
		"token_symbol":               ev.Symbol,
		"creator":                    creator,
		"trader":                     ev.Trader,
		"tx_type":                    ev.TxType,
		"signature":                  signature,
		"received_at":                ev.ReceivedAt,
		"creator_identity_claimed":   false,
		"creator_relation_scope":     "source-reported launch metadata",
		"customer_detail_visible":    true,
		"source_verified_pump_event": true,
	}
	if isLikelySolanaAddress(creator) {
		signals["creator_wallet"] = creator
		signals["deployer_wallet"] = creator
		signals["creator_wallet_source"] = "pumpportal_event_metadata"
	}
	return signals
}

func pumpPortalArmVerified(arm SecurityRadarVerdict) bool {
	if !arm.Signed || arm.Signals == nil {
		return false
	}
	for _, key := range []string{"verified_evidence", "real_onchain_evidence", "real_offchain_evidence"} {
		if value, _ := arm.Signals[key].(bool); value {
			return true
		}
	}
	return false
}

func pumpPortalWarningLabel(risk int) string {
	switch {
	case risk >= 65:
		return "HIGH_RISK_WARNING"
	case risk >= 35:
		return "WARNING"
	default:
		return "MONITOR"
	}
}

func StartPumpPortalRadarIfEnabled(ctx context.Context, db *sql.DB) func() {
	cfg := LoadPumpPortalConfigFromEnv()
	if !cfg.Enabled {
		return func() {}
	}
	if db == nil {
		log.Printf("pumpportal radar disabled: database unavailable")
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	store := NewSecurityRadarStore(db)
	adapter := NewPumpPortalRadarAdapter(store)
	client := NewPumpPortalClient(cfg)
	go client.Start(ctx, adapter.HandleEvent)
	log.Printf("pumpportal radar discovery started: data-only websocket=%s", cfg.redactedWebsocketHost())
	return cancel
}

func resolvePumpPortalMint(ev PumpPortalEvent) string {
	for _, candidate := range []string{ev.Mint, ev.TokenMint} {
		candidate = strings.TrimSpace(candidate)
		if isLikelySolanaAddress(candidate) {
			return candidate
		}
	}
	if ev.Raw != nil {
		for _, key := range []string{"mint", "tokenMint", "ca", "address"} {
			if value, ok := ev.Raw[key].(string); ok && isLikelySolanaAddress(value) {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func pumpPortalEventType(ev PumpPortalEvent) string {
	value := strings.ToLower(strings.TrimSpace(firstNonEmptyPumpPortal(ev.Type, ev.TxType)))
	if strings.Contains(value, "migrat") {
		return "pumpportal_migration"
	}
	return "pumpportal_new_token"
}

func mergePumpPortalSignals(base map[string]any, extra map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for k, v := range extra {
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		base[k] = v
	}
	base["pumpportal_discovered_at"] = time.Now().UTC()
	return base
}

func firstNonEmptyPumpPortal(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
