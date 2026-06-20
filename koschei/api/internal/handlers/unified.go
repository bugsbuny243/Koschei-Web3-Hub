package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type unifiedAnalyzeHTTPInput struct {
	Input      string         `json:"input"`
	Context    map[string]any `json:"context"`
	Target     string         `json:"target"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Network    string         `json:"network"`
	Notes      string         `json:"notes"`
}

type unifiedAnalyzeData struct {
	InputType       string           `json:"input_type"`
	Summary         string           `json:"summary"`
	Sections        map[string]any   `json:"sections"`
	SecurityRadars  any              `json:"security_radars,omitempty"`
	Sources         []string         `json:"sources"`
	PartialFailures []partialFailure `json:"partial_failures"`
}

type partialFailure struct {
	Source  string `json:"source"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *Handler) UnifiedIntelligenceHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, APICodeUnauthorized, "Unauthorized", nil)
		return
	}
	email := normalizedClaimEmail(claims)
	if _, err := h.requirePremiumOutput(claims.Sub, email); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	var in unifiedAnalyzeHTTPInput
	if err := decodeJSON(r, &in); err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid request body", nil)
		return
	}
	original := strings.TrimSpace(in.Input)
	target := strings.TrimSpace(firstNonEmptyString(in.TargetID, in.Target, extractUnifiedTarget(original), original))
	if target == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Input is required", nil)
		return
	}
	if in.Network == "" {
		in.Network = "solana-mainnet"
	}
	inputType := h.resolveUnifiedInputType(r.Context(), target, in.Network, detectUnifiedInputType(target, in.TargetType, in.Context))
	if inputType == "" || inputType == "unknown" || inputType == "question" {
		inputType = "token"
	}

	analysis := services.AnalyzeArvisRadars(services.SecurityRadarRequest{Target: target, Network: in.Network, Mode: "polling"})
	radars := services.EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	arms := services.ArvisArmsFromBundle(radars)
	final := services.ArvisFinalFromBundle(radars)
	sections := arvisSections(arms, final)
	if !services.SecurityRadarHasLiveEvidence(radars) || !final.Signed {
		h.logTool(email, "unified_analyze", "real_data_unavailable")
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "real_data_unavailable", "message": services.SecurityRadarInsufficientEvidenceMessage, "charged": false, "sections": sections})
		return
	}

	failures := []partialFailure{}
	_ = h.saveSecurityRadarBundle(r.Context(), claims.Sub, "unified_analyze", radars)
	if err := h.consumePremiumOutput(claims.Sub, email, "unified_analyze"); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	data := unifiedAnalyzeData{InputType: inputType, Summary: radars.CustomerSummary, Sections: sections, SecurityRadars: radars, Sources: []string{"koschei_security_rules", "solana_rpc"}, PartialFailures: failures}
	_ = h.saveUnifiedArvisReport(r.Context(), claims.Sub, inputType, target, final, data)
	h.logUnifiedAnalysis(email, inputType, APICodeOK, "evidence_backed_arvis", failures)
	h.logTool(email, "unified_analyze", "completed")
	h.trackEvent(email, "unified_analyze", r.URL.Path)
	writeAPISuccess(w, "Analysis completed", data)
}

func (h *Handler) UnifiedReportsHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, APICodeUnauthorized, "Unauthorized", nil)
		return
	}
	if h == nil || h.DBRead == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reports": []any{}})
		return
	}
	rows, err := h.DBRead.QueryContext(r.Context(), `SELECT request_id,target_type,target_id,overall_score,risk_level,module_results,created_at FROM unified_reports WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, claims.Sub)
	if isMissingRelation(err) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reports": []any{}, "schema_pending": true})
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, APICodeIntegrationError, "Report vault unavailable", nil)
		return
	}
	defer rows.Close()
	reports := []map[string]any{}
	for rows.Next() {
		var id, targetType, targetID, level string
		var score int
		var raw json.RawMessage
		var created time.Time
		if rows.Scan(&id, &targetType, &targetID, &score, &level, &raw, &created) != nil {
			continue
		}
		reports = append(reports, map[string]any{"request_id": id, "target_type": targetType, "target_id": targetID, "target": targetID, "overall_score": score, "score": score, "risk_level": level, "created_at": created.UTC().Format(time.RFC3339), "module_results": jsonRaw(raw)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reports": reports})
}

func (h *Handler) saveUnifiedArvisReport(ctx context.Context, userID, targetType, targetID string, final services.SecurityRadarFinalVerdict, data unifiedAnalyzeData) error {
	if h == nil || h.DB == nil || strings.TrimSpace(userID) == "" || strings.TrimSpace(targetID) == "" || !final.Signed {
		return nil
	}
	level := strings.ToUpper(strings.TrimSpace(final.RiskLevel))
	switch level {
	case "LOW", "MEDIUM", "HIGH", "CRITICAL":
	default:
		level = "UNKNOWN"
	}
	if targetType == "" {
		targetType = "token"
	}
	payload, err := json.Marshal(map[string]any{"surface": "KOSCHEİ WEB3 Arvis", "target": targetID, "target_type": targetType, "final_verdict": final, "data": data})
	if err != nil {
		return err
	}
	_, err = h.DB.ExecContext(ctx, `INSERT INTO unified_reports(request_id,user_id,target_type,target_id,overall_score,risk_level,module_results) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb) ON CONFLICT(request_id) DO NOTHING`, newArvisReportID(), userID, targetType, targetID, final.RiskIndex, level, string(payload))
	if isMissingRelation(err) {
		return nil
	}
	return err
}

func newArvisReportID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "arvis_" + hex.EncodeToString(b[:])
	}
	return "arvis_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
}

func extractUnifiedTarget(input string) string {
	for _, part := range strings.Fields(strings.TrimSpace(input)) {
		candidate := strings.Trim(part, " \t\r\n.,;:!?()[]{}<>\"'`")
		if candidate == "" {
			continue
		}
		lower := strings.ToLower(candidate)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			return candidate
		}
		if decoded, ok := base58Decode(candidate); ok && (len(decoded) == 32 || len(decoded) == 64 || len(candidate) >= 32) {
			return candidate
		}
	}
	return ""
}

func detectUnifiedInputType(input, explicit string, ctx map[string]any) string {
	hint := strings.ToLower(strings.TrimSpace(explicit))
	if hint == "" && ctx != nil {
		for _, key := range []string{"input_type", "target_type", "type"} {
			if value, ok := ctx[key].(string); ok {
				hint = strings.ToLower(strings.TrimSpace(value))
				break
			}
		}
	}
	switch hint {
	case "wallet", "wallet_address":
		return "wallet"
	case "token", "mint", "token_mint":
		return "token"
	case "tx", "transaction", "signature", "hash":
		return "transaction"
	case "program", "contract", "smart_contract":
		return "program"
	case "project", "url", "project_url", "claim_url":
		return "project"
	case "question":
		return "question"
	}
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
			return "project"
		}
	}
	if decoded, ok := base58Decode(trimmed); ok {
		if len(decoded) == 64 || len(trimmed) >= 87 {
			return "transaction"
		}
		if len(decoded) == 32 {
			return "token"
		}
	}
	if strings.Contains(trimmed, ".") {
		return "project"
	}
	if strings.Contains(trimmed, " ") || strings.Contains(trimmed, "?") {
		return "question"
	}
	return "token"
}

func (h *Handler) resolveUnifiedInputType(ctx context.Context, input, network, detected string) string {
	if detected != "question" && detected != "unknown" {
		return detected
	}
	decoded, ok := base58Decode(strings.TrimSpace(input))
	if !ok || len(decoded) != 32 || h == nil || h.SolanaRPC == nil {
		return detected
	}
	var supply struct{ Value struct{ Amount string `json:"amount"` } `json:"value"` }
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if h.SolanaRPC.Call(probeCtx, network, "getTokenSupply", []any{input}, &supply, 30*time.Second) == nil && strings.TrimSpace(supply.Value.Amount) != "" {
		return "token"
	}
	var account struct{ Value any `json:"value"` }
	probeCtx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
	defer cancel2()
	if h.SolanaRPC.Call(probeCtx2, network, "getAccountInfo", []any{input, map[string]string{"encoding": "jsonParsed"}}, &account, 30*time.Second) == nil && account.Value != nil {
		return "wallet"
	}
	return detected
}

func mapUnifiedInputTypeToEngine(inputType string) string {
	switch inputType {
	case "transaction":
		return "tx"
	case "token":
		return "token"
	case "wallet":
		return "wallet"
	case "project", "program":
		return "project"
	default:
		return inputType
	}
}

func (h *Handler) logUnifiedAnalysis(email, inputType, status, provider string, failures []partialFailure) {
	if h == nil || h.DB == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{"input_type": inputType, "provider": provider, "partial_failures": failures, "surface": "KOSCHEİ WEB3 Arvis"})
	_, _ = h.DB.Exec(`INSERT INTO tool_usage_logs(email,tool_key,status) VALUES(NULLIF($1,''),$2,$3)`, email, "unified_analyze", status)
	_, _ = h.DB.Exec(`INSERT INTO model_route_logs(email,tool,route,model,provider,prompt,status) VALUES(NULLIF($1,''),$2,$3,$4,$5,$6,$7)`, email, "unified_analyze", inputType, "", provider, string(payload), status)
}
