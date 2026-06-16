package services

import (
	"context"
	"database/sql"
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
	if signature != "" {
		inserted, err := a.Store.MarkSignatureSeen(ctx, ModulePumpSybilRadar, signature, firstNonEmptyPumpPortal(ev.Creator, ev.Trader), "solana-mainnet")
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
	}
	eventType := pumpPortalEventType(ev)
	signals := map[string]any{
		"source":      "pumpportal",
		"provider":    "pumpportal",
		"event_type":  eventType,
		"mint":        mint,
		"name":        ev.Name,
		"symbol":      ev.Symbol,
		"creator":     ev.Creator,
		"trader":      ev.Trader,
		"tx_type":     ev.TxType,
		"signature":   signature,
		"received_at": ev.ReceivedAt,
	}
	rawSummary := map[string]any{
		"source":      "pumpportal",
		"event_type":  eventType,
		"mint":        mint,
		"name":        ev.Name,
		"symbol":      ev.Symbol,
		"creator":     ev.Creator,
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
		SourceAddress: firstNonEmptyPumpPortal(ev.Creator, ev.Trader),
		EventType:     eventType,
		Signals:       signals,
		RawSummary:    rawSummary,
		Source:        "pumpportal",
	})
	if err != nil {
		return err
	}
	bundle := AnalyzeSecurityRadars(SecurityRadarRequest{Target: mint, Network: "solana-mainnet", Mode: "pumpportal_discovery"})
	verdict := bundle.PumpSybilRadar
	verdict.Signals = mergePumpPortalSignals(verdict.Signals, signals)
	_, err = a.Store.InsertVerdict(ctx, SecurityRadarVerdictRecord{
		EventID:        eventID,
		ModuleID:       verdict.ModuleID,
		Target:         verdict.Target,
		TargetType:     "token",
		Network:        verdict.Network,
		Grade:          verdict.Grade,
		RiskIndex:      verdict.RiskIndex,
		RiskLevel:      verdict.RiskLevel,
		Verdict:        verdict.Verdict,
		Recommendation: verdict.Recommendation,
		Evidence:       verdict.Evidence,
		Signals:        verdict.Signals,
		RuleVersion:    verdict.RuleVersion,
		Signed:         verdict.Signed,
		Signature:      firstNonEmptyPumpPortal(verdict.Signature, signature),
		Source:         "pumpportal",
		EventType:      eventType,
		Provider:       "alchemy+pumpportal",
	})
	return err
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
