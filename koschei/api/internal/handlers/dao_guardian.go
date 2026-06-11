package handlers

import (
	"net/http"
	"strings"
)

type daoProposalRiskRequest struct {
	DAOID               string   `json:"dao_id"`
	TreasuryAddress     string   `json:"treasury_address"`
	ProposalID          string   `json:"proposal_id"`
	Instructions        []string `json:"instructions"`
	EstimatedOutflowUSD float64  `json:"estimated_outflow_usd"`
	SignerCount         int      `json:"signer_count"`
	RequiredSigners     int      `json:"required_signers"`
}

// DAOGuardianAnalyze simulates DAO/multisig proposal outflow risk and creates
// the backend contract needed for the Turkish Owner Panel DAO Guardian tab.
func (h *Handler) DAOGuardianAnalyze(w http.ResponseWriter, r *http.Request) {
	var req daoProposalRiskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if strings.TrimSpace(req.TreasuryAddress) == "" || strings.TrimSpace(req.ProposalID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "treasury_and_proposal_required"})
		return
	}
	score := daoProposalRiskScore(req)
	level := "DÜŞÜK"
	if score >= 70 {
		level = "KRİTİK"
	} else if score >= 40 {
		level = "ORTA"
	}
	if h.DB != nil {
		_, _ = h.DB.ExecContext(r.Context(), `INSERT INTO proposal_risks (dao_id, treasury_address, proposal_id, risk_score, risk_level, estimated_outflow_usd, instruction_count, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,now())`, req.DAOID, req.TreasuryAddress, req.ProposalID, score, level, req.EstimatedOutflowUSD, len(req.Instructions))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "risk_score": score, "risk_level": level, "treasury_at_risk_usd": req.EstimatedOutflowUSD, "proposal_loss_prevented_usd": req.EstimatedOutflowUSD, "message": "DAO Guardian teklif simülasyonu tamamlandı."})
}

// OwnerDAOGuardianSummary feeds the dedicated Turkish DAO Guardian tab in the
// owner control center with aggregate enterprise KPIs.
func (h *Handler) OwnerDAOGuardianSummary(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "title": "DAO Guardian", "treasury_at_risk_usd": 0, "proposal_loss_prevented_usd": 0, "pending_reviews": 0, "message": "DAO hazine koruma paneli hazır; canlı veriler migration sonrası akacak."})
}

func daoProposalRiskScore(req daoProposalRiskRequest) int {
	score := 15
	if req.EstimatedOutflowUSD >= 100_000 {
		score += 30
	}
	if len(req.Instructions) >= 5 {
		score += 15
	}
	if req.RequiredSigners > 0 && req.SignerCount > 0 && req.RequiredSigners*2 <= req.SignerCount {
		score += 20
	}
	for _, ix := range req.Instructions {
		low := strings.ToLower(ix)
		if strings.Contains(low, "transfer") || strings.Contains(low, "withdraw") || strings.Contains(low, "set_authority") {
			score += 10
		}
	}
	if score > 100 {
		return 100
	}
	return score
}
