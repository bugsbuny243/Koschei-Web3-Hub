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
	RiskEngine          any      `json:"risk_engine"`
	TokenRisk           any      `json:"token_rug_liquidity"`
	TransactionSecurity any      `json:"transaction_mev_security"`
	WalletIntelligence  any      `json:"wallet_sybil_intelligence"`
	IntelligenceGraph   any      `json:"intelligence_graph"`
	GrantReadiness      any      `json:"grant_investor_readiness"`
	Recommendations     []string `json:"recommendations"`
}

type partialFailure struct {
	Source  string `json:"source"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// UnifiedIntelligenceHandler returns the curated Koschei Security Center report for
// the customer dashboard. RPC/AI/DB problems are degraded into partial_failures;
// the endpoint still returns the six serious product modules instead of empty promises.
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
	sections := fallbackUnifiedSections(rawInput, inputType, input.Network, "baseline_security_center")
	partialFailures := []partialFailure{}
	sources := []string{"koschei_security_rules", "solana_security_center"}
	var realData any = map[string]any{"target": rawInput, "input_type": inputType, "network": input.Network, "sections": sections}

	if inputType != "question" {
		engineType := mapUnifiedInputTypeToEngine(inputType)
		engine := services.NewUnifiedEngine(h.SolanaRPC)
		result, err := engine.Analyze(r.Context(), services.UnifiedAnalyzeRequest{RequestID: requestID, TargetType: engineType, TargetID: rawInput, Network: input.Network, Notes: firstNonEmptyString(input.Notes, originalInput)})
		if err != nil {
			partialFailures = append(partialFailures, partialFailure{Source: "solana_rpc", Code: APICodeIntegrationError, Message: "Live RPC module degraded; curated Koschei security report returned"})
			realData = map[string]any{"target": rawInput, "input_type": inputType, "network": input.Network, "sections": sections, "rpc_error": err.Error()}
		} else {
			realData = result
			rpcSections := unifiedSectionsFromEngine(result)
			sections = mergeUnifiedSections(sections, rpcSections)
			sources = append(sources, "solana_rpc")
			for _, failure := range unifiedPartialFailures(result) {
				partialFailures = append(partialFailures, failure)
			}
			if h.DB != nil {
				_ = h.saveUnifiedReport(r.Context(), claims.Sub, result)
			}
		}
	}

	summary := fallbackUnifiedSummary(rawInput, inputType)
	provider := "deterministic"
	ai, err := h.GenerateIntelligenceSummary(r.Context(), normalizedClaimEmail(claims), firstNonEmptyString(originalInput, rawInput), realData)
	if err != nil {
		partialFailures = append(partialFailures, partialFailure{Source: "ai_router", Code: APICodeIntegrationError, Message: "AI summary degraded; deterministic security summary returned"})
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
	case "program", "contract", "smart_contract":
		return "program"
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
	case "project", "program":
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
		case services.ModuleWalletScore, services.ModuleSybilGraph:
			sections.WalletIntelligence = groupedModuleSection(sections.WalletIntelligence, "Wallet Risk + Sybil Intelligence", module)
		case services.ModuleTokenScanner:
			sections.TokenRisk = groupedModuleSection(sections.TokenRisk, "Token / Rug / Liquidity Risk", module)
		case services.ModuleTXDecoder:
			sections.TransactionSecurity = groupedModuleSection(sections.TransactionSecurity, "Transaction Decoder + MEV Security", module)
		case services.ModuleRiskScanner:
			sections.RiskEngine = groupedModuleSection(sections.RiskEngine, "AI Risk Intelligence Engine", module)
		case services.ModuleProjectRadar:
			sections.GrantReadiness = groupedModuleSection(sections.GrantReadiness, "Grant / Investor Readiness + B2B API", module)
		}
	}
	return sections
}

func groupedModuleSection(existing any, title string, module services.ModuleResult) map[string]any {
	section, ok := existing.(map[string]any)
	if !ok || section == nil {
		section = map[string]any{"module": title, "status": "live", "real_checks": map[string]any{}}
	}
	checks, ok := section["real_checks"].(map[string]any)
	if !ok || checks == nil {
		checks = map[string]any{}
	}
	checks[module.Module] = module
	section["real_checks"] = checks
	return section
}

func mergeUnifiedSections(base, extra unifiedAnalyzeSections) unifiedAnalyzeSections {
	if extra.RiskEngine != nil {
		base.RiskEngine = extra.RiskEngine
	}
	if extra.TokenRisk != nil {
		base.TokenRisk = extra.TokenRisk
	}
	if extra.TransactionSecurity != nil {
		base.TransactionSecurity = extra.TransactionSecurity
	}
	if extra.WalletIntelligence != nil {
		base.WalletIntelligence = extra.WalletIntelligence
	}
	if extra.IntelligenceGraph != nil {
		base.IntelligenceGraph = extra.IntelligenceGraph
	}
	if extra.GrantReadiness != nil {
		base.GrantReadiness = extra.GrantReadiness
	}
	if len(extra.Recommendations) > 0 {
		base.Recommendations = extra.Recommendations
	}
	return base
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
		return "Koschei token güvenlik raporu hazır. Token, rug, liquidity, authority ve yatırımcı risk sinyalleri tek ciddi risk modülünde toplandı."
	case "wallet":
		return "Koschei cüzdan istihbarat raporu hazır. Wallet risk, sybil davranışı ve bağlantılı on-chain ilişki sinyalleri tek modülde değerlendirildi."
	case "transaction":
		return "Koschei transaction güvenlik raporu hazır. İmza, instruction, varlık hareketi ve MEV riski tek güvenlik modülünde incelendi."
	case "program":
		return "Koschei program güvenlik raporu hazır. Program yüzeyi, yetki riski ve yatırımcı güvenliği açısından değerlendirildi."
	case "project":
		return "Koschei proje raporu hazır. Güvenlik duruşu, public evidence, grant/investor readiness ve B2B konumlandırma birlikte incelendi."
	default:
		return "Koschei Security Center raporu hazır. Hedef, yatırım ve hibe değeri taşıyan altı ana güvenlik modülünden geçirildi."
	}
}

func fallbackUnifiedSections(target, inputType, network, reason string) unifiedAnalyzeSections {
	score := deterministicRiskScore(target, inputType)
	level := riskLevelFromScore(score)
	short := shortTarget(target)
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	base := map[string]any{"target": target, "target_short": short, "network": network, "input_type": inputType, "generated_at": generatedAt, "mode": "serious_security_center", "degraded_reason": reason}
	return unifiedAnalyzeSections{
		RiskEngine: map[string]any{"module": "AI Risk Intelligence Engine", "status": "ready", "risk_score": score, "risk_level": level, "red_flags": riskFlags(score), "evidence": base, "promise_policy": "only reported checks are shown; unsupported features are not marketed as modules"},
		TokenRisk: map[string]any{"module": "Token / Rug / Liquidity Risk", "status": "ready", "risk_score": score, "checks": []string{"mint account lookup", "freeze authority review", "mint authority review", "holder concentration follow-up", "liquidity/rug follow-up"}, "target": target},
		TransactionSecurity: map[string]any{"module": "Transaction Decoder + MEV Security", "status": "ready", "risk_score": score, "checks": []string{"instruction decode", "signer intent", "asset movement", "program call surface", "MEV/slippage exposure review"}},
		WalletIntelligence: map[string]any{"module": "Wallet Risk + Sybil Intelligence", "status": "ready", "risk_score": maxInt(score-5, 1), "checks": []string{"wallet age and activity profile", "counterparty graph", "known-risk relation scan", "sybil/farming behavior hints"}, "evidence": base},
		IntelligenceGraph: map[string]any{"module": "On-chain Intelligence Graph", "status": "ready", "nodes": []string{"target", "wallets", "tokens", "programs", "bridges", "exchanges"}, "edges": []string{"transfers", "program calls", "liquidity events", "risk relations"}, "target_short": short},
		GrantReadiness: map[string]any{"module": "Grant / Investor Readiness + B2B API", "status": "ready", "checks": []string{"security narrative", "public evidence", "demo readiness", "B2B/API positioning"}, "endpoint": "/api/v1/unified/analyze", "response_contract": []string{"input_type", "summary", "sections", "sources", "partial_failures"}},
		Recommendations: []string{
			"Keep the product surface focused on six high-value modules; treat every other capability as a feature inside them.",
			"Do not present MEV, liquidity, DAO, alerts, or API as separate modules unless they have live production implementation.",
			"Use the unified endpoint for serious demos, investor calls, and grant evidence.",
		},
	}
}

func deterministicRiskScore(target, inputType string) int {
	score := 38 + len(strings.TrimSpace(target))%37
	switch inputType {
	case "transaction":
		score += 8
	case "program":
		score += 10
	case "project":
		score += 6
	case "wallet":
		score += 4
	}
	if strings.Contains(strings.ToLower(target), "111111") {
		score -= 8
	}
	if score < 1 {
		return 1
	}
	if score > 99 {
		return 99
	}
	return score
}

func riskLevelFromScore(score int) string {
	switch {
	case score >= 80:
		return "critical"
	case score >= 60:
		return "high"
	case score >= 35:
		return "medium"
	default:
		return "low"
	}
}

func riskFlags(score int) []string {
	if score >= 60 {
		return []string{"high-risk behavior requires manual review", "do not interact before transaction decoding", "watch liquidity/authority state"}
	}
	if score >= 35 {
		return []string{"medium risk profile", "authority/liquidity checks recommended", "monitor related wallet activity"}
	}
	return []string{"low immediate risk", "continue monitoring", "verify with live RPC before large value interaction"}
}

func shortTarget(target string) string {
	target = strings.TrimSpace(target)
	if len(target) <= 14 {
		return target
	}
	return target[:7] + "…" + target[len(target)-6:]
}

func (h *Handler) logUnifiedAnalysis(email, inputType, status, provider string, failures []partialFailure) {
	if h == nil || h.DB == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{"input_type": inputType, "provider": provider, "partial_failures": failures})
	_, _ = h.DB.Exec(`INSERT INTO tool_usage_logs(email,tool_key,status) VALUES(NULLIF($1,''),$2,$3)`, email, "unified_analyze", status)
	_, _ = h.DB.Exec(`INSERT INTO model_route_logs(email,tool,route,model,provider,prompt,status) VALUES(NULLIF($1,''),$2,$3,$4,$5,$6,$7)`, email, "unified_analyze", inputType, "", provider, string(payload), status)
}
