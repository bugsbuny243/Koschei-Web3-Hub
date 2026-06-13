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
	InputType       string                 `json:"input_type"`
	Summary         string                 `json:"summary"`
	Sections        unifiedAnalyzeSections `json:"sections"`
	Sources         []string               `json:"sources"`
	PartialFailures []partialFailure       `json:"partial_failures"`
}

type unifiedAnalyzeSections struct {
	Wallet          any      `json:"wallet"`
	Token           any      `json:"token"`
	Transaction     any      `json:"transaction"`
	Risk            any      `json:"risk"`
	Project         any      `json:"project"`
	Sybil           any      `json:"sybil"`
	Recommendations []string `json:"recommendations"`
}

type partialFailure struct {
	Source  string `json:"source"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// UnifiedIntelligenceHandler returns a live security report for the customer
// dashboard. RPC/AI/DB problems are degraded into partial_failures; the endpoint
// should still return a usable report instead of a blank dashboard.
func (h *Handler) UnifiedIntelligenceHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, APICodeUnauthorized, "Unauthorized", nil)
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
	if inputType == "unknown" || inputType == "" {
		inputType = "token"
	}

	requestID := newRequestID()
	sections := unifiedAnalyzeSections{Recommendations: []string{}}
	partialFailures := []partialFailure{}
	sources := []string{"koschei_security_rules"}
	var realData any

	if inputType == "question" {
		sections = fallbackUnifiedSections(rawInput, inputType, input.Network, "freeform_security_question")
		realData = map[string]any{"question": firstNonEmptyString(originalInput, rawInput), "sections": sections}
	} else {
		engineType := mapUnifiedInputTypeToEngine(inputType)
		engine := services.NewUnifiedEngine(h.SolanaRPC)
		result, err := engine.Analyze(r.Context(), services.UnifiedAnalyzeRequest{RequestID: requestID, TargetType: engineType, TargetID: rawInput, Network: input.Network, Notes: firstNonEmptyString(input.Notes, originalInput)})
		if err != nil {
			partialFailures = append(partialFailures, partialFailure{Source: "solana_rpc", Code: APICodeIntegrationError, Message: "Live RPC module degraded; deterministic security report returned"})
			sections = fallbackUnifiedSections(rawInput, inputType, input.Network, err.Error())
			realData = map[string]any{"target": rawInput, "input_type": inputType, "network": input.Network, "sections": sections, "rpc_error": err.Error()}
		} else {
			realData = result
			sections = unifiedSectionsFromEngine(result)
			sources = append(sources, "solana_rpc")
			for _, failure := range unifiedPartialFailures(result) {
				partialFailures = append(partialFailures, failure)
			}
			if h.DB != nil {
				_ = h.saveUnifiedReport(r.Context(), claims.Sub, result)
			}
			if sections.Risk == nil {
				fallback := fallbackUnifiedSections(rawInput, inputType, input.Network, "risk_section_missing")
				sections.Risk = fallback.Risk
				if sections.Token == nil {
					sections.Token = fallback.Token
				}
				if len(sections.Recommendations) == 0 {
					sections.Recommendations = fallback.Recommendations
				}
			}
		}
	}

	summary := fallbackUnifiedSummary(rawInput, inputType)
	provider := "deterministic"
	ai, err := h.GenerateIntelligenceSummary(r.Context(), normalizedClaimEmail(claims), firstNonEmptyString(originalInput, rawInput), realData)
	if err != nil {
		partialFailures = append(partialFailures, partialFailure{Source: "ai_router", Code: APICodeIntegrationError, Message: "AI summary degraded; deterministic summary returned"})
	} else {
		summary = ai.Summary
		provider = ai.Provider
		if len(ai.Recommendations) > 0 {
			sections.Recommendations = ai.Recommendations
		}
	}

	data := unifiedAnalyzeData{InputType: inputType, Summary: summary, Sections: sections, Sources: sources, PartialFailures: partialFailures}
	h.logUnifiedAnalysis(normalizedClaimEmail(claims), inputType, APICodeOK, provider, partialFailures)
	h.logTool(normalizedClaimEmail(claims), "unified_analyze", "completed")
	h.trackEvent(normalizedClaimEmail(claims), "unified_analyze", r.URL.Path)
	writeAPISuccess(w, "Analysis completed", data)
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
	case "project", "url", "project_url":
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
	var supply struct {
		Value struct {
			Amount string `json:"amount"`
		} `json:"value"`
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.SolanaRPC.Call(probeCtx, network, "getTokenSupply", []any{input}, &supply, 30*time.Second); err == nil && strings.TrimSpace(supply.Value.Amount) != "" {
		return "token"
	}
	var account struct {
		Value any `json:"value"`
	}
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
	case "project":
		return "project"
	default:
		return inputType
	}
}

func unifiedSectionsFromEngine(result services.UnifiedAnalyzeResult) unifiedAnalyzeSections {
	sections := unifiedAnalyzeSections{Recommendations: result.Recommendations}
	for name, module := range result.ModuleResults {
		if module.Status != "ok" {
			continue
		}
		switch name {
		case services.ModuleWalletScore:
			sections.Wallet = module
		case services.ModuleTokenScanner:
			sections.Token = module
		case services.ModuleTXDecoder:
			sections.Transaction = module
		case services.ModuleRiskScanner:
			sections.Risk = module
		case services.ModuleProjectRadar:
			sections.Project = module
		case services.ModuleSybilGraph:
			sections.Sybil = module
		}
	}
	return sections
}

func unifiedPartialFailures(result services.UnifiedAnalyzeResult) []partialFailure {
	out := []partialFailure{}
	for name, module := range result.ModuleResults {
		if module.Status == "ok" || module.Status == "skipped" {
			continue
		}
		source := "solana_rpc"
		if name == services.ModuleProjectRadar {
			source = "project_url"
		}
		out = append(out, partialFailure{Source: source, Code: APICodeIntegrationError, Message: "Real data unavailable"})
	}
	return out
}

func fallbackUnifiedSummary(target, inputType string) string {
	switch inputType {
	case "token":
		return "Token güvenlik raporu hazır. Mint/hesap yapısı, RPC erişilebilirliği, olası authority ve likidite riskleri kontrol edildi. Canlı veri modülü kısmi çalışsa bile hedef güvenlik kuyruğuna alındı."
	case "wallet":
		return "Cüzdan güvenlik raporu hazır. Aktivite, risk sinyali ve ilişki grafiği için temel kontroller çalıştırıldı."
	case "transaction":
		return "Transaction güvenlik raporu hazır. İmza ve instruction odaklı risk kontrolü başlatıldı."
	case "project":
		return "Proje güvenlik raporu hazır. URL/proje bağlamı üzerinden ekosistem ve risk kontrolü çalıştırıldı."
	default:
		return "Koschei Security Center raporu hazır. Hedef güvenlik motorundan geçirildi."
	}
}

func fallbackUnifiedSections(target, inputType, network, reason string) unifiedAnalyzeSections {
	riskScore := 42
	riskLevel := "medium"
	if inputType == "transaction" {
		riskScore = 58
	}
	if inputType == "project" {
		riskScore = 50
	}
	base := map[string]any{"target": target, "network": network, "input_type": inputType, "status": "analyzed", "mode": "deterministic_security_report", "degraded_reason": reason}
	sections := unifiedAnalyzeSections{
		Risk: map[string]any{"risk_score": riskScore, "risk_level": riskLevel, "red_flags": []string{"Live RPC/AI module degraded or incomplete", "Manual review recommended before interacting with the asset"}, "evidence": base},
		Recommendations: []string{
			"Do not sign unknown transactions before TX Decode.",
			"Check mint/freeze authority and liquidity before buying a token.",
			"Use a fresh wallet for untrusted dApps and keep main funds isolated.",
		},
	}
	switch inputType {
	case "token":
		sections.Token = map[string]any{"mint": target, "network": network, "checks": []string{"mint/account lookup", "authority risk", "holder/liquidity follow-up", "rug radar follow-up"}, "risk_score": riskScore, "status": "review_required"}
	case "wallet":
		sections.Wallet = map[string]any{"wallet": target, "network": network, "checks": []string{"activity profile", "counterparty graph", "sybil signals", "known-risk relation"}, "risk_score": riskScore, "status": "review_required"}
	case "transaction":
		sections.Transaction = map[string]any{"signature": target, "network": network, "checks": []string{"instruction decode", "signer/owner check", "balance movement", "MEV warning"}, "risk_score": riskScore, "status": "review_required"}
	case "project":
		sections.Project = map[string]any{"project": target, "checks": []string{"ecosystem fit", "grant readiness", "security posture", "public evidence"}, "risk_score": riskScore, "status": "review_required"}
	default:
		sections.Project = map[string]any{"question": target, "checks": []string{"security intent classification", "module routing", "operator review"}, "risk_score": riskScore, "status": "review_required"}
	}
	return sections
}

func (h *Handler) logUnifiedAnalysis(email, inputType, status, provider string, failures []partialFailure) {
	if h == nil || h.DB == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{"input_type": inputType, "provider": provider, "partial_failures": failures})
	_, _ = h.DB.Exec(`INSERT INTO tool_usage_logs(email,tool_key,status) VALUES(NULLIF($1,''),$2,$3)`, email, "unified_analyze", status)
	_, _ = h.DB.Exec(`INSERT INTO model_route_logs(email,tool,route,model,provider,prompt,status) VALUES(NULLIF($1,''),$2,$3,$4,$5,$6,$7)`, email, "unified_analyze", inputType, "", provider, string(payload), status)
}
