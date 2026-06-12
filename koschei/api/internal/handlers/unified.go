package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type unifiedAnalyzeHTTPInput struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Network    string `json:"network"`
	Notes      string `json:"notes"`
}

type unifiedAnalyzeHTTPResponse struct {
	OK              bool                          `json:"ok"`
	ReportSaved     bool                          `json:"report_saved"`
	ReportSaveError string                        `json:"report_save_error,omitempty"`
	Result          services.UnifiedAnalyzeResult `json:"result"`
}

// UnifiedIntelligenceHandler runs the six enterprise intelligence modules in
// parallel and aggregates them into one persisted report. It intentionally uses
// the existing RequireAuth middleware and does not alter auth/login/session flow.
func (h *Handler) UnifiedIntelligenceHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var input unifiedAnalyzeHTTPInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	input.TargetType = strings.TrimSpace(input.TargetType)
	input.TargetID = strings.TrimSpace(input.TargetID)
	input.Network = strings.TrimSpace(input.Network)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.TargetType == "" || input.TargetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target_type_and_target_id_required"})
		return
	}
	if input.Network == "" {
		input.Network = "solana-mainnet"
	}

	requestID := newRequestID()
	engine := services.NewUnifiedEngine(h.SolanaRPC)
	result, err := engine.Analyze(r.Context(), services.UnifiedAnalyzeRequest{
		RequestID:  requestID,
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
		Network:    input.Network,
		Notes:      input.Notes,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	saved := false
	saveErr := ""
	if h.DB != nil {
		if err := h.saveUnifiedReport(r.Context(), claims.Sub, result); err != nil {
			saveErr = err.Error()
		} else {
			saved = true
		}
	}

	h.logTool(normalizedClaimEmail(claims), "unified_intelligence", "completed")
	h.trackEvent(normalizedClaimEmail(claims), "unified_intelligence_run", r.URL.Path)
	writeJSON(w, http.StatusOK, unifiedAnalyzeHTTPResponse{OK: true, ReportSaved: saved, ReportSaveError: saveErr, Result: result})
}

func (h *Handler) saveUnifiedReport(ctx context.Context, userID string, result services.UnifiedAnalyzeResult) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	moduleResults, err := json.Marshal(result.ModuleResults)
	if err != nil {
		return err
	}
	_, err = h.DB.ExecContext(ctx, `
		INSERT INTO unified_reports (request_id, user_id, target_type, target_id, overall_score, risk_level, module_results, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, now())`,
		result.RequestID, nullIfEmpty(userID), result.TargetType, result.TargetID, result.OverallScore, result.RiskLevel, string(moduleResults))
	return err
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}
