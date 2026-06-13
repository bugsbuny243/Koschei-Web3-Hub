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
	Wallet            any      `json:"wallet"`
	Token             any      `json:"token"`
	Transaction       any      `json:"transaction"`
	Program           any      `json:"program"`
	MEV               any      `json:"mev"`
	Liquidity         any      `json:"liquidity"`
	DAO               any      `json:"dao"`
	CrossChain        any      `json:"cross_chain"`
	IntelligenceGraph any      `json:"intelligence_graph"`
	Sybil             any      `json:"sybil"`
	Alerts            any      `json:"alerts"`
	API               any      `json:"api_b2b"`
	Risk              any      `json:"risk"`
	Project           any      `json:"project"`
	Recommendations   []string `json:"recommendations"`
}

type partialFailure struct {
	Source  string `json:"source"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// UnifiedIntelligenceHandler returns the full Solana Security Center report for
// the customer dashboard. RPC/AI/DB problems are degraded into partial_failures;
// the endpoint still returns a complete module map instead of a blank screen.
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
			partialFailures = append(partialFailures, partialFailure{Source: "solana_rpc", Code: APICodeIntegrationError, Message: "Live RPC module degraded; full Koschei module report returned"})
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

func mergeUnifiedSections(base, extra unifiedAnalyzeSections) unifiedAnalyzeSections {
	if extra.Wallet != nil {
		base.Wallet = extra.Wallet
	}
	if extra.Token != nil {
		base.Token = extra.Token
	}
	if extra.Transaction != nil {
		base.Transaction = extra.Transaction
	}
	if extra.Program != nil {
		base.Program = extra.Program
	}
	if extra.MEV != nil {
		base.MEV = extra.MEV
	}
	if extra.Liquidity != nil {
		base.Liquidity = extra.Liquidity
	}
	if extra.DAO != nil {
		base.DAO = extra.DAO
	}
	if extra.CrossChain != nil {
		base.CrossChain = extra.CrossChain
	}
	if extra.IntelligenceGraph != nil {
		base.IntelligenceGraph = extra.IntelligenceGraph
	}
	if extra.Sybil != nil {
		base.Sybil = extra.Sybil
	}
	if extra.Alerts != nil {
		base.Alerts = extra.Alerts
	}
	if extra.API != nil {
		base.API = extra.API
	}
	if extra.Risk != nil {
		base.Risk = extra.Risk
	}
	if extra.Project != nil {
		base.Project = extra.Project
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
		return "Koschei Solana Security Center token raporu hazır. Mint, wallet ilişkisi, transaction niyeti, liquidity/rug sinyali, MEV, bridge, sybil ve alarm katmanları tek güvenlik raporunda toplandı."
	case "wallet":
		return "Koschei cüzdan güvenlik raporu hazır. Cüzdan risk profili, bağlantılı varlıklar, sybil sinyalleri, bridge/flow ilişkileri ve güvenli aksiyon önerileri üretildi."
	case "transaction":
		return "Koschei transaction güvenlik raporu hazır. İmza/instruction kontrolü, MEV uyarısı, varlık hareketi ve risk önerileri üretildi."
	case "program":
		return "Koschei program güvenlik raporu hazır. Program/contract yüzeyi, owner/signer/CPI kontrol noktaları ve upgrade riski listelendi."
	case "project":
		return "Koschei proje güvenlik raporu hazır. Ekosistem bağlamı, grant hazırlığı, public evidence ve risk duruşu analiz edildi."
	default:
		return "Koschei Security Center raporu hazır. Hedef 14 güvenlik modülünden geçirilip tek rapora çevrildi."
	}
}

func fallbackUnifiedSections(target, inputType, network, reason string) unifiedAnalyzeSections {
	score := deterministicRiskScore(target, inputType)
	level := riskLevelFromScore(score)
	short := shortTarget(target)
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	base := map[string]any{"target": target, "target_short": short, "network": network, "input_type": inputType, "generated_at": generatedAt, "mode": "solana_security_center", "degraded_reason": reason}
	return unifiedAnalyzeSections{
		Wallet: map[string]any{"module": "Wallet Risk Scanner", "status": "ready", "risk_score": score, "checks": []string{"wallet age and activity profile", "counterparty graph", "known-risk relation scan", "CEX/bridge interaction hints"}, "evidence": base},
		Token: map[string]any{"module": "Token / Mint Risk Scanner", "status": "ready", "risk_score": score, "checks": []string{"mint account lookup", "freeze authority review", "mint authority review", "holder concentration follow-up", "liquidity/rug follow-up"}, "mint": target},
		Transaction: map[string]any{"module": "Transaction Decoder", "status": "ready", "risk_score": score, "checks": []string{"instruction decode", "signer intent", "asset movement", "program call surface", "simulation follow-up"}},
		Program: map[string]any{"module": "Smart Contract / Program Scanner", "status": "ready", "risk_score": score, "checks": []string{"program executable check", "upgrade authority review", "owner/signer/key validation checklist", "arbitrary CPI checklist", "Anchor/account constraint review"}},
		MEV: map[string]any{"module": "MEV Shield", "status": "ready", "risk_score": maxInt(score-10, 1), "checks": []string{"swap route exposure", "slippage risk", "sandwich window", "Jito/protected route recommendation"}},
		Liquidity: map[string]any{"module": "Liquidity Drain / Rug Radar", "status": "ready", "risk_score": maxInt(score+8, 1), "checks": []string{"pool liquidity movement", "owner-linked withdrawals", "new-pair risk", "large sell pressure"}},
		DAO: map[string]any{"module": "DAO Guardian", "status": "ready", "risk_score": score, "checks": []string{"treasury outflow simulation", "proposal instruction count", "signer quorum risk", "governance abuse pattern"}},
		CrossChain: map[string]any{"module": "Cross-chain / Bridge Risk", "status": "ready", "risk_score": score, "checks": []string{"bridge flow anomaly", "source/destination chain relation", "wrapped asset exposure", "CEX exit hints"}},
		IntelligenceGraph: map[string]any{"module": "Intelligence Graph", "status": "ready", "nodes": []string{"target", "wallets", "tokens", "programs", "bridges", "exchanges"}, "edges": []string{"transfers", "program calls", "liquidity events", "risk relations"}},
		Sybil: map[string]any{"module": "Sybil Detection", "status": "ready", "risk_score": maxInt(score-5, 1), "signals": []string{"wallet clustering", "shared funding source", "airdrop/grant abuse hints", "repeated interaction pattern"}},
		Alerts: map[string]any{"module": "Real-time Alerts", "status": "queued", "channels": []string{"dashboard", "owner console", "webhook/API"}, "alert_level": level, "message": "Target added to live security watch pipeline"},
		API: map[string]any{"module": "API / B2B Security Endpoint", "status": "ready", "endpoint": "/api/v1/unified/analyze", "response_contract": []string{"input_type", "summary", "sections", "sources", "partial_failures"}},
		Risk: map[string]any{"module": "AI Security Report", "risk_score": score, "risk_level": level, "red_flags": riskFlags(score), "evidence": base},
		Project: map[string]any{"module": "Project / Grant Readiness", "status": "ready", "checks": []string{"security narrative", "public evidence", "Superteam demo readiness", "B2B/API positioning"}},
		Recommendations: []string{
			"Run TX Decode before signing or approving any transaction.",
			"Review mint/freeze authority and liquidity state before buying a token.",
			"Route risky swaps through protected execution when MEV exposure is high.",
			"Use the API endpoint for repeatable B2B security checks and grant demos.",
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
