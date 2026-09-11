package handlers

import (
	"net/http"

	"koschei/api/internal/services"
)

type defenseValidationAgentExecutionEvidenceResponse struct {
	OK              bool                                   `json:"ok"`
	Product         string                                 `json:"product"`
	ContractVersion string                                 `json:"contract_version"`
	TraceCount      int                                    `json:"trace_count"`
	Traces          []services.AgentExecutionEvidenceTrace `json:"traces"`
	Limitations     []string                               `json:"limitations"`
}

// DefenseValidationAgentExecutionEvidenceV1 projects already-validated
// defense-validation evidence into the additive agent execution trace contract.
// It does not grant authority, execute payloads, submit transactions, or infer
// identity/delegation evidence that is absent from the source corpus.
func (h *Handler) DefenseValidationAgentExecutionEvidenceV1(w http.ResponseWriter, r *http.Request) {
	var input defenseValidationAPIRequest
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "code": "invalid_request", "message": "Invalid defense validation request.",
		})
		return
	}

	trustedCollectors, err := trustedDefenseValidationCollectorsFromEnv()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "code": "defense_validation_trust_unavailable", "message": "Defense validation collector trust is unavailable.",
		})
		return
	}
	if err := validateDefenseValidationAPICollectorTrust(input.Controls, trustedCollectors); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "code": "defense_validation_evidence_rejected", "message": err.Error(),
		})
		return
	}

	validation, err := evaluateDefenseValidationAPIRequest(input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "code": "defense_validation_evidence_rejected", "message": err.Error(),
		})
		return
	}
	traces, err := buildDefenseValidationAgentExecutionTraces(input, validation.Report)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "code": "agent_execution_evidence_rejected", "message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, defenseValidationAgentExecutionEvidenceResponse{
		OK:              true,
		Product:         "Koschei Agent Execution Evidence",
		ContractVersion: services.AgentExecutionEvidenceContractVersion,
		TraceCount:      len(traces),
		Traces:          traces,
		Limitations: []string{
			"This projection is evidence-only and cannot grant execution authority, trust, ownership, identity or delegation.",
			"The source corpus is isolated fork/sandbox defense validation; no mainnet transaction or production-control mutation is claimed.",
			"A VERIFIED stage describes evidence quality for that stage; its outcome field separately records allow, block or observed execution state.",
		},
	})
}
