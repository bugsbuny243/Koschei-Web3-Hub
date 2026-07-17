package handlers

import (
	"net/http"
	"strings"
)

// SecurityRadarDetailV3 returns the same technical investigation contract used
// by public, owner and API callers. Caller type changes operational capacity,
// never evidence interpretation.
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
			"final_verdict": map[string]any{"grade": "-", "signed": false, "verdict": "INSUFFICIENT EVIDENCE"},
		})
		return
	}
	assembly := h.buildUnifiedInvestigationReport(r.Context(), target, network, "manual_detail")
	assembly.Report["target_classification"] = classification
	writeJSON(w, http.StatusOK, assembly.Report)
}
