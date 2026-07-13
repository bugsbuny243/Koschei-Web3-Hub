package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type publicScanVerdict struct {
	Grade          string    `json:"grade"`
	RiskIndex      int       `json:"risk_index"`
	RiskLevel      string    `json:"risk_level"`
	Verdict        string    `json:"verdict"`
	Recommendation string    `json:"recommendation"`
	Evidence       []string  `json:"evidence"`
	RuleVersion    string    `json:"rule_version"`
	Signature      string    `json:"signature"`
	CreatedAt      time.Time `json:"created_at"`
}

// PublicScanHistory exposes only signed, customer-visible final verdicts for a
// Solana mint. It never exposes private owner signals or unsigned previews.
func (h *Handler) PublicScanHistory(w http.ResponseWriter, r *http.Request) {
	mint := strings.TrimSpace(firstNonEmptyString(r.URL.Query().Get("mint"), r.URL.Query().Get("target")))
	if mint == "" || !isValidSolanaAddress(mint) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid_mint_required"})
		return
	}
	limit := 12
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 30 {
			limit = parsed
		}
	}
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mint": mint, "items": []publicScanVerdict{}, "count": 0})
		return
	}
	rows, err := db.QueryContext(r.Context(), `
		SELECT grade,risk_index,risk_level,verdict,recommendation,evidence,rule_version,
		       COALESCE(signature,''),created_at
		FROM security_radar_verdicts
		WHERE lower(target)=lower($1)
		  AND module_id='final_verdict_engine'
		  AND signed=true
		  AND signature IS NOT NULL
		  AND btrim(signature)<>''
		  AND COALESCE(signals->>'customer_detail_visible','true')<>'false'
		  AND (
			COALESCE(signals->>'verified_evidence','false')='true'
			OR COALESCE(signals->>'real_onchain_evidence','false')='true'
			OR COALESCE(signals->>'real_offchain_evidence','false')='true'
		  )
		ORDER BY created_at DESC,id DESC
		LIMIT $2`, mint, limit)
	if err != nil {
		if err == sql.ErrNoRows || isMissingRelation(err) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mint": mint, "items": []publicScanVerdict{}, "count": 0})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "scan_history_unavailable"})
		return
	}
	defer rows.Close()
	items := make([]publicScanVerdict, 0, limit)
	for rows.Next() {
		var item publicScanVerdict
		var evidenceRaw []byte
		if err := rows.Scan(&item.Grade, &item.RiskIndex, &item.RiskLevel, &item.Verdict, &item.Recommendation, &evidenceRaw, &item.RuleVersion, &item.Signature, &item.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "scan_history_unavailable"})
			return
		}
		_ = json.Unmarshal(evidenceRaw, &item.Evidence)
		if item.Evidence == nil {
			item.Evidence = []string{}
		}
		items = append(items, item)
	}
	response := map[string]any{"ok": true, "mint": mint, "items": items, "count": len(items)}
	if len(items) > 0 {
		response["latest"] = items[0]
	}
	if len(items) > 1 {
		response["previous"] = items[1]
		response["risk_change"] = items[0].RiskIndex - items[1].RiskIndex
	}
	writeJSON(w, http.StatusOK, response)
}
