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
	claimEmail := normalizedClaimEmail(claims)
	if _, err := h.requirePremiumOutput(claims.Sub, claimEmail); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}

	var input unifiedAnalyzeHTTPInput
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid request body", nil)
		return
	}
	originalInput := strings.TrimSpace(input.Input)
	rawInput := strings.TrimSpace(firstNonEmptyString(input.TargetID, input.Target, extractUnifiedTarget(originalInput), originalInput))
	if rawInput == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Input is required", nil)
		return
	}
	if input.Network == "" {
		input.Network = "solana-mainnet"
	}
	inputType := detectUnifiedInputType(rawInput, input.TargetType, input.Context)
	inputType = h.resolveUnifiedInputType(r.Context(), rawInput, input.Network, inputType)
	if inputType == "unknown" || inputType == "" || inputType == "question" {
		inputType = "token"
	}

	radars := services.AnalyzeSecurityRadars(services.SecurityRadarRequest{
		Target:  rawInput,
		Network: input.Network,
		Mode:    "polling",
	})
	final := services.FinalSecurityRadarVerdict(radars)
	sections := map[string]any{
		"pump_sybil_radar":        radars.PumpSybilRadar,
		"raydium_pool_guardian":   radars.RaydiumPoolGuardian,
		"walletless_claim_shield": radars.WalletlessClaimShield,
		"final_verdict": map[string]any{
			"grade":          final.Grade,
			"risk_index":     final.RiskIndex,
			"risk_level":     final.RiskLevel,
			"recommendation": final.Recommendation,
			"rule_version":   final.RuleVersion,
			"signed":         final.Signed,
		},
	}
	partialFailures := []partialFailure{}
	_ = h.saveSecurityRadarBundle(r.Context(), claims.Sub, "unified_analyze", radars)
	if err := h.consumePremiumOutput(claims.Sub, claimEmail, "unified_analyze"); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}

	data := unifiedAnalyzeData{
		InputType:       inputType,
		Summary:         radars.CustomerSummary,
		Sections:        sections,
		SecurityRadars:  radars,
		Sources:         []string{"koschei_security_rules", "alchemy_solana_rpc"},
		PartialFailures: partialFailures,
	}
	_ = h.saveUnifiedArvisReport(r.Context(), claims.Sub, inputType, rawInput, final, data)
	h.logUnifiedAnalysis(claimEmail, inputType, APICodeOK, "deterministic_signed", partialFailures)
	h.logTool(claimEmail, "unified_analyze", "completed")
	h.trackEvent(claimEmail, "unified_analyze", r.URL.Path)
	writeAPISuccess(w, "Analysis completed", data)
}

func (h *Handler) UnifiedReportsHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok { writeAPIError(w, http.StatusUnauthorized, APICodeUnauthorized, "Unauthorized", nil); return }
	if h == nil || h.DBRead == nil { writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reports": []any{}}); return }
	rows, err := h.DBRead.QueryContext(r.Context(), `
		SELECT request_id, target_type, target_id, overall_score, risk_level, module_results, created_at
		FROM unified_reports
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 50`, claims.Sub)
	if isMissingRelation(err) { writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reports": []any{}, "schema_pending": true}); return }
	if err != nil { writeAPIError(w, http.StatusInternalServerError, APICodeIntegrationError, "Report vault unavailable", nil); return }
	defer rows.Close()
	reports := []map[string]any{}
	for rows.Next() {
		var id, targetType, targetID, riskLevel string
		var score int
		var raw json.RawMessage
		var createdAt time.Time
		if err := rows.Scan(&id, &targetType, &targetID, &score, &riskLevel, &raw, &createdAt); err != nil { continue }
		reports = append(reports, map[string]any{"request_id": id, "target_type": targetType, "target_id": targetID, "target": targetID, "overall_score": score, "score": score, "risk_level": riskLevel, "created_at": createdAt.UTC().Format(time.RFC3339), "module_results": jsonRaw(raw)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reports": reports})
}

func (h *Handler) saveUnifiedArvisReport(ctx context.Context, userID, targetType, targetID string, final services.SecurityRadarFinalVerdict, data unifiedAnalyzeData) error {
	if h == nil || h.DB == nil { return nil }
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(targetID) == "" { return nil }
	riskLevel := strings.ToUpper(strings.TrimSpace(final.RiskLevel))
	switch riskLevel { case "LOW", "MEDIUM", "HIGH", "CRITICAL": default: riskLevel = "UNKNOWN" }
	if targetType == "" { targetType = "token" }
	payload, err := json.Marshal(map[string]any{"surface": "KOSCHEİ WEB3 Arvıs", "target": targetID, "target_type": targetType, "final_verdict": final, "data": data})
	if err != nil { return err }
	_, err = h.DB.ExecContext(ctx, `
		INSERT INTO unified_reports (request_id, user_id, target_type, target_id, overall_score, risk_level, module_results)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
		ON CONFLICT (request_id) DO NOTHING`, newArvisReportID(), userID, targetType, targetID, final.RiskIndex, riskLevel, string(payload))
	if isMissingRelation(err) { return nil }
	return err
}

func newArvisReportID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err == nil { return "arvis_" + hex.EncodeToString(b[:]) }
	return "arvis_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
}

func extractUnifiedTarget(input string) string {
	for _, part := range strings.Fields(strings.TrimSpace(input)) {
		candidate := strings.Trim(part, " \t\r\n.,;:!?()[]{}<>\"'`“”‘’")
		if candidate == "" {
			continue
		}
		lower := strings.ToLower(candidate)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			return candidate
		}
		if decoded, ok := base58Decode(candidate); ok {
			if len(decoded) == 32 || len(decoded) == 64 || len(candidate) >= 32 {
				return candidate
			}
		}
	}
	return ""
}

func detectUnifiedInputType(input, explicit string, ctx map[string]any) string {
	hint := strings.ToLower(strings.TrimSpace(explicit))
	if hint == "" && ctx != nil {
		for _, key := range []string{"input_type", "target_type", "type"} {
			if v, ok := ctx[key].(string); ok {
				hint = strings.ToLower(strings.TrimSpace(v))
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
		if u, err := url.Parse(trimmed); err == nil && u.Host != "" {
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
	if err := h.SolanaRPC.Call(probeCtx, network, "getTokenSupply", []any{input}, &supply, 30*time.Second); err == nil && strings.TrimSpace(supply.Value.Amount) != "" {
		return "token"
	}
	var account struct{ Value any `json:"value"` }
	probeCtx, cancel = context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.SolanaRPC.Call(probeCtx, network, "getAccountInfo", []any{input, map[string]string{"encoding": "jsonParsed"}}, &account, 30*time.Second); err == nil && account.Value != nil {
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
	payload, _ := json.Marshal(map[string]any{"input_type": inputType, "provider": provider, "partial_failures": failures, "surface": "KOSCHEİ WEB3 Arvıs"})
	_, _ = h.DB.Exec(`INSERT INTO tool_usage_logs(email,tool_key,status) VALUES(NULLIF($1,''),$2,$3)`, email, "unified_analyze", status)
	_, _ = h.DB.Exec(`INSERT INTO model_route_logs(email,tool,route,model,provider,prompt,status) VALUES(NULLIF($1,''),$2,$3,$4,$5,$6,$7)`, email, "unified_analyze", inputType, "", provider, string(payload), status)
}
