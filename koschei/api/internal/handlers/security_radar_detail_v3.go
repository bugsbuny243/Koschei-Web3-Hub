package handlers

import (
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

// SecurityRadarDetailV3 returns the complete premium Radar investigation with
// actor intelligence joined into the same evidence contract. It reuses fresh
// holder evidence and never treats a balance alone as identity or wrongdoing.
func (h *Handler) SecurityRadarDetailV3(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(firstNonEmptyString(r.URL.Query().Get("target"), r.URL.Query().Get("mint"), r.URL.Query().Get("address")))
	if target == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "target is required")
		return
	}
	network := strings.TrimSpace(r.URL.Query().Get("network"))
	if network == "" {
		network = "solana-mainnet"
	}
	classification := classifyRadarTarget(r.Context(), target)
	if !radarTargetTokenVerdictAllowed(classification) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"ok": false, "error": "target_not_token_mint", "message": radarTargetRejectionMessage(classification),
			"target": target, "target_classification": classification,
			"final_verdict": map[string]any{"risk_index": nil, "risk_level": "unknown", "grade": "-", "signed": false, "verdict": "INSUFFICIENT EVIDENCE"},
		})
		return
	}

	analysis := services.AnalyzeArvisRadars(services.SecurityRadarRequest{Target: target, Network: network, Mode: "manual_detail"})
	bundle := services.EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	arms := services.ArvisArmsFromBundle(bundle)
	if len(arms) == 0 {
		arms = analysis.Arms
	}
	freshFinal := services.ArvisFinalFromBundle(bundle)
	holderRoles := services.ArvisHolderRolesFromBundle(bundle)
	distribution := radarDetailHolderDistributionFromRoles(holderRoles)
	if !holderRoles.Available {
		distribution, holderRoles = radarDetailHolderDistribution(r.Context(), target)
	}
	holderCluster := services.ArvisHolderClusterFromBundle(bundle)
	market := radarDetailMarketSnapshot(r.Context(), target)
	holderIntelligence := services.BuildHolderIntelligence(holderRoles, holderCluster, market, time.Now().UTC())
	sourceContext := h.radarDetailSourceContext(r.Context(), target, network)
	actorIntelligence := h.actorSecurityIntelligenceForDetail(r.Context(), target, network, sourceContext, holderRoles, holderCluster, market)
	structural := h.radarDetailStructuralContext(r.Context(), target, network)
	persisted := h.radarDetailPersistedVerdict(r.Context(), target)
	final := radarDetailFinalMap(freshFinal, persisted)
	modules := radarDetailModules(arms)
	allEvidence := radarDetailEvidence(arms)
	warning := radarDetailWarning(final, distribution, structural, modules, sourceContext)
	graph := h.radarDetailGraph(r.Context(), target)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                   true,
		"schema_version":       "koschei-radar-detail-v3",
		"target":               target,
		"network":              network,
		"generated_at":         time.Now().UTC().Format(time.RFC3339),
		"final_verdict":        final,
		"warning":              warning,
		"holder_distribution":  distribution,
		"holder_intelligence":  holderIntelligence,
		"holder_cluster":       holderCluster,
		"actor_intelligence":   actorIntelligence,
		"market":               market,
		"structural_memory":    structural,
		"source_context":       sourceContext,
		"modules":              modules,
		"evidence":             allEvidence,
		"graph":                graph,
		"evidence_policy": map[string]any{
			"hide_verified_details": false,
			"no_evidence_no_claim":  true,
			"creator_wallet_scope":  "source-reported or on-chain relation; not proof of wrongdoing or real-world identity",
			"actor_relation_scope":  "wallet funding, token flow and launch relations are evidence-scoped; no real-world identity claim",
			"financial_advice":      false,
		},
	})
}
