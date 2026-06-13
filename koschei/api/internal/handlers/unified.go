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
	Success         bool                          `json:"success"`
	OK              bool                          `json:"ok"`
	Code            string                        `json:"code,omitempty"`
	Message         string                        `json:"message,omitempty"`
	ReportSaved     bool                          `json:"report_saved"`
	ReportSaveError string                        `json:"report_save_error,omitempty"`
	Result          services.UnifiedAnalyzeResult `json:"result,omitempty"`
}

type unifiedReportListItem struct {
	RequestID     string          `json:"request_id"`
	TargetType    string          `json:"target_type"`
	TargetID      string          `json:"target_id"`
	OverallScore  int             `json:"overall_score"`
	RiskLevel     string          `json:"risk_level"`
	ModuleResults json.RawMessage `json:"module_results,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// UnifiedIntelligenceHandler runs the six enterprise intelligence modules in
// parallel and aggregates them into one persisted report. It intentionally uses
// the existing RequireAuth middleware and does not alter auth/login/session flow.
func (h *Handler) UnifiedIntelligenceHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeUnifiedError(w, http.StatusUnauthorized, "INVALID_INPUT", "Authentication required")
		return
	}

	var input unifiedAnalyzeHTTPInput
	if err := decodeJSON(r, &input); err != nil {
		writeUnifiedError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON request body")
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
		writeUnifiedError(w, http.StatusBadRequest, "INVALID_INPUT", "Wallet address, token mint, transaction hash, project URL, or project name is required.")
		return
	}
	if input.Network == "" {
		input.Network = "solana-mainnet"
	}
	if !isSupportedUnifiedTargetType(input.TargetType) {
		writeUnifiedError(w, http.StatusBadRequest, "INVALID_CATEGORY", "Unsupported analysis category")
		return
	}

	activePackage, err := h.hasActivePaidPackage(claims.Sub, normalizedClaimEmail(claims))
	if err != nil {
		writeUnifiedError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Package access could not be verified")
		return
	}
	if !activePackage {
		writeUnifiedError(w, http.StatusPaymentRequired, "PACKAGE_REQUIRED", "Active Koschei package required")
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
		writeUnifiedError(w, http.StatusBadRequest, "INVALID_INPUT", "Analysis input could not be processed")
		return
	}

	if !unifiedHasCompletedModule(result) {
		writeUnifiedError(w, http.StatusBadGateway, "INTEGRATION_ERROR", "Real data unavailable")
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
	writeJSON(w, http.StatusOK, unifiedAnalyzeHTTPResponse{Success: true, OK: true, ReportSaved: saved, ReportSaveError: saveErr, Result: result})
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

// UnifiedReportsHandler returns only the authenticated customer's persisted unified reports.
func (h *Handler) UnifiedReportsHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"success": false, "code": "INVALID_INPUT", "message": "Authentication required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	rows, err := h.DB.QueryContext(ctx, `
		SELECT request_id, target_type, target_id, overall_score, risk_level, module_results, created_at
		FROM unified_reports
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 25`, claims.Sub)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "reports": []unifiedReportListItem{}})
		return
	}
	defer rows.Close()
	reports := []unifiedReportListItem{}
	for rows.Next() {
		var item unifiedReportListItem
		if err := rows.Scan(&item.RequestID, &item.TargetType, &item.TargetID, &item.OverallScore, &item.RiskLevel, &item.ModuleResults, &item.CreatedAt); err != nil {
			continue
		}
		reports = append(reports, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "reports": reports})
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

func writeUnifiedError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"success": false, "code": code, "message": message})
}

func isSupportedUnifiedTargetType(targetType string) bool {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "wallet", "address", "token", "mint", "tx", "transaction", "project", "url":
		return true
	default:
		return false
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
