package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type ownerUnifiedRadarRequest struct {
	Target        string `json:"target"`
	Address       string `json:"address"`
	Network       string `json:"network"`
	LiveEvidence  *bool  `json:"live_evidence,omitempty"`
	Court         *bool  `json:"court,omitempty"`
	ExtendedCourt *bool  `json:"extended_court,omitempty"`
}

// OwnerUnifiedRadarScan is the single owner-facing manual Radar entry point.
// Token targets join the existing 14 ARVIS arms, persistent actor memory and
// four deterministic market/holder behavior rules. Wallet targets use the same
// response contract but correctly mark token-only arms as not applicable.
func (h *Handler) OwnerUnifiedRadarScan(w http.ResponseWriter, r *http.Request) {
	var input ownerUnifiedRadarRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid request body")
		return
	}
	target := strings.TrimSpace(firstNonEmptyString(input.Target, input.Address))
	if target == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "target is required")
		return
	}
	network := strings.TrimSpace(input.Network)
	if network == "" {
		network = "solana-mainnet"
	}
	courtRequested := envBool("KOSCHEI_OWNER_COURT_AUTO_ENABLED", false)
	if input.Court != nil {
		courtRequested = *input.Court
	}
	extendedCourt := envBool("KOSCHEI_OWNER_COURT_EXTENDED", false)
	if input.ExtendedCourt != nil {
		extendedCourt = *input.ExtendedCourt
	}
	classification := classifyRadarTarget(r.Context(), target)
	switch classification.Type {
	case radarTargetTokenMint:
		h.ownerUnifiedTokenRadar(w, r, target, network, classification, courtRequested, extendedCourt)
	case radarTargetWallet, radarTargetTokenAccount:
		wallet := target
		if classification.Type == radarTargetTokenAccount {
			wallet = strings.TrimSpace(classification.TokenOwnerWallet)
		}
		if wallet == "" {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"ok": false, "error": "token_account_owner_unresolved",
				"target": target, "target_classification": classification,
				"message": "Token hesabının owner cüzdanı çözümlenemedi; birleşik Radar başlatılmadı.",
			})
			return
		}
		liveEvidence := true
		if input.LiveEvidence != nil {
			liveEvidence = *input.LiveEvidence
		}
		h.ownerUnifiedWalletRadar(w, r, target, wallet, network, classification, liveEvidence, courtRequested, extendedCourt)
	default:
		status := http.StatusUnprocessableEntity
		if classification.Type == radarTargetUnknown {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]any{
			"ok": false, "error": "unsupported_radar_target", "target": target,
			"network": network, "target_classification": classification,
			"message": "Tek Radar şu anda doğrulanmış token mint, wallet veya token-account hedefini kabul eder.",
		})
	}
}

func (h *Handler) ownerUnifiedTokenRadar(w http.ResponseWriter, r *http.Request, target, network string, classification radarTargetClassification, courtRequested, extendedCourt bool) {
	timeout := 180 * time.Second
	if courtRequested {
		timeout = 360 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	core := h.runHolderIntelligenceCore(ctx, target, network, "owner_unified_manual_scan")
	if services.SecurityRadarHasLiveEvidence(core.Bundle) {
		_ = h.saveSecurityRadarBundle(ctx, ownerChatIdentity(), "owner_unified_manual_scan", core.Bundle)
	}
	modules := radarDetailModules(core.Arms)
	legacyFinal := radarDetailFinalMap(core.Final, h.radarDetailPersistedVerdict(ctx, target))
	structural := h.radarDetailStructuralContext(ctx, target, network)
	warning := radarDetailWarning(legacyFinal, core.Distribution, structural, modules, core.SourceContext)
	graph := h.radarDetailGraph(ctx, target)
	creator := strings.TrimSpace(fmt.Sprint(core.SourceContext["creator_wallet"]))
	if creator == "<nil>" {
		creator = ""
	}

	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	actorDossier := services.ActorDefenseDossier{
		Wallet: creator, Network: network, Tokens: []services.ActorDefenseTokenObservation{},
		RelatedActors: []services.ActorDefenseRelatedActor{}, Evidence: []services.ActorDefenseEvidenceRecord{},
		Coverage: map[string]any{}, Policy: map[string]any{}, GeneratedAt: time.Now().UTC(),
	}
	actorTrack := services.ActorDefenseTrack{Network: network, TargetKind: "wallet", TargetID: creator, Dossier: map[string]any{}}
	actorStoreStatus := "creator_unavailable"
	var store *services.ActorDefenseStore
	if db != nil && creator != "" {
		store = services.NewActorDefenseStore(db)
		if loaded, err := store.LoadPersistentWalletDossier(ctx, creator, network, 150); err == nil {
			actorDossier = loaded
			actorTrack = loaded.Track
			actorStoreStatus = "loaded"
		} else {
			actorStoreStatus = "load_failed"
		}
	}

	now := time.Now().UTC()
	sales := services.LoadCreatorSellAcceleration(ctx, db, target, creator, now)
	sellVerification := services.VerifyCreatorSellTransactions(ctx, creatorIntelRPCURL(), sales)
	behavior := services.EvaluateUnifiedRadarBehavior(target, creator, core.Market, core.Intelligence, core.Cluster, sales, now)
	behavior = services.HardenUnifiedRadarBehavior(behavior, sellVerification, core.Cluster)
	threatAnticipation := services.BuildThreatAnticipation(services.ThreatAnticipationInput{
		Target: target, Market: core.Market, Holder: core.Intelligence, Cluster: core.Cluster,
		Arms: core.Arms, Behavior: behavior,
	})
	behaviorPersistence := "not_applicable"
	if store != nil && len(behavior.Evidence) > 0 {
		behaviorPersistence = "persisted"
		for _, item := range behavior.Evidence {
			item.Network = network
			if err := store.UpsertEvidence(ctx, item); err != nil {
				behaviorPersistence = "partial_failure"
			}
		}
		if loaded, err := store.LoadPersistentWalletDossier(ctx, creator, network, 150); err == nil {
			actorDossier = loaded
			actorTrack = loaded.Track
		}
	}
	combinedEvidence := append([]services.ActorDefenseEvidenceRecord{}, actorDossier.Evidence...)
	combinedEvidence = append(combinedEvidence, behavior.Evidence...)
	actorVerdict := services.EvaluateActorDefenseRules(actorTrack, combinedEvidence)
	actorVerdictPersistence := "not_applicable"
	if store != nil && strings.TrimSpace(actorTrack.TargetID) != "" {
		actorVerdictPersistence = "persisted"
		if err := store.PersistRuleVerdict(ctx, actorTrack, actorVerdict); err != nil {
			actorVerdictPersistence = "failed"
		}
	}
	unifiedVerdict := services.EvaluateUnifiedRadarVerdict(target, actorVerdict, behavior)
	unifiedPersistence, unifiedHistory := h.persistUnifiedRadarVerdict(ctx, db, network, "token", target, unifiedVerdict, behavior)
	courtInput := CourtReadOnlyInput{
		Target: target,
		Network: network,
		SignedVerdict: unifiedVerdict,
		VerdictCard: map[string]any{"grade": unifiedVerdict.Grade, "verdict": unifiedVerdict.Verdict, "signed": unifiedVerdict.Signed, "signature": unifiedVerdict.Signature},
		EvidencePacket: map[string]any{
			"threat_anticipation": threatAnticipation,
			"behavior_signals": behavior,
			"actor_rule_verdict": actorVerdict,
			"actor_evidence": combinedEvidence,
			"holder_intelligence": core.Intelligence,
			"holder_cluster": core.Cluster,
			"market": core.Market,
			"modules": modules,
			"evidence": radarDetailEvidence(core.Arms),
			"graph": graph,
		},
	}
	var court *CourtReport
	if courtRequested {
		courtCtx := context.WithValue(ctx, courtTierOverrideKey{}, "enterprise")
		court = h.courtNarrative(courtCtx, courtInput, extendedCourt)
		if court == nil {
			court = ownerCourtUnavailableReport("disabled")
		}
	}

	legacy := map[string]any{
		"architecture_arm_count": 14,
		"final_verdict": legacyFinal,
		"warning": warning,
		"holder_distribution": core.Distribution,
		"holder_intelligence": core.Intelligence,
		"holder_cluster": core.Cluster,
		"launch_forensics": core.LaunchForensics,
		"market": core.Market,
		"structural_memory": structural,
		"source_context": core.SourceContext,
		"modules": modules,
		"evidence": radarDetailEvidence(core.Arms),
		"graph": graph,
		"compatibility_note": "Legacy arm risk indexes remain internal module diagnostics; the unified final verdict is letter-only and deterministic.",
	}
	response := map[string]any{
		"ok": true,
		"schema_version": "koschei-unified-radar-v1",
		"target": target,
		"network": network,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"target_classification": classification,
		"analysis_scope": "token_plus_actor_plus_market_behavior",
		"manual_only": true,
		"automatic_scanning": false,
		"architecture": map[string]any{"legacy_arvis_arms": 14, "actor_investigation": true, "behavior_rules": 4, "single_final_verdict": true, "court_requested": courtRequested},
		"final_verdict": unifiedVerdict,
		"final_verdict_persistence": unifiedPersistence,
		"final_verdict_history": unifiedHistory,
		"threat_anticipation": threatAnticipation,
		"legacy_14_arm_radar": legacy,
		"actor_investigation": map[string]any{
			"wallet": creator,
			"dossier": actorDossier,
			"rule_verdict": actorVerdict,
			"store_status": actorStoreStatus,
			"rule_verdict_persistence": actorVerdictPersistence,
		},
		"behavior_signals": behavior,
		"creator_sell_transaction_verification": sellVerification,
		"behavior_evidence_persistence": behaviorPersistence,
		"evidence_policy": map[string]any{
			"court_receives_signed_verdict_read_only": true,
			"court_returns_narrative_only": true,
			"numeric_final_score_disabled": true,
			"numeric_rug_probability_disabled": true,
			"threat_capacity_is_not_intent": true,
			"no_evidence_no_claim": true,
			"inferred_watch_only": true,
			"unverified_excluded": true,
			"ai_may_explain_but_cannot_grade": true,
			"creator_sell_acceleration_is_observed": true,
			"verified_behavior_evidence_requires_complete_canonical_line": true,
		},
	}
	if court != nil {
		response["court"] = court
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ownerUnifiedWalletRadar(w http.ResponseWriter, r *http.Request, requestedTarget, wallet, network string, classification radarTargetClassification, liveEvidence, courtRequested, extendedCourt bool) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Unified Radar database is unavailable")
		return
	}
	timeout := 180 * time.Second
	if courtRequested {
		timeout = 360 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	store := services.NewActorDefenseStore(db)
	initial, err := store.LoadPersistentWalletDossier(ctx, wallet, network, 150)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Actor dossier could not be assembled")
		return
	}
	coverage := actorDefenseLiveCoverage{Status: "stored_evidence_only", Limitations: []string{}}
	funding := services.ActorFundingOrigin{Wallet: wallet, Status: "stored_evidence_only", VerificationStatus: "unverified", TrailStatus: "not_investigated", IdentityScope: "onchain_wallet_only", Limitations: []string{}}
	fundingPersistence := "not_requested"
	if liveEvidence {
		funding, fundingPersistence = h.collectActorFundingOrigin(ctx, store, wallet, network)
		coverage = h.collectActorDefenseLiveEvidence(ctx, store, initial)
	}
	final, err := store.LoadPersistentWalletDossier(ctx, wallet, network, 200)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Actor dossier could not be refreshed")
		return
	}
	actorVerdict := services.EvaluateActorDefenseRules(final.Track, final.Evidence)
	persistence := "persisted"
	if err := store.PersistRuleVerdict(ctx, final.Track, actorVerdict); err != nil {
		persistence = "failed"
	}
	behavior := services.EvaluateUnifiedRadarBehavior("", wallet, services.TokenMarketSnapshot{}, services.HolderIntelligence{}, services.HolderClusterAnalysis{}, services.CreatorSellAcceleration{}, time.Now().UTC())
	unifiedVerdict := services.EvaluateUnifiedRadarVerdict(wallet, actorVerdict, behavior)
	unifiedPersistence, unifiedHistory := h.persistUnifiedRadarVerdict(ctx, db, network, "wallet", wallet, unifiedVerdict, behavior)
	var court *CourtReport
	if courtRequested {
		courtInput := CourtReadOnlyInput{
			Target: wallet,
			Network: network,
			SignedVerdict: unifiedVerdict,
			VerdictCard: map[string]any{"grade": unifiedVerdict.Grade, "verdict": unifiedVerdict.Verdict, "signed": unifiedVerdict.Signed, "signature": unifiedVerdict.Signature},
			EvidencePacket: map[string]any{"actor_dossier": final, "funding_origin": funding, "live_evidence": coverage, "actor_rule_verdict": actorVerdict, "behavior_signals": behavior},
		}
		courtCtx := context.WithValue(ctx, courtTierOverrideKey{}, "enterprise")
		court = h.courtNarrative(courtCtx, courtInput, extendedCourt)
		if court == nil {
			court = ownerCourtUnavailableReport("disabled")
		}
	}
	response := map[string]any{
		"ok": true,
		"schema_version": "koschei-unified-radar-v1",
		"target": requestedTarget,
		"wallet": wallet,
		"network": network,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"target_classification": classification,
		"analysis_scope": "wallet_actor_investigation",
		"manual_only": true,
		"automatic_scanning": false,
		"architecture": map[string]any{
			"legacy_arvis_arms": 14,
			"legacy_arms_applicability": "token_mint_required",
			"actor_investigation": true,
			"behavior_rules": 4,
			"behavior_rules_applicability": "token_mint_required",
			"single_final_verdict": true,
			"court_requested": courtRequested,
		},
		"final_verdict": unifiedVerdict,
		"final_verdict_persistence": unifiedPersistence,
		"final_verdict_history": unifiedHistory,
		"legacy_14_arm_radar": map[string]any{"applicable": false, "reason": "The legacy 14 token arms require a token mint. They remain part of the same Radar architecture but are not fabricated for wallet targets.", "modules": []any{}},
		"actor_investigation": map[string]any{
			"wallet": wallet,
			"dossier": final,
			"funding_origin": funding,
			"funding_origin_persistence": fundingPersistence,
			"live_evidence": coverage,
			"rule_verdict": actorVerdict,
			"rule_verdict_persistence": persistence,
		},
		"behavior_signals": behavior,
		"evidence_policy": map[string]any{
			"numeric_final_score_disabled": true,
			"no_evidence_no_claim": true,
			"inferred_watch_only": true,
			"unverified_excluded": true,
			"ai_may_explain_but_cannot_grade": true,
		},
	}
	if court != nil {
		response["court"] = court
	}
	writeJSON(w, http.StatusOK, response)
}

func ownerCourtUnavailableReport(status string) *CourtReport {
	return &CourtReport{
		Status: status,
		TierApplied: "enterprise",
		Authority: "the signed deterministic verdict is final; court output is commentary/explanation",
		GeneratedAt: time.Now().UTC(),
	}
}

func (h *Handler) persistUnifiedRadarVerdict(ctx context.Context, db *sql.DB, network, targetKind, targetID string, verdict services.UnifiedRadarVerdict, behavior services.UnifiedRadarBehaviorReport) (string, []services.UnifiedRadarVerdictHistoryRecord) {
	if db == nil {
		return "database_unavailable", []services.UnifiedRadarVerdictHistoryRecord{}
	}
	store := services.NewUnifiedRadarVerdictStore(db)
	status := "persisted"
	if _, err := store.Persist(ctx, network, targetKind, targetID, verdict, behavior); err != nil {
		status = "failed"
	}
	history, err := store.History(ctx, network, targetKind, targetID, 20)
	if err != nil {
		if status == "persisted" {
			status = "history_read_failed"
		}
		return status, []services.UnifiedRadarVerdictHistoryRecord{}
	}
	return status, history
}
