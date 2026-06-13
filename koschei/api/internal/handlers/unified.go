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

// UnifiedIntelligenceHandler runs the enterprise intelligence modules and
// aggregates them into one persisted report. Premium access is enforced by the
// route middleware; this handler must not reintroduce credit-based gates.
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
	rawInput := strings.TrimSpace(firstNonEmptyString(input.Input, input.TargetID, input.Target))
	if rawInput == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Input is required", nil)
		return
	}
	if input.Network == "" {
		input.Network = "solana-mainnet"
	}
	inputType := detectUnifiedInputType(rawInput, input.TargetType, input.Context)
	inputType = h.resolveUnifiedInputType(r.Context(), rawInput, input.Network, inputType)
	if inputType == "unknown" {
		inputType = "question"
	}

	requestID := newRequestID()
	sections := unifiedAnalyzeSections{Recommendations: []string{}}
	partialFailures := []partialFailure{}
	sources := []string{}
	var realData any

	if inputType == "question" {
		realData = map[string]any{"question": rawInput}
	} else {
		engineType := mapUnifiedInputTypeToEngine(inputType)
		engine := services.NewUnifiedEngine(h.SolanaRPC)
		result, err := engine.Analyze(r.Context(), services.UnifiedAnalyzeRequest{RequestID: requestID, TargetType: engineType, TargetID: rawInput, Network: input.Network, Notes: input.Notes})
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid input", nil)
			return
		}
		realData = result
		sections = unifiedSectionsFromEngine(result)
		sources = append(sources, "solana_rpc")
		for _, failure := range unifiedPartialFailures(result) {
			partialFailures = append(partialFailures, failure)
		}
		if h.DB != nil {
			_ = h.saveUnifiedReport(r.Context(), claims.Sub, result)
		}
	}

	summary := "Real data analysis completed. AI summary unavailable."
	if inputType == "question" {
		summary = "AI answer unavailable."
	}
	provider := "none"
	ai, err := h.GenerateIntelligenceSummary(r.Context(), normalizedClaimEmail(claims), rawInput, realData)
	if err != nil {
		partialFailures = append(partialFailures, partialFailure{Source: "ai_router", Code: APICodeIntegrationError, Message: "Real data unavailable"})
		if inputType == "question" {
			h.logUnifiedAnalysis(normalizedClaimEmail(claims), inputType, APICodeIntegrationError, provider, partialFailures)
			writeAPIError(w, http.StatusBadGateway, APICodeIntegrationError, "AI provider unavailable", nil)
			return
		}
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
	if strings.Contains(trimmed, " ") || strings.Contains(trimmed, "?") {
		return "question"
	}
	if decoded, ok := base58Decode(trimmed); ok {
		if len(decoded) == 64 || len(trimmed) >= 87 {
			return "transaction"
		}
		if len(decoded) == 32 {
			return "question"
		}
	}
	if strings.Contains(trimmed, ".") {
		return "project"
	}
	return "question"
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

func (h *Handler) logUnifiedAnalysis(email, inputType, status, provider string, failures []partialFailure) {
	if h == nil || h.DB == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{"input_type": inputType, "provider": provider, "partial_failures": failures})
	_, _ = h.DB.Exec(`INSERT INTO tool_usage_logs(email,tool_key,status) VALUES(NULLIF($1,''),$2,$3)`, email, "unified_analyze", status)
	_, _ = h.DB.Exec(`INSERT INTO model_route_logs(email,tool,route,model,provider,prompt,status) VALUES(NULLIF($1,''),$2,$3,$4,$5,$6,$7)`, email, "unified_analyze", inputType, "", provider, string(payload), status)
}
