package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const projectRadarDisclaimer = "Informational public-data signals only. Not financial, investment, legal, or security advice. Needs manual review before any action."

var projectRadarAllowedCategories = map[string]bool{
	"payments":     true,
	"depin":        true,
	"ai agent":     true,
	"security":     true,
	"token":        true,
	"nft":          true,
	"gaming":       true,
	"consumer app": true,
	"infra":        true,
}

type projectRadarRequest struct {
	ProjectName      string `json:"project_name"`
	Website          string `json:"website"`
	TwitterHandle    string `json:"twitter_handle"`
	TokenMintAddress string `json:"token_mint_address"`
	WalletAddress    string `json:"wallet_address"`
	ChainEcosystem   string `json:"chain_ecosystem"`
	Category         string `json:"category"`
}

type projectRadarResponse struct {
	OK                    bool               `json:"ok"`
	ProjectSummary        string             `json:"project_summary"`
	RiskScore             int                `json:"risk_score"`
	OpportunityScore      int                `json:"opportunity_score"`
	PublicGoodScore       int                `json:"public_good_score"`
	MetadataQuality       string             `json:"metadata_quality"`
	WebsiteSocialQuality  string             `json:"website_social_quality"`
	TokenRiskHints        []string           `json:"token_risk_hints"`
	WalletReputationHints []string           `json:"wallet_reputation_hints"`
	CategoryTrendFit      string             `json:"category_trend_fit"`
	Recommendation        string             `json:"watch_this_project_recommendation"`
	NeedsManualReview     bool               `json:"needs_manual_review"`
	Signals               []string           `json:"signals"`
	RiskHints             []string           `json:"risk_hints"`
	Inputs                projectRadarInputs `json:"inputs"`
	Disclaimer            string             `json:"disclaimer"`
	UsedFallback          bool               `json:"used_fallback"`
}

type projectRadarInputs struct {
	ProjectName      string `json:"project_name"`
	Website          string `json:"website"`
	TwitterHandle    string `json:"twitter_handle"`
	TokenMintAddress string `json:"token_mint_address,omitempty"`
	WalletAddress    string `json:"wallet_address,omitempty"`
	ChainEcosystem   string `json:"chain_ecosystem"`
	Category         string `json:"category"`
}

type projectRadarTokenSignals struct {
	Hints []string
	Risk  int
}

type projectRadarWalletSignals struct {
	Hints       []string
	Opportunity int
	Risk        int
}

func (h *Handler) ProjectRadarScan(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req projectRadarRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	normalized, err := normalizeProjectRadarRequest(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if containsSecretPhrase(normalized.ProjectName, normalized.Website, normalized.TwitterHandle, normalized.TokenMintAddress, normalized.WalletAddress, normalized.ChainEcosystem) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "secret_material_not_allowed", "message": "Do not enter private keys, seed phrases, recovery phrases, or wallet credentials."})
		return
	}

	isPrivileged, credits, err := h.userCreditsAndRole(claims.Sub)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if !isPrivileged && credits < 1 {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}

	tokenSignals := projectRadarTokenSignals{Hints: []string{"No token mint provided; token-specific risk hints need manual review."}}
	if normalized.TokenMintAddress != "" {
		tokenSignals = h.projectRadarTokenHints(r.Context(), normalized)
	}

	walletSignals := projectRadarWalletSignals{Hints: []string{"No wallet address provided; wallet reputation hints need manual review."}}
	if normalized.WalletAddress != "" {
		walletSignals = h.projectRadarWalletHints(r.Context(), normalized)
	}

	result := buildProjectRadarResult(normalized, tokenSignals, walletSignals)

	if !isPrivileged {
		if err := h.spendOutput(claims.Email, "project_radar"); err != nil {
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
			return
		}
	}
	_ = h.saveProjectRadarOutput(r.Context(), strings.ToLower(strings.TrimSpace(claims.Email)), result)
	h.logTool(strings.ToLower(strings.TrimSpace(claims.Email)), "project_radar", "completed")
	h.trackEvent(strings.ToLower(strings.TrimSpace(claims.Email)), "project_radar_scan", r.URL.Path)

	writeJSON(w, http.StatusOK, result)
}

func normalizeProjectRadarRequest(req projectRadarRequest) (projectRadarRequest, error) {
	out := projectRadarRequest{
		ProjectName:      strings.TrimSpace(req.ProjectName),
		Website:          strings.TrimSpace(req.Website),
		TwitterHandle:    normalizeTwitterHandle(req.TwitterHandle),
		TokenMintAddress: strings.TrimSpace(req.TokenMintAddress),
		WalletAddress:    strings.TrimSpace(req.WalletAddress),
		ChainEcosystem:   strings.TrimSpace(req.ChainEcosystem),
		Category:         strings.ToLower(strings.TrimSpace(req.Category)),
	}
	if out.ProjectName == "" || out.Website == "" || out.TwitterHandle == "" || out.ChainEcosystem == "" || out.Category == "" {
		return out, fmt.Errorf("project_name, website, twitter_handle, chain_ecosystem, and category are required")
	}
	if !projectRadarAllowedCategories[out.Category] {
		return out, fmt.Errorf("unsupported category")
	}
	if !strings.HasPrefix(strings.ToLower(out.Website), "https://") && !strings.HasPrefix(strings.ToLower(out.Website), "http://") {
		return out, fmt.Errorf("website must include https:// or http://")
	}
	return out, nil
}

func normalizeTwitterHandle(handle string) string {
	h := strings.TrimSpace(handle)
	h = strings.TrimPrefix(h, "https://x.com/")
	h = strings.TrimPrefix(h, "http://x.com/")
	h = strings.TrimPrefix(h, "https://twitter.com/")
	h = strings.TrimPrefix(h, "http://twitter.com/")
	h = strings.TrimPrefix(h, "@")
	if idx := strings.IndexAny(h, "/?"); idx >= 0 {
		h = h[:idx]
	}
	return h
}

func containsSecretPhrase(values ...string) bool {
	joined := strings.ToLower(strings.Join(values, " "))
	secretTerms := []string{"private key", "seed phrase", "recovery phrase", "mnemonic", "secret key", "wallet password"}
	for _, term := range secretTerms {
		if strings.Contains(joined, term) {
			return true
		}
	}
	words := regexp.MustCompile(`\b[a-z]{3,10}\b`).FindAllString(joined, -1)
	return len(words) >= 12 && strings.Contains(joined, " ") && !strings.Contains(joined, ".") && !strings.Contains(joined, "/")
}

func buildProjectRadarResult(req projectRadarRequest, token projectRadarTokenSignals, wallet projectRadarWalletSignals) projectRadarResponse {
	metadataScore := 45
	riskScore := 45
	opportunityScore := 45
	publicGoodScore := 35
	signals := []string{}
	riskHints := []string{"Needs manual review against official channels, explorer data, and independent community context."}

	if req.Website != "" {
		metadataScore += 12
		opportunityScore += 5
		signals = append(signals, "Website provided as a public metadata anchor.")
		if strings.HasPrefix(strings.ToLower(req.Website), "https://") {
			metadataScore += 6
			riskScore -= 5
			signals = append(signals, "Website uses HTTPS.")
		} else {
			riskScore += 8
			riskHints = append(riskHints, "Website is not HTTPS; verify authenticity carefully.")
		}
	}
	if req.TwitterHandle != "" {
		metadataScore += 10
		opportunityScore += 5
		signals = append(signals, "X/Twitter handle supplied for social verification.")
	}
	if req.TokenMintAddress != "" {
		metadataScore += 6
		riskScore += token.Risk
		riskHints = append(riskHints, token.Hints...)
	}
	if req.WalletAddress != "" {
		metadataScore += 6
		riskScore += wallet.Risk
		opportunityScore += wallet.Opportunity
		riskHints = append(riskHints, wallet.Hints...)
	}

	trend := categoryTrendFit(req.Category)
	switch req.Category {
	case "depin", "ai agent", "security", "infra", "payments":
		opportunityScore += 18
		publicGoodScore += 15
	case "consumer app", "gaming":
		opportunityScore += 12
		publicGoodScore += 8
	case "nft", "token":
		opportunityScore += 8
		riskScore += 8
	}
	if strings.Contains(strings.ToLower(req.ChainEcosystem), "solana") {
		opportunityScore += 8
		signals = append(signals, "Solana ecosystem fit: fast settlement, consumer UX, and low-fee experimentation can support early projects.")
	}

	metadataQuality := qualityLabel(metadataScore)
	websiteSocialQuality := websiteSocialQuality(req)
	riskScore = clampScore(riskScore)
	opportunityScore = clampScore(opportunityScore)
	publicGoodScore = clampScore(publicGoodScore)

	recommendation := "Needs manual review before watching."
	if opportunityScore >= 75 && riskScore <= 55 {
		recommendation = "Watch this project: yes, with manual review and signal monitoring."
	} else if opportunityScore >= 55 && riskScore <= 75 {
		recommendation = "Watch this project: maybe, keep on a low-priority watchlist and re-check public signals."
	} else if riskScore > 75 {
		recommendation = "Watch this project: only for risk monitoring; do not treat this as a positive signal."
	}

	return projectRadarResponse{
		OK:                    true,
		ProjectSummary:        fmt.Sprintf("%s is an early %s project in the %s ecosystem. This radar result combines public metadata, website/social signals, optional token hints, optional wallet/activity hints, and category trend fit. It is not financial advice and needs manual review.", req.ProjectName, req.Category, req.ChainEcosystem),
		RiskScore:             riskScore,
		OpportunityScore:      opportunityScore,
		PublicGoodScore:       publicGoodScore,
		MetadataQuality:       metadataQuality,
		WebsiteSocialQuality:  websiteSocialQuality,
		TokenRiskHints:        token.Hints,
		WalletReputationHints: wallet.Hints,
		CategoryTrendFit:      trend,
		Recommendation:        recommendation,
		NeedsManualReview:     true,
		Signals:               signals,
		RiskHints:             riskHints,
		Inputs: projectRadarInputs{
			ProjectName:      req.ProjectName,
			Website:          req.Website,
			TwitterHandle:    req.TwitterHandle,
			TokenMintAddress: req.TokenMintAddress,
			WalletAddress:    req.WalletAddress,
			ChainEcosystem:   req.ChainEcosystem,
			Category:         req.Category,
		},
		Disclaimer:   projectRadarDisclaimer,
		UsedFallback: true,
	}
}

func categoryTrendFit(category string) string {
	switch category {
	case "depin":
		return "Strong trend fit: DePIN remains a high-attention Solana/Web3 category, but execution and real-world utility need manual review."
	case "ai agent":
		return "Strong trend fit: AI agents are a high-attention category; verify working product, on-chain utility, and safety controls."
	case "security":
		return "Strong public-good fit: security tooling can improve ecosystem resilience if claims are transparent and verifiable."
	case "payments":
		return "Good trend fit: payment projects can benefit from low-fee rails; verify compliance posture and user adoption."
	case "infra":
		return "Good trend fit: infrastructure can compound ecosystem value; verify developer adoption and reliability."
	case "consumer app", "gaming":
		return "Moderate-to-good trend fit: consumer traction matters more than narrative; verify retention and product usage."
	case "nft", "token":
		return "Speculative trend fit: require stronger manual review of token mechanics, ownership, and community authenticity."
	default:
		return "Needs manual trend review."
	}
}

func qualityLabel(score int) string {
	score = clampScore(score)
	switch {
	case score >= 75:
		return "strong"
	case score >= 55:
		return "medium"
	default:
		return "needs manual review"
	}
}

func websiteSocialQuality(req projectRadarRequest) string {
	hasHTTPS := strings.HasPrefix(strings.ToLower(req.Website), "https://")
	hasHandle := strings.TrimSpace(req.TwitterHandle) != ""
	switch {
	case hasHTTPS && hasHandle:
		return "medium — website and social anchors are present; verify ownership and content freshness manually"
	case hasHTTPS || hasHandle:
		return "needs manual review — one public anchor is missing or weak"
	default:
		return "needs manual review"
	}
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func projectRadarSolanaNetwork(ecosystem string) string {
	lower := strings.ToLower(strings.TrimSpace(ecosystem))
	if strings.Contains(lower, "devnet") {
		return "solana-devnet"
	}
	if strings.Contains(lower, "testnet") {
		return "solana-testnet"
	}
	if strings.Contains(lower, "solana") || lower == "" {
		return "solana-mainnet"
	}
	return ecosystem
}

func (h *Handler) projectRadarTokenHints(ctx context.Context, req projectRadarRequest) projectRadarTokenSignals {
	hints := []string{}
	risk := 0
	client := &http.Client{Timeout: 8 * time.Second}
	rpcURL := solanaRPCURL(projectRadarSolanaNetwork(req.ChainEcosystem), os.Getenv("ALCHEMY_API_KEY"))
	var supply struct {
		Value struct {
			Amount   string `json:"amount"`
			Decimals int    `json:"decimals"`
		} `json:"value"`
	}
	if err := callSolanaRPC(client, rpcURL, "getTokenSupply", []interface{}{req.TokenMintAddress}, &supply); err != nil {
		return projectRadarTokenSignals{Hints: []string{"Token mint was provided, but public RPC token checks were unavailable; needs manual explorer review."}, Risk: 18}
	}
	hints = append(hints, fmt.Sprintf("Public RPC found token supply metadata with %d decimals.", supply.Value.Decimals))

	var account struct {
		Value *struct {
			Data struct {
				Parsed struct {
					Info struct {
						MintAuthority   *string `json:"mintAuthority"`
						FreezeAuthority *string `json:"freezeAuthority"`
					} `json:"info"`
				} `json:"parsed"`
			} `json:"data"`
		} `json:"value"`
	}
	_ = callSolanaRPC(client, rpcURL, "getAccountInfo", []interface{}{req.TokenMintAddress, map[string]string{"encoding": "jsonParsed"}}, &account)
	if account.Value != nil {
		if account.Value.Data.Parsed.Info.MintAuthority != nil {
			risk += 18
			hints = append(hints, "Mint authority appears active; supply can potentially change.")
		} else {
			hints = append(hints, "Mint authority appears disabled.")
		}
		if account.Value.Data.Parsed.Info.FreezeAuthority != nil {
			risk += 15
			hints = append(hints, "Freeze authority appears active; token accounts can potentially be frozen.")
		} else {
			hints = append(hints, "Freeze authority appears disabled.")
		}
	}
	_ = ctx
	return projectRadarTokenSignals{Hints: hints, Risk: risk}
}

func (h *Handler) projectRadarWalletHints(ctx context.Context, req projectRadarRequest) projectRadarWalletSignals {
	hints := []string{}
	risk := 0
	opportunity := 0
	client := &http.Client{Timeout: 8 * time.Second}
	rpcURL := solanaRPCURL(projectRadarSolanaNetwork(req.ChainEcosystem), os.Getenv("ALCHEMY_API_KEY"))
	var signatures []struct {
		Signature string `json:"signature"`
		BlockTime *int64 `json:"blockTime"`
		Err       any    `json:"err"`
	}
	if err := callSolanaRPC(client, rpcURL, "getSignaturesForAddress", []interface{}{req.WalletAddress, map[string]int{"limit": 20}}, &signatures); err != nil {
		return projectRadarWalletSignals{Hints: []string{"Wallet was provided, but public RPC activity checks were unavailable; needs manual explorer review."}, Risk: 12}
	}
	hints = append(hints, fmt.Sprintf("Public RPC returned %d recent wallet activity entries.", len(signatures)))
	if len(signatures) == 0 {
		risk += 15
		hints = append(hints, "No recent public wallet activity found in this check.")
	} else if len(signatures) >= 10 {
		opportunity += 10
		hints = append(hints, "Wallet shows multiple recent public activity entries.")
	}
	failed := 0
	for _, sig := range signatures {
		if sig.Err != nil {
			failed++
		}
	}
	if failed >= 5 {
		risk += 8
		hints = append(hints, "Several recent wallet entries show failed transactions; review behavior manually.")
	}
	_ = ctx
	return projectRadarWalletSignals{Hints: hints, Opportunity: opportunity, Risk: risk}
}

func (h *Handler) saveProjectRadarOutput(ctx context.Context, email string, result projectRadarResponse) error {
	if h.DB == nil || email == "" {
		return nil
	}
	content, _ := json.Marshal(result)
	title := result.Inputs.ProjectName
	text := fmt.Sprintf("Project Radar for %s\nRisk score: %d\nOpportunity score: %d\nPublic-good score: %d\nRecommendation: %s\nDisclaimer: %s", title, result.RiskScore, result.OpportunityScore, result.PublicGoodScore, result.Recommendation, result.Disclaimer)
	_, err := h.DB.ExecContext(ctx, `
		INSERT INTO web3_outputs (email, output_type, title, ecosystem, content_json, content_text, used_ai, used_fallback)
		VALUES ($1, 'project_radar', $2, $3, $4::jsonb, $5, false, true)`, email, title, result.Inputs.ChainEcosystem, string(content), text)
	if errorsIsUndefinedColumn(err) {
		return nil
	}
	return err
}

func errorsIsUndefinedColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "undefined_column") || strings.Contains(msg, "does not exist")
}
