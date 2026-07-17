package handlers

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const unifiedInvestigationSchemaVersion = "koschei-unified-investigation-v1"

type unifiedInvestigationAssembly struct {
	Report           map[string]any
	Core             holderIntelligenceCoreResult
	DB               *sql.DB
	Store            *services.ActorDefenseStore
	Creator          string
	ActorDossier     services.ActorDefenseDossier
	ActorTrack       services.ActorDefenseTrack
	ActorVerdict     services.ActorDefenseRuleVerdict
	Behavior         services.UnifiedRadarBehaviorReport
	UnifiedVerdict   services.UnifiedRadarVerdict
	Threat           services.ThreatAnticipationReport
	CombinedEvidence []services.ActorDefenseEvidenceRecord
	Modules          []map[string]any
	Structural       map[string]any
	Graph            any
	TradeLedger      map[string]any
	ActorStoreStatus string
}

// buildUnifiedInvestigationReport runs the shared evidence engine used by public,
// authenticated, owner and API callers. Caller type is intentionally absent from
// the technical result. Operational metadata is added outside Report.
func (h *Handler) buildUnifiedInvestigationReport(ctx context.Context, target, network, mode string) unifiedInvestigationAssembly {
	core := h.runHolderIntelligenceCore(ctx, target, network, mode)
	return h.assembleUnifiedInvestigationReport(ctx, core)
}

func (h *Handler) assembleUnifiedInvestigationReport(ctx context.Context, core holderIntelligenceCoreResult) unifiedInvestigationAssembly {
	target := strings.TrimSpace(core.Request.Target)
	network := strings.TrimSpace(core.Request.Network)
	if network == "" { network = "solana-mainnet" }
	now := time.Now().UTC()
	creator := strings.TrimSpace(creatorIntelCleanString(core.SourceContext["creator_wallet"]))

	db := h.DBRead
	if db == nil { db = h.DB }
	actorDossier := services.ActorDefenseDossier{
		Wallet: creator, Network: network,
		Tokens: []services.ActorDefenseTokenObservation{}, RelatedActors: []services.ActorDefenseRelatedActor{},
		Evidence: []services.ActorDefenseEvidenceRecord{}, Coverage: map[string]any{}, Policy: map[string]any{}, GeneratedAt: now,
	}
	actorTrack := services.ActorDefenseTrack{Network: network, TargetKind: "wallet", TargetID: creator, Dossier: map[string]any{}}
	actorStoreStatus := "creator_unavailable"
	var store *services.ActorDefenseStore
	if db != nil && creator != "" {
		store = services.NewActorDefenseStore(db)
		if loaded, err := store.LoadPersistentWalletDossier(ctx, creator, network, 150); err == nil {
			actorDossier, actorTrack, actorStoreStatus = loaded, loaded.Track, "loaded"
		} else { actorStoreStatus = "load_failed" }
	}

	sales := services.LoadCreatorSellAcceleration(ctx, db, target, creator, now)
	storedVerification := services.CreatorSellVerification{
		CandidateSignatures: append([]string{}, sales.Signatures...), VerifiedSignatures: []string{},
		Limitations: []string{"Transaction-level creator-sell verification was not requested by this stored-data report assembly."},
	}
	behavior := services.EvaluateUnifiedRadarBehavior(target, creator, core.Market, core.Intelligence, core.Cluster, sales, now)
	behavior = services.HardenUnifiedRadarBehavior(behavior, storedVerification, core.Cluster)
	behavior = services.ApplyOwnerConcentrationRuleV110(behavior, core.Intelligence, now)
	threat := services.BuildThreatAnticipation(services.ThreatAnticipationInput{
		Target: target, Market: core.Market, Holder: core.Intelligence, Cluster: core.Cluster,
		Arms: core.Arms, Behavior: behavior,
	})
	combinedEvidence := append([]services.ActorDefenseEvidenceRecord{}, actorDossier.Evidence...)
	combinedEvidence = append(combinedEvidence, behavior.Evidence...)
	actorVerdict := services.EvaluateActorDefenseRules(actorTrack, combinedEvidence)
	unifiedVerdict := services.EvaluateUnifiedRadarVerdictV110(target, actorVerdict, behavior)
	modules := radarDetailModules(core.Arms)
	coverage := services.BuildArvisInvestigationCoverage(core.Arms)
	structural := h.radarDetailStructuralContext(ctx, target, network)
	graph := h.radarDetailGraph(ctx, target)
	tradeLedger := h.unifiedTradeLedgerAggregates(ctx, target)
	transactionEvidence := h.loadUnifiedTransactionEvidence(ctx, target, 50)
	evidenceReferences := buildUnifiedEvidenceReferences(core, creator, transactionEvidence, behavior, unifiedVerdict)

	report := map[string]any{
		"ok": true, "schema_version": unifiedInvestigationSchemaVersion,
		"target": target, "network": network, "generated_at": now.Format(time.RFC3339),
		"analysis_scope": "token_plus_actor_plus_market_behavior",
		"final_verdict": unifiedVerdict, "threat_anticipation": threat,
		"investigation_coverage": coverage, "holder_distribution": core.Distribution,
		"holder_intelligence": core.Intelligence, "holder_cluster": core.Cluster,
		"launch_forensics": core.LaunchForensics, "market": core.Market,
		"lp_control": core.LPControl, "jupiter_market_context": core.JupiterContext,
		"source_context": core.SourceContext, "structural_memory": structural,
		"modules": modules, "evidence_arms": modules, "evidence": radarDetailEvidence(core.Arms),
		"behavior_signals": behavior, "trade_ledger_aggregates": tradeLedger,
		"transaction_evidence": transactionEvidence, "evidence_references": evidenceReferences,
		"actor_investigation": map[string]any{
			"wallet": creator, "dossier": actorDossier, "rule_verdict": actorVerdict, "store_status": actorStoreStatus,
		},
		"graph": graph,
		"investigation_output_policy": services.SharedInvestigationOutputPolicy(),
		"evidence_policy": map[string]any{
			"numeric_final_score_disabled": true, "numeric_rug_probability_disabled": true,
			"threat_capacity_is_not_intent": true, "no_evidence_no_claim": true,
			"identity_scope": "onchain_wallet_only", "caller_type_changes_evidence": false,
			"jupiter_context_can_change_verdict": false, "lp_control_arm_can_change_grade": false,
		},
	}
	return unifiedInvestigationAssembly{
		Report: report, Core: core, DB: db, Store: store, Creator: creator,
		ActorDossier: actorDossier, ActorTrack: actorTrack, ActorVerdict: actorVerdict,
		Behavior: behavior, UnifiedVerdict: unifiedVerdict, Threat: threat,
		CombinedEvidence: combinedEvidence, Modules: modules, Structural: structural,
		Graph: graph, TradeLedger: tradeLedger, ActorStoreStatus: actorStoreStatus,
	}
}

func (h *Handler) unifiedTradeLedgerAggregates(ctx context.Context, mint string) map[string]any {
	out := map[string]any{
		"available": false, "status": "monitoring_window_active", "trade_count": int64(0),
		"buy_count": int64(0), "sell_count": int64(0), "unique_trader_count": int64(0),
		"round_trip_wallet_count": int64(0), "wash_classification": "not_proven",
	}
	db := h.DBRead
	if db == nil { db = h.DB }
	if db == nil || strings.TrimSpace(mint) == "" {
		out["status"] = "trade_ledger_unavailable"
		return out
	}
	var tradeCount, buyCount, sellCount, uniqueTraders, roundTrip int64
	var firstSeen, lastSeen sql.NullTime
	err := db.QueryRowContext(ctx, `
		WITH per_trader AS (
			SELECT trader, bool_or(side='buy') AS bought, bool_or(side='sell') AS sold
			FROM token_trade_events WHERE mint=$1 GROUP BY trader
		)
		SELECT
			(SELECT count(*) FROM token_trade_events WHERE mint=$1),
			(SELECT count(*) FROM token_trade_events WHERE mint=$1 AND side='buy'),
			(SELECT count(*) FROM token_trade_events WHERE mint=$1 AND side='sell'),
			(SELECT count(*) FROM per_trader),
			(SELECT count(*) FROM per_trader WHERE bought AND sold),
			(SELECT min(COALESCE(block_time,created_at)) FROM token_trade_events WHERE mint=$1),
			(SELECT max(COALESCE(block_time,created_at)) FROM token_trade_events WHERE mint=$1)`, mint).Scan(
		&tradeCount, &buyCount, &sellCount, &uniqueTraders, &roundTrip, &firstSeen, &lastSeen,
	)
	if err != nil { out["status"] = "trade_ledger_query_failed"; return out }
	out["available"], out["status"] = tradeCount > 0, "observed_trade_ledger_aggregates"
	out["trade_count"], out["buy_count"], out["sell_count"] = tradeCount, buyCount, sellCount
	out["unique_trader_count"], out["round_trip_wallet_count"] = uniqueTraders, roundTrip
	out["wash_classification"] = "not_proven"
	out["interpretation"] = "Round-trip wallets are an investigation context signal; they are not, by themselves, proof of wash trading."
	if firstSeen.Valid { out["first_observed_at"] = firstSeen.Time.UTC().Format(time.RFC3339) }
	if lastSeen.Valid { out["last_observed_at"] = lastSeen.Time.UTC().Format(time.RFC3339) }
	return out
}
