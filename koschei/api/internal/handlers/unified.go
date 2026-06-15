package handlers

import (
	"context"
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
	InputType       string         `json:"input_type"`
	Summary         string         `json:"summary"`
	Sections        map[string]any `json:"sections"`
	Sources         []string       `json:"sources"`
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
	if _, err := h.requirePremiumOutput(claims.Sub); err != nil {
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

	bundle := services.AnalyzeSecurityRadars(services.SecurityRadarRequest{Target: rawInput, Network: input.Network, Mode: "unified_analyze"})
	final := services.FinalSecurityRadarVerdict(bundle)
	sections := map[string]any{
		"pump_sybil_radar":       bundle.PumpSybilRadar,
		"raydium_pool_guardian":  bundle.RaydiumPoolGuardian,
		"walletless_claim_shield": bundle.WalletlessClaimShield,
		"final_verdict": map[string]any{
			"grade":          final.Grade,
			"risk_index":     final.RiskIndex,
			"risk_level":     final.RiskLevel,
			"verdict":        final.Verdict,
			"recommendation": final.Recommendation,
			"rule_version":   final.RuleVersion,
			"signed":         final.Signed,
		},
	}
	partialFailures := []partialFailure{}
	_ = h.saveSecurityRadarBundle(r.Context(), claims.Sub, "unified_analyze", bundle)
	if err := h.consumePremiumOutput(claims.Sub, normalizedClaimEmail(claims), "unified_analyze"); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}

	data := unifiedAnalyzeData{InputType: inputType, Summary: bundle.CustomerSummary, Sections: sections, Sources: []string{"koschei_security_rules", "alchemy_solana_rpc"}, PartialFailures: partialFailures}
	h.logUnifiedAnalysis(normalizedClaimEmail(claims), inputType, APICodeOK, "deterministic_signed", partialFailures)
	h.logTool(normalizedClaimEmail(claims), "unified_analyze", "completed")
	h.trackEvent(normalizedClaimEmail(claims), "unified_analyze", r.URL.Path)
	writeAPISuccess(w, "Analysis completed", data)
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
	payload, _ := json.Marshal(map[string]any{"input_type": inputType, "provider": provider, "partial_failures": failures, "surface": "Koschei Web3 Hub Security Radar"})
	_, _ = h.DB.Exec(`INSERT INTO tool_usage_logs(email,tool_key,status) VALUES(NULLIF($1,''),$2,$3)`, email, "unified_analyze", status)
	_, _ = h.DB.Exec(`INSERT INTO model_route_logs(email,tool,route,model,provider,prompt,status) VALUES(NULLIF($1,''),$2,$3,$4,$5,$6,$7)`, email, "unified_analyze", inputType, "", provider, string(payload), status)
}
