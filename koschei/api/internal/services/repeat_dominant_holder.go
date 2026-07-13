package services

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const RepeatDominantObservationDays = 30

type RepeatDominantHolderMatch struct {
	Mint       string  `json:"mint"`
	Percentage float64 `json:"percentage"`
	Rank       int     `json:"rank"`
	ScannedAt  string  `json:"scanned_at"`
}

type RepeatDominantHolderEvidence struct {
	OwnerWallet       string                      `json:"owner_wallet"`
	CurrentMint       string                      `json:"current_mint"`
	CurrentPercentage float64                     `json:"current_percentage"`
	TokenCount        int                         `json:"token_count"`
	ObservationDays   int                         `json:"observation_days"`
	ObservationWindow string                      `json:"observation_window"`
	RiskWeight        int                         `json:"risk_weight"`
	Matches           []RepeatDominantHolderMatch `json:"matches"`
	EvidenceLine      string                      `json:"evidence_line"`
}

func RepeatDominantRiskWeight(currentPercentage float64, tokenCount int) int {
	if currentPercentage < 20 || tokenCount < 2 {
		return 0
	}
	risk := 55 + int(currentPercentage/5) + (tokenCount-1)*6
	if risk > 90 {
		risk = 90
	}
	if risk < 65 {
		risk = 65
	}
	return risk
}

func RepeatDominantEvidenceLine(owner string, matches []RepeatDominantHolderMatch, days int) string {
	if days <= 0 {
		days = RepeatDominantObservationDays
	}
	items := make([]string, 0, len(matches))
	for _, match := range matches {
		date := strings.TrimSpace(match.ScannedAt)
		if parsed, err := time.Parse(time.RFC3339, date); err == nil {
			date = parsed.UTC().Format("2006-01-02")
		}
		items = append(items, fmt.Sprintf("%s %.2f%% (%s)", repeatDominantShortMint(match.Mint), match.Percentage, date))
	}
	return fmt.Sprintf("REPEAT DOMINANT HOLDER: Bu cüzdan son %d gün Koschei gözleminde %d farklı token'da top-5 holder: %s. Bu yalnızca saklanan zincir üstü holder gözlemlerini bağlar; kimlik veya niyet iddiası değildir.", days, len(matches), strings.Join(items, ", "))
}

func repeatDominantShortMint(mint string) string {
	mint = strings.TrimSpace(mint)
	if len(mint) <= 18 {
		return mint
	}
	return mint[:8] + "…" + mint[len(mint)-6:]
}

func ApplyRepeatDominantHolderEvidenceToHolderIntelligence(in HolderIntelligence, evidence []RepeatDominantHolderEvidence) HolderIntelligence {
	if len(evidence) == 0 {
		return in
	}
	byOwner := map[string]RepeatDominantHolderEvidence{}
	for _, item := range evidence {
		byOwner[strings.TrimSpace(item.OwnerWallet)] = item
		if strings.TrimSpace(item.EvidenceLine) != "" {
			in.Findings = appendUniqueHolderEvidence(in.Findings, item.EvidenceLine)
		}
	}
	for i := range in.Rows {
		item, ok := byOwner[strings.TrimSpace(in.Rows[i].OwnerWallet)]
		if !ok {
			continue
		}
		in.Rows[i].RepeatDominantHolder = true
		in.Rows[i].RepeatDominantTokenCount = item.TokenCount
		in.Rows[i].RepeatDominantObservationWindow = item.ObservationWindow
		in.Rows[i].RepeatDominantRiskWeight = item.RiskWeight
		in.Rows[i].RepeatDominantMatches = append([]RepeatDominantHolderMatch{}, item.Matches...)
		in.Rows[i].Evidence = appendUniqueHolderEvidence(in.Rows[i].Evidence, item.EvidenceLine)
		in.RepeatDominantHolderCount++
	}
	return in
}

func ApplyRepeatDominantHolderEvidenceToAnalysis(analysis ArvisAnalysis, req SecurityRadarRequest, evidence []RepeatDominantHolderEvidence) ArvisAnalysis {
	if len(evidence) == 0 {
		return analysis
	}
	strongest := 0
	lines := []string{}
	owners := []string{}
	for _, item := range evidence {
		if item.RiskWeight > strongest {
			strongest = item.RiskWeight
		}
		if strings.TrimSpace(item.EvidenceLine) != "" {
			lines = append(lines, item.EvidenceLine)
		}
		if strings.TrimSpace(item.OwnerWallet) != "" {
			owners = append(owners, item.OwnerWallet)
		}
	}
	if strongest <= 0 {
		return analysis
	}

	arms := make([]SecurityRadarVerdict, 0, len(analysis.Arms)+1)
	graphFound := false
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	for _, arm := range analysis.Arms {
		if arm.ModuleID == ModuleFinalVerdictEngine {
			continue
		}
		if arm.ModuleID != ModuleIntelligenceGraph {
			arms = append(arms, arm)
			continue
		}
		graphFound = true
		if arm.Signals == nil {
			arm.Signals = map[string]any{}
		}
		arm.Signals["repeat_dominant_holder"] = true
		arm.Signals["repeat_dominant_holders"] = evidence
		arm.Signals["repeat_dominant_owner_wallets"] = owners
		arm.Signals["repeat_dominant_observation_days"] = RepeatDominantObservationDays
		arm.Signals["repeat_dominant_risk_weight"] = strongest
		arm.Signals["stored_scan_evidence"] = true
		arm.Signals["real_onchain_evidence"] = true
		risk := arm.RiskIndex
		if strongest > risk {
			risk = strongest
		}
		updated := evidenceArm(arm.Module, arm.ModuleID, req, risk, arm.Signals, append(append([]string{}, arm.Evidence...), lines...), generatedAt)
		updated.Verdict = "Aynı zincir üstü holder cüzdanı, Koschei'nin saklanan gözlem penceresinde birden fazla tokenda baskın owner olarak doğrulandı."
		updated.Recommendation = "Bu cüzdanın diğer tokenlardaki yoğunlaşma ve likidite etkisini birlikte inceleyin; ilişki kimlik veya niyet kanıtı değildir."
		arms = append(arms, updated)
	}
	if !graphFound {
		signals := map[string]any{
			"real_onchain_evidence":            true,
			"stored_scan_evidence":             true,
			"repeat_dominant_holder":           true,
			"repeat_dominant_holders":          evidence,
			"repeat_dominant_owner_wallets":    owners,
			"repeat_dominant_observation_days": RepeatDominantObservationDays,
			"repeat_dominant_risk_weight":      strongest,
		}
		graph := evidenceArm("Intelligence Graph", ModuleIntelligenceGraph, req, strongest, signals, lines, generatedAt)
		graph.Verdict = "Aynı zincir üstü holder cüzdanı, Koschei'nin saklanan gözlem penceresinde birden fazla tokenda baskın owner olarak doğrulandı."
		graph.Recommendation = "Bu cüzdanın diğer tokenlardaki yoğunlaşma ve likidite etkisini birlikte inceleyin; ilişki kimlik veya niyet kanıtı değildir."
		arms = append(arms, graph)
	}

	sort.SliceStable(arms, func(i, j int) bool {
		return arms[i].ModuleID < arms[j].ModuleID
	})
	finalArm := buildFinalArm(req, arms, generatedAt)
	arms = append(arms, finalArm)
	final := finalVerdictFromArm(finalArm)
	analysis.Arms = arms
	analysis.Final = final
	if analysis.Bundle.Metadata == nil {
		analysis.Bundle.Metadata = map[string]any{}
	}
	analysis.Bundle.Metadata["arvis_arms"] = arms
	analysis.Bundle.Metadata["repeat_dominant_holders"] = evidence
	analysis.Bundle.Metadata["repeat_dominant_holder_count"] = len(evidence)
	analysis.Bundle.Metadata["final_grade"] = final.Grade
	analysis.Bundle.Metadata["final_risk_index"] = final.RiskIndex
	analysis.Bundle.Metadata["final_risk_level"] = final.RiskLevel
	analysis.Bundle.Metadata["final_recommendation"] = final.Recommendation
	analysis.Bundle.Metadata["verified_arm_count"] = verifiedArvisEvidenceCount(arms)
	analysis.Bundle.CustomerSummary = fmt.Sprintf("ARVIS connected %d repeat-dominant holder observation(s) from the last %d days of stored Koschei scans.", len(evidence), RepeatDominantObservationDays)
	analysis.Bundle.CustomerRecommendation = final.Recommendation
	return analysis
}
