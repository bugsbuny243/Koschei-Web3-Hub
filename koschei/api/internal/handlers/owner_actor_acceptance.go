package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type actorAcceptanceRequest struct {
	Target       string `json:"target"`
	Address      string `json:"address"`
	Network      string `json:"network"`
	LiveEvidence *bool  `json:"live_evidence,omitempty"`
}

// OwnerActorAcceptance executes the existing wallet-first actor investigation
// collectors and evaluates ACTOR_INVESTIGATION_ENGINE.md section 8 as ten
// explicit pass/fail/not_investigated items. It does not create a numeric score,
// broaden recipient history, or grant AI any verdict authority.
func (h *Handler) OwnerActorAcceptance(w http.ResponseWriter, r *http.Request) {
	var input actorAcceptanceRequest
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

	classification := classifyRadarTarget(r.Context(), target)
	wallet := target
	switch classification.Type {
	case radarTargetWallet:
	case radarTargetTokenAccount:
		wallet = strings.TrimSpace(classification.TokenOwnerWallet)
		if wallet == "" {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"ok": false, "error": "token_account_owner_unresolved", "target": target,
				"target_classification": classification,
			})
			return
		}
	default:
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"ok": false, "error": "wallet_target_required", "target": target,
			"target_classification": classification,
		})
		return
	}
	if h == nil || h.DB == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Actor defense database is unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	store := services.NewActorDefenseStore(h.DB)
	initial, err := store.LoadPersistentWalletDossier(ctx, wallet, network, 100)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Actor defense dossier could not be assembled")
		return
	}

	liveEnabled := true
	if input.LiveEvidence != nil {
		liveEnabled = *input.LiveEvidence
	}
	coverage := actorDefenseLiveCoverage{Status: "stored_evidence_only", Limitations: []string{}}
	fundingOrigin := services.ActorFundingOrigin{
		Wallet: wallet, Status: "stored_evidence_only", VerificationStatus: "unverified",
		TrailStatus: "not_investigated", IdentityScope: "onchain_wallet_only", Limitations: []string{},
	}
	fundingPersistence := "not_requested"
	if liveEnabled {
		fundingOrigin, fundingPersistence = h.collectActorFundingOrigin(ctx, store, wallet, network)
		coverage = h.collectActorDefenseLiveEvidence(ctx, store, initial)
		if fundingPersistence == "failed" {
			coverage.PersistenceFailures++
			if coverage.Status == "complete" {
				coverage.Status = "partial_persistence"
			}
			coverage.Limitations = append(coverage.Limitations, "Funding-origin evidence could not be persisted.")
		}
	}

	final, err := store.LoadPersistentWalletDossier(ctx, wallet, network, 200)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Actor defense dossier could not be refreshed")
		return
	}
	verdict := services.EvaluateActorDefenseRules(final.Track, final.Evidence)
	verdictPersistence := "persisted"
	if err := store.PersistRuleVerdict(ctx, final.Track, verdict); err != nil {
		verdictPersistence = "failed"
		coverage.PersistenceFailures++
		coverage.Limitations = append(coverage.Limitations, "Deterministic actor verdict could not be persisted.")
	}

	acceptance := services.EvaluateActorAcceptance(services.ActorAcceptanceInput{
		Wallet: wallet, Network: network, TargetKind: "wallet", Dossier: final,
		FundingOrigin: fundingOrigin, Verdict: verdict,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"schema_version": services.ActorAcceptanceContractVersion,
		"target": target,
		"wallet": wallet,
		"network": network,
		"target_classification": classification,
		"live_evidence": coverage,
		"funding_origin_persistence": fundingPersistence,
		"rule_verdict_persistence": verdictPersistence,
		"acceptance": acceptance,
	})
}
