package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

const projectRadarDisclaimer = "Project Radar is informational and not financial advice. Koschei does not recommend buying or selling tokens."

type projectRadarRequest struct {
	ProjectName         string `json:"project_name"`
	WebsiteURL          string `json:"website_url"`
	TwitterHandle       string `json:"twitter_handle"`
	GitHubURL           string `json:"github_url"`
	TokenMintAddress    string `json:"token_mint_address"`
	PublicWalletAddress string `json:"public_wallet_address"`
	Ecosystem           string `json:"ecosystem"`
	Category            string `json:"category"`
	Description         string `json:"description"`
	KnownTraction       string `json:"known_traction"`
	Notes               string `json:"notes"`
}

type projectRadarScore struct {
	Score int    `json:"score"`
	Label string `json:"label"`
}

type projectRadarResult struct {
	OK                      bool              `json:"ok"`
	ProjectSummary          string            `json:"project_summary"`
	RiskScore               projectRadarScore `json:"risk_score"`
	OpportunityScore        projectRadarScore `json:"opportunity_score"`
	PublicGoodScore         projectRadarScore `json:"public_good_score"`
	MetadataQuality         string            `json:"metadata_quality"`
	WebsiteSocialQuality    string            `json:"website_social_quality"`
	TokenRiskHints          []string          `json:"token_risk_hints"`
	WalletReputationHints   []string          `json:"wallet_reputation_hints"`
	CategoryTrendFit        string            `json:"category_trend_fit"`
	WatchlistRecommendation string            `json:"watchlist_recommendation"`
	WhatToCheckNext         []string          `json:"what_to_check_next"`
	ManualReviewNotes       []string          `json:"manual_review_notes"`
	ManualReviewNeeded      bool              `json:"manual_review_needed"`
	Signals                 []string          `json:"signals"`
	Disclaimer              string            `json:"disclaimer"`
}

func (h *Handler) ProjectRadar(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	email := normalizedClaimEmail(claims)
	if email == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req projectRadarRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	req = normalizeProjectRadarRequest(req)
	if req.ProjectName == "" || req.Category == "" || req.Description == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_name_category_description_required"})
		return
	}

	result := buildProjectRadarResult(req)
	resultJSON, err := json.Marshal(result)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "project_radar_failed"})
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer tx.Rollback()

	if err := h.applyCreditChargeTxWithReason(tx, claims.Sub, email, "project_radar"); err != nil {
		writeJSON(w, http.StatusPaymentRequired, projectRadarInsufficientOutputsResponse())
		return
	}
	if err := saveProjectRadarOutput(r.Context(), tx, email, req, result, string(resultJSON)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	h.logTool(email, "project_radar", "completed")
	h.trackEvent(email, "project_radar_run", r.URL.Path)
	writeJSON(w, http.StatusOK, result)
}

func projectRadarInsufficientOutputsResponse() map[string]string {
	return map[string]string{
		"error":   "insufficient_outputs",
		"message": "This tool requires paid credits. Demo examples are free.",
	}
}

func normalizeProjectRadarRequest(req projectRadarRequest) projectRadarRequest {
	req.ProjectName = strings.TrimSpace(req.ProjectName)
	req.WebsiteURL = strings.TrimSpace(req.WebsiteURL)
	req.TwitterHandle = strings.TrimSpace(req.TwitterHandle)
	req.GitHubURL = strings.TrimSpace(req.GitHubURL)
	req.TokenMintAddress = strings.TrimSpace(req.TokenMintAddress)
	req.PublicWalletAddress = strings.TrimSpace(req.PublicWalletAddress)
	req.Ecosystem = strings.TrimSpace(req.Ecosystem)
	req.Category = strings.ToLower(strings.TrimSpace(req.Category))
	req.Description = strings.TrimSpace(req.Description)
	req.KnownTraction = strings.TrimSpace(req.KnownTraction)
	req.Notes = strings.TrimSpace(req.Notes)
	return req
}

func buildProjectRadarResult(req projectRadarRequest) projectRadarResult {
	desc := strings.ToLower(req.Description + " " + req.Notes + " " + req.KnownTraction)
	category := strings.ToLower(req.Category)
	ecosystem := strings.ToLower(req.Ecosystem)

	risk := 10
	opportunity := 20
	publicGood := 5
	signals := []string{}
	manual := []string{"Needs manual review against official links, explorer data, public repositories and community context."}
	tokenHints := []string{"Token scanner was not run in this Project Radar v1 heuristic. Treat token-related signals as unknown hints."}
	walletHints := []string{"Wallet reputation was not checked on-chain in this Project Radar v1 heuristic. Review public wallet history manually."}

	if req.WebsiteURL == "" {
		risk += 15
		manual = append(manual, "Missing website increases uncertainty for metadata and legitimacy review.")
	} else {
		signals = append(signals, "Website URL provided for public verification.")
	}
	if weakProjectDescription(req.Description) {
		risk += 10
		manual = append(manual, "Description is short or generic; ask for a clearer problem statement and evidence.")
	}
	if req.TwitterHandle == "" {
		risk += 10
		manual = append(manual, "No X/Twitter handle provided; social quality remains unknown.")
	} else {
		signals = append(signals, "Public social handle provided.")
	}
	if category == "developer tool" && req.GitHubURL == "" {
		risk += 8
		manual = append(manual, "Developer-tool category without GitHub creates implementation-quality uncertainty.")
	}
	if req.TokenMintAddress != "" {
		risk += 5
		tokenHints = append(tokenHints, "Token mint supplied, but token scanner is unavailable in this module run; unknown risk +5 applied.")
	} else {
		tokenHints = append(tokenHints, "No token mint supplied; token-level risks are unknown, not cleared.")
	}
	if suspiciousProjectClaims(desc) {
		risk += 25
		manual = append(manual, "Suspicious promotional wording detected, such as guaranteed, 100x, risk-free or APY language.")
	}
	if req.KnownTraction == "" {
		risk += 5
		manual = append(manual, "No known traction supplied; validate users, usage, grants, commits or partnerships manually.")
	} else {
		signals = append(signals, "Known traction supplied as a public context hint.")
	}

	if inSet(category, "security", "developer tool", "payments", "depin", "ai agent", "public good") {
		opportunity += 15
	}
	if clearProblemStatement(req.Description) {
		opportunity += 15
		signals = append(signals, "Description appears to include a clearer problem or user need.")
	}
	if strings.Contains(ecosystem, "solana") {
		opportunity += 10
		signals = append(signals, "Solana ecosystem fit detected.")
	}
	if req.KnownTraction != "" {
		opportunity += 10
	}
	if req.GitHubURL != "" {
		opportunity += 8
		signals = append(signals, "GitHub URL provided for technical review.")
	}
	if publicGoodLanguage(desc, category) {
		opportunity += 12
	}

	if inSet(category, "security", "education", "developer tool", "public good") {
		publicGood += 25
	}
	if containsAny(desc, []string{"no-custody", "non-custodial", "safety", "safe", "transparency", "transparent"}) {
		publicGood += 20
	}
	if req.GitHubURL != "" || containsAny(desc, []string{"open docs", "documentation", "open-source", "open source", "github"}) {
		publicGood += 15
	}
	if containsAny(desc, []string{"ecosystem", "builders", "developers", "community", "public", "users"}) {
		publicGood += 20
	}

	risk = clampScore(risk)
	opportunity = clampScore(opportunity)
	publicGood = clampScore(publicGood)
	metadataQuality := projectMetadataQuality(req)
	websiteSocialQuality := projectWebsiteSocialQuality(req)
	recommendation := projectWatchlistRecommendation(risk, opportunity, publicGood)

	return projectRadarResult{
		OK:                      true,
		ProjectSummary:          projectSummary(req),
		RiskScore:               projectRadarScore{Score: risk, Label: riskScoreLabel(risk)},
		OpportunityScore:        projectRadarScore{Score: opportunity, Label: upsideScoreLabel(opportunity)},
		PublicGoodScore:         projectRadarScore{Score: publicGood, Label: upsideScoreLabel(publicGood)},
		MetadataQuality:         metadataQuality,
		WebsiteSocialQuality:    websiteSocialQuality,
		TokenRiskHints:          tokenHints,
		WalletReputationHints:   walletHintsFor(req, walletHints),
		CategoryTrendFit:        categoryTrendFit(category),
		WatchlistRecommendation: recommendation,
		WhatToCheckNext: []string{
			"Verify official website, social links and repository ownership.",
			"Review token mint authorities, supply, holders and liquidity on trusted public explorers if a token exists.",
			"Review public wallet activity and counterparties if a wallet is provided.",
			"Check GitHub commit history, docs, issues and releases for developer or security tools.",
			"Ask for traction evidence and independent ecosystem references.",
		},
		ManualReviewNotes:  manual,
		ManualReviewNeeded: true,
		Signals:            signals,
		Disclaimer:         projectRadarDisclaimer,
	}
}

func projectSummary(req projectRadarRequest) string {
	ecosystem := firstNonEmptyString(req.Ecosystem, "Web3")
	return fmt.Sprintf("%s is a %s project in %s. Project Radar produced preliminary public-data signals and hints only; manual review is needed before relying on any score.", req.ProjectName, req.Category, ecosystem)
}

func saveProjectRadarOutput(ctx context.Context, tx *sql.Tx, email string, req projectRadarRequest, result projectRadarResult, resultJSON string) error {
	contentText := fmt.Sprintf("Project Radar: %s\nRisk: %d/100\nOpportunity: %d/100\nPublic-good: %d/100\nRecommendation: %s\nNot financial advice.", req.ProjectName, result.RiskScore.Score, result.OpportunityScore.Score, result.PublicGoodScore.Score, result.WatchlistRecommendation)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO web3_outputs (email, output_type, title, ecosystem, content_json, content_text, used_ai, used_fallback)
		VALUES ($1, 'project_radar', $2, $3, $4::jsonb, $5, false, true)`, email, req.ProjectName, firstNonEmptyString(req.Ecosystem, "web3"), resultJSON, contentText)
	return err
}

func weakProjectDescription(s string) bool {
	trimmed := strings.TrimSpace(s)
	return utf8.RuneCountInString(trimmed) < 80 || !clearProblemStatement(trimmed)
}

func clearProblemStatement(s string) bool {
	text := strings.ToLower(s)
	return utf8.RuneCountInString(strings.TrimSpace(s)) >= 80 && containsAny(text, []string{"problem", "helps", "solve", "need", "users", "builders", "developers", "protect", "improve", "enable"})
}

func suspiciousProjectClaims(s string) bool {
	return regexp.MustCompile(`(?i)\b(guaranteed|100x|risk-free|apy)\b`).MatchString(s)
}

func publicGoodLanguage(desc, category string) bool {
	return inSet(category, "security", "developer tool", "public good") || containsAny(desc, []string{"public good", "open source", "safety", "transparency", "education", "ecosystem", "builders", "developers"})
}

func projectMetadataQuality(req projectRadarRequest) string {
	points := 0
	if req.ProjectName != "" {
		points++
	}
	if req.WebsiteURL != "" {
		points++
	}
	if req.TwitterHandle != "" {
		points++
	}
	if req.GitHubURL != "" {
		points++
	}
	if clearProblemStatement(req.Description) {
		points++
	}
	if req.Ecosystem != "" && req.Category != "" {
		points++
	}
	if points >= 5 {
		return "Strong"
	}
	if points >= 3 {
		return "Medium"
	}
	if points >= 1 {
		return "Low"
	}
	return "Unknown"
}

func projectWebsiteSocialQuality(req projectRadarRequest) string {
	points := 0
	if req.WebsiteURL != "" {
		points++
	}
	if req.TwitterHandle != "" {
		points++
	}
	if req.GitHubURL != "" {
		points++
	}
	if points >= 3 {
		return "Strong"
	}
	if points >= 2 {
		return "Medium"
	}
	if points == 1 {
		return "Low"
	}
	return "Unknown"
}

func walletHintsFor(req projectRadarRequest, hints []string) []string {
	if req.PublicWalletAddress == "" {
		return append(hints, "No public wallet supplied; wallet reputation remains unknown.")
	}
	return append(hints, "Public wallet supplied; review age, counterparties, signer changes and funding source on trusted explorers.")
}

func categoryTrendFit(category string) string {
	if inSet(category, "security", "developer tool", "payments", "depin", "ai agent", "public good") {
		return "High — category is currently relevant for Solana/Web3 builder and public-good workflows, but fit still needs manual evidence review."
	}
	if category == "infrastructure" || category == "consumer app" || category == "gaming" {
		return "Medium — category can be relevant, but traction, differentiation and execution quality matter."
	}
	return "Unknown — category trend fit needs external market and ecosystem review."
}

func projectWatchlistRecommendation(risk, opportunity, publicGood int) string {
	if risk >= 75 && opportunity < 60 {
		return "Avoid"
	}
	if risk >= 60 {
		return "Review"
	}
	if opportunity >= 65 || publicGood >= 65 {
		return "Watch"
	}
	return "Unknown"
}

func riskScoreLabel(score int) string {
	if score >= 67 {
		return "High"
	}
	if score >= 34 {
		return "Medium"
	}
	return "Low"
}

func upsideScoreLabel(score int) string {
	if score >= 67 {
		return "High"
	}
	if score >= 34 {
		return "Medium"
	}
	if score >= 0 {
		return "Low"
	}
	return "Unknown"
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

func inSet(v string, candidates ...string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, c := range candidates {
		if v == strings.ToLower(c) {
			return true
		}
	}
	return false
}

func containsAny(s string, needles []string) bool {
	s = strings.ToLower(s)
	for _, n := range needles {
		if strings.Contains(s, strings.ToLower(n)) {
			return true
		}
	}
	return false
}
