package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const token2022ProgramID = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"

type tokenScanRequest struct {
	Mint    string `json:"mint"`
	Address string `json:"address"`
	Network string `json:"network"`
}

type tokenExtensionAssessment struct {
	Name        string         `json:"name"`
	Severity    string         `json:"severity"`
	RiskPenalty int            `json:"risk_penalty"`
	Summary     string         `json:"summary"`
	Details     map[string]any `json:"details,omitempty"`
}

type tokenScanResponse struct {
	Mint                     string                     `json:"mint"`
	Network                  string                     `json:"network"`
	Score                    int                        `json:"score"`
	RiskLevel                string                     `json:"risk_level"`
	Supply                   string                     `json:"supply"`
	Decimals                 int                        `json:"decimals"`
	MintAuthority            string                     `json:"mint_authority,omitempty"`
	FreezeAuthority          string                     `json:"freeze_authority,omitempty"`
	LargestHolderPercent     float64                    `json:"largest_holder_percent"`
	TopTenPercent            float64                    `json:"top_ten_percent"`
	Findings                 []string                   `json:"findings"`
	TokenProgram             string                     `json:"token_program"`
	Token2022                bool                       `json:"token_2022"`
	Extensions               []tokenExtensionAssessment `json:"extensions"`
	ExtensionRiskPenalty     int                        `json:"extension_risk_penalty"`
	TransferBehavior         map[string]any             `json:"transfer_behavior"`
	VisibilityLimitations    []string                   `json:"visibility_limitations"`
	CompatibilityWarnings    []string                   `json:"compatibility_warnings"`
	FinalPolicy              string                     `json:"final_policy"`
	Disclaimer               string                     `json:"disclaimer"`
}

type rpcEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (h *Handler) TokenScan(w http.ResponseWriter, r *http.Request) {
	var req tokenScanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	mint := strings.TrimSpace(req.Mint)
	if mint == "" {
		mint = strings.TrimSpace(req.Address)
	}
	if mint == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mint required"})
		return
	}
	if req.Network == "" {
		req.Network = "solana-mainnet"
	}

	client := &http.Client{Timeout: 12 * time.Second}
	rpcURL := solanaRPCURL(req.Network, os.Getenv("ALCHEMY_API_KEY"))
	var supply struct {
		Value struct {
			Amount   string `json:"amount"`
			Decimals int    `json:"decimals"`
		} `json:"value"`
	}
	if err := h.callSolanaRPC(r.Context(), client, rpcURL, req.Network, "getTokenSupply", []interface{}{mint}, &supply); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "token mint not found or RPC request failed"})
		return
	}

	var account struct {
		Value *struct {
			Owner string `json:"owner"`
			Data  struct {
				Program string `json:"program"`
				Parsed  struct {
					Type string         `json:"type"`
					Info map[string]any `json:"info"`
				} `json:"parsed"`
				Space int `json:"space"`
			} `json:"data"`
		} `json:"value"`
	}
	_ = h.callSolanaRPC(r.Context(), client, rpcURL, req.Network, "getAccountInfo", []interface{}{mint, map[string]string{"encoding": "jsonParsed"}}, &account)

	var largest struct {
		Value []struct {
			Amount string `json:"amount"`
		} `json:"value"`
	}
	_ = h.callSolanaRPC(r.Context(), client, rpcURL, req.Network, "getTokenLargestAccounts", []interface{}{mint}, &largest)

	total, _ := strconv.ParseFloat(supply.Value.Amount, 64)
	topOne, topTen := 0.0, 0.0
	for i, holder := range largest.Value {
		amount, _ := strconv.ParseFloat(holder.Amount, 64)
		if total > 0 && i < 10 {
			topTen += amount / total * 100
			if i == 0 {
				topOne = amount / total * 100
			}
		}
	}

	score := 100
	findings := []string{}
	mintAuthority, freezeAuthority := "", ""
	tokenProgram := "spl-token"
	isToken2022 := false
	extensions := []tokenExtensionAssessment{}
	transferBehavior := map[string]any{"standard_transfer": true}
	visibilityLimitations := []string{}
	compatibilityWarnings := []string{}
	extensionPenalty := 0

	if account.Value != nil {
		info := account.Value.Data.Parsed.Info
		mintAuthority = tokenInfoString(info, "mintAuthority")
		freezeAuthority = tokenInfoString(info, "freezeAuthority")
		if mintAuthority != "" {
			score -= 25
			findings = append(findings, "Mint authority is active and can create additional supply.")
		} else {
			findings = append(findings, "Mint authority is disabled.")
		}
		if freezeAuthority != "" {
			score -= 20
			findings = append(findings, "Freeze authority is active and can freeze token accounts.")
		} else {
			findings = append(findings, "Freeze authority is disabled.")
		}

		isToken2022 = account.Value.Owner == token2022ProgramID || strings.EqualFold(account.Value.Data.Program, "spl-token-2022")
		if isToken2022 {
			tokenProgram = "token-2022"
			extensions = parseToken2022Extensions(info)
			extensionPenalty, transferBehavior, visibilityLimitations, compatibilityWarnings = summarizeToken2022Extensions(extensions)
			score -= extensionPenalty
			findings = append(findings, token2022Findings(extensions)...)
			if len(extensions) == 0 {
				findings = append(findings, "Token is owned by the Token-2022 program and no active mint extensions were reported by the RPC parser.")
			}
		}
	}

	if topOne >= 50 {
		score -= 35
		findings = append(findings, "The largest token account controls at least half of the supply.")
	} else if topOne >= 20 {
		score -= 20
		findings = append(findings, "The largest token account has a significant concentration.")
	}
	if topTen >= 80 {
		score -= 20
		findings = append(findings, "The ten largest token accounts control most of the supply.")
	}
	if score < 0 {
		score = 0
	}

	risk := tokenRiskLevel(score)
	policy := tokenFinalPolicy(score, extensions, visibilityLimitations)
	disclaimer := "Koschei provides read-only risk signals based on public on-chain data. Token extensions can be legitimate but may materially change transfer, authority, fee, privacy, or compatibility behavior. This is not financial advice."

	writeJSON(w, http.StatusOK, tokenScanResponse{
		Mint:                  mint,
		Network:               req.Network,
		Score:                 score,
		RiskLevel:             risk,
		Supply:                supply.Value.Amount,
		Decimals:              supply.Value.Decimals,
		MintAuthority:         mintAuthority,
		FreezeAuthority:       freezeAuthority,
		LargestHolderPercent:  roundPercent(topOne),
		TopTenPercent:         roundPercent(topTen),
		Findings:              findings,
		TokenProgram:          tokenProgram,
		Token2022:             isToken2022,
		Extensions:            extensions,
		ExtensionRiskPenalty:  extensionPenalty,
		TransferBehavior:      transferBehavior,
		VisibilityLimitations: visibilityLimitations,
		CompatibilityWarnings: compatibilityWarnings,
		FinalPolicy:           policy,
		Disclaimer:            disclaimer,
	})
}

func parseToken2022Extensions(info map[string]any) []tokenExtensionAssessment {
	raw, ok := info["extensions"]
	if !ok {
		return []tokenExtensionAssessment{}
	}
	items, ok := raw.([]any)
	if !ok {
		return []tokenExtensionAssessment{}
	}
	out := make([]tokenExtensionAssessment, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := firstMapStringValue(object, "extension", "extensionType", "type")
		if name == "" {
			continue
		}
		details := map[string]any{}
		if state, ok := object["state"].(map[string]any); ok {
			details = state
		} else {
			for key, value := range object {
				if key == "extension" || key == "extensionType" || key == "type" {
					continue
				}
				details[key] = value
			}
		}
		out = append(out, assessToken2022Extension(name, details))
	}
	return out
}

func assessToken2022Extension(name string, details map[string]any) tokenExtensionAssessment {
	normalized := normalizeExtensionName(name)
	assessment := tokenExtensionAssessment{Name: canonicalExtensionName(name), Severity: "info", RiskPenalty: 0, Summary: "Token-2022 extension is active.", Details: details}
	switch normalized {
	case "permanentdelegate":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "critical", 50, "A permanent delegate may transfer or burn tokens from holder accounts."
	case "transferhook":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "high", 30, "A custom program can run on every token transfer."
	case "transferfeeconfig":
		bps := nestedNumber(details, "transferFeeBasisPoints")
		assessment.Severity, assessment.RiskPenalty = "medium", 18
		assessment.Summary = "Protocol-level transfer fees are enabled."
		if bps >= 1000 {
			assessment.Severity, assessment.RiskPenalty = "high", 30
			assessment.Summary = "High protocol-level transfer fees are configured."
		}
		if bps >= 5000 {
			assessment.Severity, assessment.RiskPenalty = "critical", 45
			assessment.Summary = "Extremely high protocol-level transfer fees are configured."
		}
	case "mintcloseauthority":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "high", 30, "The mint account can be closed by an authority."
	case "defaultaccountstate":
		state := strings.ToLower(nestedString(details, "state"))
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "medium", 15, "New token accounts use a configured default state."
		if strings.Contains(state, "frozen") {
			assessment.Severity, assessment.RiskPenalty, assessment.Summary = "high", 30, "New token accounts start frozen and require authority intervention."
		}
	case "nontransferable":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "medium", 20, "The token is non-transferable (soulbound behavior)."
	case "confidentialtransfermint":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "medium", 18, "Confidential balances or transfer amounts may limit public visibility."
	case "confidentialtransferfeeconfig":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "high", 25, "Confidential transfer-fee behavior limits complete public inspection."
	case "confidentialmintburn":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "high", 30, "Confidential mint or burn activity may not be fully observable from public amounts."
	case "pausableconfig":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "high", 35, "An authority can pause token transfers globally."
	case "scaleduiamountconfig":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "medium", 15, "Displayed token amounts can be scaled independently from raw balances."
	case "interestbearingconfig":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "low", 5, "UI balances may include an interest-bearing display calculation."
	case "metadatapointer":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "low", 2, "Metadata is resolved through a configured pointer."
	case "tokenmetadata":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "info", 0, "Token metadata is stored through the Token-2022 metadata extension."
	case "grouppointer", "tokengroup", "groupmemberpointer", "tokengroupmember":
		assessment.Severity, assessment.RiskPenalty, assessment.Summary = "info", 0, "Token grouping metadata is enabled."
	default:
		assessment.Summary = "An extension not yet assigned a Koschei risk rule is active and should be reviewed for compatibility."
	}
	return assessment
}

func summarizeToken2022Extensions(extensions []tokenExtensionAssessment) (int, map[string]any, []string, []string) {
	penalty := 0
	behavior := map[string]any{
		"standard_transfer": true,
		"transfer_fee":      false,
		"transfer_hook":     false,
		"non_transferable":  false,
		"pausable":          false,
		"permanent_delegate": false,
	}
	visibility := []string{}
	compatibility := []string{}
	seenVisibility := map[string]struct{}{}
	seenCompatibility := map[string]struct{}{}
	for _, extension := range extensions {
		penalty += extension.RiskPenalty
		normalized := normalizeExtensionName(extension.Name)
		switch normalized {
		case "transferfeeconfig":
			behavior["transfer_fee"] = true
			behavior["standard_transfer"] = false
			if value := nestedNumber(extension.Details, "transferFeeBasisPoints"); value > 0 {
				behavior["transfer_fee_basis_points"] = value
			}
			if value := nestedAny(extension.Details, "maximumFee"); value != nil {
				behavior["maximum_fee"] = value
			}
			appendUnique(&compatibility, seenCompatibility, "Integrations must account for protocol-level transfer fees.")
		case "transferhook":
			behavior["transfer_hook"] = true
			behavior["standard_transfer"] = false
			if value := nestedString(extension.Details, "programId"); value != "" {
				behavior["transfer_hook_program"] = value
			}
			appendUnique(&compatibility, seenCompatibility, "Every transfer may invoke a custom program and require additional accounts.")
		case "nontransferable":
			behavior["non_transferable"] = true
			behavior["standard_transfer"] = false
			appendUnique(&compatibility, seenCompatibility, "The token cannot be transferred through normal wallet or DEX flows.")
		case "pausableconfig":
			behavior["pausable"] = true
			behavior["standard_transfer"] = false
			appendUnique(&compatibility, seenCompatibility, "Token transfers may be paused globally by an authority.")
		case "permanentdelegate":
			behavior["permanent_delegate"] = true
			if value := nestedString(extension.Details, "delegate"); value != "" {
				behavior["permanent_delegate_address"] = value
			}
			appendUnique(&compatibility, seenCompatibility, "A permanent delegate may transfer or burn holder balances.")
		case "confidentialtransfermint", "confidentialtransferfeeconfig", "confidentialmintburn":
			appendUnique(&visibility, seenVisibility, "Confidential Token-2022 features may hide balances, transfer amounts, fees, minting, or burning from ordinary public inspection.")
		case "scaleduiamountconfig":
			appendUnique(&compatibility, seenCompatibility, "Displayed balances may differ from raw token amounts because of UI scaling.")
		case "defaultaccountstate":
			appendUnique(&compatibility, seenCompatibility, "New token accounts may require thawing before they can receive or transfer tokens.")
		default:
			if extension.Summary == "An extension not yet assigned a Koschei risk rule is active and should be reviewed for compatibility." {
				appendUnique(&compatibility, seenCompatibility, "Unknown or newly introduced Token-2022 extension: "+extension.Name)
			}
		}
	}
	if penalty > 75 {
		penalty = 75
	}
	return penalty, behavior, visibility, compatibility
}

func token2022Findings(extensions []tokenExtensionAssessment) []string {
	out := make([]string, 0, len(extensions)+1)
	if len(extensions) > 0 {
		out = append(out, fmt.Sprintf("Token-2022 mint exposes %d parsed extension(s).", len(extensions)))
	}
	for _, extension := range extensions {
		out = append(out, extension.Name+": "+extension.Summary)
	}
	return out
}

func tokenFinalPolicy(score int, extensions []tokenExtensionAssessment, visibility []string) string {
	for _, extension := range extensions {
		if extension.Severity == "critical" {
			return "block"
		}
	}
	if len(visibility) > 0 {
		return "warn"
	}
	for _, extension := range extensions {
		if extension.Severity == "high" || extension.Severity == "medium" {
			return "warn"
		}
	}
	if score < 40 {
		return "block"
	}
	if score < 70 {
		return "warn"
	}
	return "allow"
}

func tokenRiskLevel(score int) string {
	if score < 40 {
		return "high"
	}
	if score < 70 {
		return "medium"
	}
	return "low"
}

func tokenInfoString(info map[string]any, key string) string {
	value, exists := info[key]
	if !exists || value == nil {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func firstMapStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nestedAny(value any, key string) any {
	target := normalizeExtensionName(key)
	switch current := value.(type) {
	case map[string]any:
		for candidate, child := range current {
			if normalizeExtensionName(candidate) == target {
				return child
			}
		for _, child := range current {
			if found := nestedAny(child, key); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range current {
			if found := nestedAny(child, key); found != nil {
				return found
			}
		}
	}
	return nil
}

func nestedString(value any, key string) string {
	found := nestedAny(value, key)
	switch typed := found.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func nestedNumber(value any, key string) float64 {
	found := nestedAny(value, key)
	switch typed := found.(type) {
	case float64:
		return typed
	case json.Number:
		number, _ := typed.Float64()
		return number
	case string:
		number, _ := strconv.ParseFloat(typed, 64)
		return number
	default:
		return 0
	}
}

func appendUnique(target *[]string, seen map[string]struct{}, value string) {
	if _, exists := seen[value]; exists {
		return
	}
	seen[value] = struct{}{}
	*target = append(*target, value)
}

func canonicalExtensionName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "UnknownExtension"
	}
	return value
}

func normalizeExtensionName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func (h *Handler) callSolanaRPC(ctx context.Context, client *http.Client, rpcURL, network, method string, params interface{}, target interface{}) error {
	if h != nil && h.SolanaRPC != nil {
		return h.SolanaRPC.Call(ctx, network, method, params, target, 0)
	}
	return callSolanaRPC(client, rpcURL, method, params, target)
}

func callSolanaRPC(client *http.Client, rpcURL, method string, params interface{}, target interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return err
	}
	resp, err := client.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rpc status %d", resp.StatusCode)
	}
	var envelope rpcEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("rpc error: %s", envelope.Error.Message)
	}
	return json.Unmarshal(envelope.Result, target)
}

func roundPercent(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
