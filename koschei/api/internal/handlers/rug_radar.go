package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type rugRadarLaunch struct {
	ID          string    `json:"id"`
	MintAddress string    `json:"mint_address"`
	Network     string    `json:"network"`
	RiskScore   int       `json:"risk_score"`
	RiskLevel   string    `json:"risk_level"`
	RiskSummary string    `json:"risk_summary"`
	IsRenounced bool      `json:"is_renounced"`
	IsFrozen    bool      `json:"is_frozen"`
	TxCount     int       `json:"tx_count"`
	SubmittedBy string    `json:"submitted_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type rugRadarSubmitRequest struct {
	MintAddress string `json:"mint_address"`
	Network     string `json:"network"`
	RiskScore   int    `json:"risk_score"`
	RiskLevel   string `json:"risk_level"`
	RiskSummary string `json:"risk_summary"`
	IsRenounced bool   `json:"is_renounced"`
	IsFrozen    bool   `json:"is_frozen"`
	TxCount     int    `json:"tx_count"`
}

func (h *Handler) RugRadarFeed(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id::text, mint_address, network, risk_score, risk_level, risk_summary,
		       is_renounced, is_frozen, tx_count, submitted_by, created_at
		FROM token_launches
		ORDER BY created_at DESC
		LIMIT 100`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "feed_unavailable"})
		return
	}
	defer rows.Close()

	launches := make([]rugRadarLaunch, 0)
	for rows.Next() {
		var launch rugRadarLaunch
		if err := rows.Scan(&launch.ID, &launch.MintAddress, &launch.Network, &launch.RiskScore, &launch.RiskLevel, &launch.RiskSummary, &launch.IsRenounced, &launch.IsFrozen, &launch.TxCount, &launch.SubmittedBy, &launch.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "feed_unavailable"})
			return
		}
		launches = append(launches, launch)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "feed_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"launches": launches, "count": len(launches)})
}

func (h *Handler) RugRadarSubmit(w http.ResponseWriter, r *http.Request) {
	var req rugRadarSubmitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	req.MintAddress = strings.TrimSpace(req.MintAddress)
	if req.MintAddress == "" || len(req.MintAddress) > 64 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid mint_address required"})
		return
	}
	req.Network = strings.TrimSpace(req.Network)
	if req.Network == "" {
		req.Network = "solana-mainnet"
	}
	if len(req.Network) > 64 || req.RiskScore < 0 || req.RiskScore > 100 || req.TxCount < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid launch data"})
		return
	}
	req.RiskLevel = strings.ToUpper(strings.TrimSpace(req.RiskLevel))
	if req.RiskLevel == "" {
		req.RiskLevel = "UNKNOWN"
	}
	if len(req.RiskLevel) > 32 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid risk_level"})
		return
	}
	req.RiskSummary = strings.TrimSpace(req.RiskSummary)
	if len(req.RiskSummary) > 1000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "risk_summary too long"})
		return
	}

	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	submittedBy := normalizedClaimEmail(claims)
	if submittedBy == "" {
		submittedBy = currentUserID(claims)
	}

	var launch rugRadarLaunch
	err := h.DB.QueryRowContext(r.Context(), `
		INSERT INTO token_launches
			(mint_address, network, risk_score, risk_level, risk_summary, is_renounced, is_frozen, tx_count, submitted_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (mint_address) DO NOTHING
		RETURNING id::text, mint_address, network, risk_score, risk_level, risk_summary,
		          is_renounced, is_frozen, tx_count, submitted_by, created_at`,
		req.MintAddress, req.Network, req.RiskScore, req.RiskLevel, req.RiskSummary, req.IsRenounced, req.IsFrozen, req.TxCount, submittedBy,
	).Scan(&launch.ID, &launch.MintAddress, &launch.Network, &launch.RiskScore, &launch.RiskLevel, &launch.RiskSummary, &launch.IsRenounced, &launch.IsFrozen, &launch.TxCount, &launch.SubmittedBy, &launch.CreatedAt)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "mint_already_submitted"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "submission_failed"})
		return
	}
	writeJSON(w, http.StatusCreated, launch)
}
