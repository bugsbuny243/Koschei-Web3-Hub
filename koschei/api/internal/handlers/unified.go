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
	Target     string `json:"target"`
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
	input.Target = strings.TrimSpace(input.Target)
	input.TargetType = strings.TrimSpace(input.TargetType)
	input.TargetID = strings.TrimSpace(input.TargetID)
	input.Network = strings.TrimSpace(input.Network)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.TargetID == "" {
		input.TargetID = input.Target
	}
	if input.TargetType == "" {
		input.TargetType = inferUnifiedTargetType(input.TargetID)
	}
	if input.TargetType == "" || input.TargetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target_required", "message": "Wallet address, token mint, transaction hash, project URL, or project name is required."})
		return
	}
	if input.Network == "" {
		input.Network = "solana-mainnet"
	}

	activePackage, err := h.hasActivePaidPackage(claims.Sub, normalizedClaimEmail(claims))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed", "message": "Package access could not be verified."})
		return
	}
	if !activePackage {
		writeJSON(w, http.StatusPaymentRequired, unifiedAccessError("no_active_package", "An active Koschei package is required to access the A.R.V.I.S Command Center."))
		return
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

	if !unifiedHasCompletedModule(result) {
		writeJSON(w, http.StatusBadGateway, unifiedAnalyzeHTTPResponse{OK: false, Result: result})
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

func inferUnifiedTargetType(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.Contains(lower, ".") && !strings.Contains(lower, " ") {
		return "project"
	}
	if strings.HasPrefix(lower, "0x") && len(lower) == 66 {
		return "tx"
	}
	if len(target) >= 87 {
		return "tx"
	}
	if len(target) >= 32 && len(target) <= 44 && !strings.Contains(target, " ") {
		return "address"
	}
	return "project"
}

func unifiedAccessError(code, message string) map[string]any {
	return map[string]any{
		"ok":                   false,
		"error":                code,
		"message":              message,
		"entitlement_required": true,
	}
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}

func unifiedHasCompletedModule(result services.UnifiedAnalyzeResult) bool {
	for _, module := range result.ModuleResults {
		if module.Status == "ok" {
			return true
		}
	}
	return false
}
