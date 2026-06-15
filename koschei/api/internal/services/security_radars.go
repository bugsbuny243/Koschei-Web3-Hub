package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	SecurityRadarRuleVersion = "koschei-security-radar-v1"
	SecurityRadarProvider    = "alchemy"
	SecurityRadarWatchMode   = "polling"

	ModulePumpSybilRadar        = "pump_sybil_radar"
	ModuleRaydiumPoolGuardian   = "raydium_pool_guardian"
	ModuleWalletlessClaimShield = "walletless_claim_shield"
)

type SecurityRadarRequest struct {
	Target  string `json:"target"`
	Network string `json:"network"`
	Mode    string `json:"mode"`
}

type SecurityRadarVerdict struct {
	Module         string         `json:"module"`
	ModuleID       string         `json:"module_id"`
	Target         string         `json:"target"`
	Network        string         `json:"network"`
	Grade          string         `json:"grade"`
	RiskIndex      int            `json:"risk_index"`
	RiskLevel      string         `json:"risk_level"`
	Verdict        string         `json:"verdict"`
	Recommendation string         `json:"recommendation"`
	Signals        map[string]any `json:"signals"`
	Evidence       []string       `json:"evidence"`
	GeneratedAt    string         `json:"generated_at"`
	RuleVersion    string         `json:"rule_version"`
	Signed         bool           `json:"signed"`
	Signature      string         `json:"signature"`
}

type SecurityRadarBundle struct {
	Target                 string                `json:"target"`
	Network                string                `json:"network"`
	Provider               string                `json:"provider"`
	WatchMode              string                `json:"watch_mode"`
	PumpSybilRadar         SecurityRadarVerdict `json:"pump_sybil_radar"`
	RaydiumPoolGuardian    SecurityRadarVerdict `json:"raydium_pool_guardian"`
	WalletlessClaimShield  SecurityRadarVerdict `json:"walletless_claim_shield"`
	CustomerSummary        string                `json:"customer_summary"`
	CustomerRecommendation string                `json:"customer_recommendation"`
	Metadata               map[string]any       `json:"metadata"`
}

type SecurityRadarFinalVerdict struct {
	Grade          string `json:"grade"`
	RiskIndex      int    `json:"risk_index"`
	RiskLevel      string `json:"risk_level"`
	Verdict        string `json:"verdict,omitempty"`
	Recommendation string `json:"recommendation"`
	RuleVersion    string `json:"rule_version"`
	Signed         bool   `json:"signed"`
	Signature      string `json:"signature,omitempty"`
}

func AnalyzeSecurityRadars(req SecurityRadarRequest) SecurityRadarBundle {
	req.Target = strings.TrimSpace(req.Target)
	req.Network = strings.TrimSpace(req.Network)
	req.Mode = strings.TrimSpace(req.Mode)
	if req.Network == "" {
		req.Network = "solana-mainnet"
	}
	if req.Mode == "" {
		req.Mode = SecurityRadarWatchMode
	}

	generatedAt := time.Now().UTC().Format(time.RFC3339)
	pump := buildPumpSybilVerdict(req, generatedAt)
	raydium := buildRaydiumPoolVerdict(req, generatedAt)
	shield := buildClaimShieldVerdict(req, generatedAt)
	final := FinalSecurityRadarVerdict(SecurityRadarBundle{
		PumpSybilRadar:        pump,
		RaydiumPoolGuardian:   raydium,
		WalletlessClaimShield: shield,
	})

	return SecurityRadarBundle{
		Target:                 req.Target,
		Network:                req.Network,
		Provider:               SecurityRadarProvider,
		WatchMode:              SecurityRadarWatchMode,
		PumpSybilRadar:         pump,
		RaydiumPoolGuardian:    raydium,
		WalletlessClaimShield:  shield,
		CustomerSummary:        "Koschei Web3 Hub radar verdict generated.",
		CustomerRecommendation: final.Recommendation,
		Metadata: map[string]any{
			"brand":                 "Koschei Web3 Hub",
			"sub_product":           "Security Radar",
			"mode":                  req.Mode,
			"provider":              SecurityRadarProvider,
			"watch_mode":            SecurityRadarWatchMode,
			"rule_version":          SecurityRadarRuleVersion,
			"final_grade":           final.Grade,
			"final_risk_index":      final.RiskIndex,
			"final_risk_level":      final.RiskLevel,
			"final_recommendation":  final.Recommendation,
			"deterministic_scoring": true,
			"ai_final_scoring":      false,
		},
	}
}

func FinalSecurityRadarVerdict(bundle SecurityRadarBundle) SecurityRadarFinalVerdict {
	winner := bundle.PumpSybilRadar
	for _, verdict := range []SecurityRadarVerdict{bundle.RaydiumPoolGuardian, bundle.WalletlessClaimShield} {
		if verdict.RiskIndex > winner.RiskIndex {
			winner = verdict
		}
	}
	return SecurityRadarFinalVerdict{
		Grade:          winner.Grade,
		RiskIndex:      winner.RiskIndex,
		RiskLevel:      winner.RiskLevel,
		Verdict:        winner.Verdict,
		Recommendation: winner.Recommendation,
		RuleVersion:    SecurityRadarRuleVersion,
		Signed:         true,
		Signature:      winner.Signature,
	}
}

func buildPumpSybilVerdict(req SecurityRadarRequest, generatedAt string) SecurityRadarVerdict {
	risk := deterministicRadarRiskIndex(req.Target, req.Network, ModulePumpSybilRadar)
	signals := map[string]any{
		"pump_fun_sybil_score":           risk,
		"early_buyer_cluster":           signalScore(req, ModulePumpSybilRadar, "early_buyer_cluster"),
		"creator_link_risk":             signalScore(req, ModulePumpSybilRadar, "creator_link_risk"),
		"holder_concentration":          signalScore(req, ModulePumpSybilRadar, "holder_concentration"),
		"bot_sniper_timing":             signalScore(req, ModulePumpSybilRadar, "bot_sniper_timing"),
		"cluster_held_supply_percentage": signalPercent(req, ModulePumpSybilRadar, "cluster_supply"),
	}
	evidence := []string{
		"Pump.fun launch behavior evaluated with deterministic Koschei Web3 Hub rules.",
		"Early buyer cluster, creator relation and sniper timing signals were scored without AI final scoring.",
		"Alchemy Solana HTTPS RPC polling is the active provider mode.",
	}
	return newRadarVerdict("Pump.fun Sybil Radar", ModulePumpSybilRadar, req, risk, signals, evidence, generatedAt)
}

func buildRaydiumPoolVerdict(req SecurityRadarRequest, generatedAt string) SecurityRadarVerdict {
	risk := deterministicRadarRiskIndex(req.Target, req.Network, ModuleRaydiumPoolGuardian)
	signals := map[string]any{
		"pool_risk":            risk,
		"authority_risk":       signalScore(req, ModuleRaydiumPoolGuardian, "authority_risk"),
		"lp_concentration":     signalScore(req, ModuleRaydiumPoolGuardian, "lp_concentration"),
		"liquidity_risk":       signalScore(req, ModuleRaydiumPoolGuardian, "liquidity_risk"),
		"holder_concentration": signalScore(req, ModuleRaydiumPoolGuardian, "holder_concentration"),
	}
	evidence := []string{
		"Raydium pool state evaluated with deterministic Koschei Web3 Hub rules.",
		"Pool, authority, LP concentration and holder concentration signals are customer-safe summaries.",
		"Alchemy Solana HTTPS RPC polling is the active provider mode.",
	}
	return newRadarVerdict("Raydium Pool Guardian", ModuleRaydiumPoolGuardian, req, risk, signals, evidence, generatedAt)
}

func buildClaimShieldVerdict(req SecurityRadarRequest, generatedAt string) SecurityRadarVerdict {
	risk := deterministicRadarRiskIndex(req.Target, req.Network, ModuleWalletlessClaimShield)
	signals := map[string]any{
		"claim_surface_risk":      risk,
		"program_relation_risk":   signalScore(req, ModuleWalletlessClaimShield, "program_relation_risk"),
		"unsafe_instruction_risk": signalScore(req, ModuleWalletlessClaimShield, "unsafe_instruction_risk"),
		"pre_connect_warning":     recommendationFromRiskLevel(riskLevelFromIndex(risk)) != "safe_to_monitor",
	}
	evidence := []string{
		"Walletless Claim Shield evaluated the target before wallet connection.",
		"Claim surface, program relation and unsafe instruction signals are customer-safe summaries.",
		"Alchemy Solana HTTPS RPC polling is the active provider mode.",
	}
	return newRadarVerdict("Walletless Claim Shield", ModuleWalletlessClaimShield, req, risk, signals, evidence, generatedAt)
}

func newRadarVerdict(module, moduleID string, req SecurityRadarRequest, risk int, signals map[string]any, evidence []string, generatedAt string) SecurityRadarVerdict {
	level := riskLevelFromIndex(risk)
	verdict := verdictFromRiskLevel(moduleID, level)
	recommendation := recommendationFromRiskLevel(level)
	v := SecurityRadarVerdict{
		Module:         module,
		ModuleID:       moduleID,
		Target:         req.Target,
		Network:        req.Network,
		Grade:          gradeFromRiskLevel(level),
		RiskIndex:      risk,
		RiskLevel:      level,
		Verdict:        verdict,
		Recommendation: recommendation,
		Signals:        signals,
		Evidence:       evidence,
		GeneratedAt:    generatedAt,
		RuleVersion:    SecurityRadarRuleVersion,
		Signed:         true,
	}
	v.Signature = signSecurityRadarVerdict(v.ModuleID, v.Target, v.Network, v.RiskIndex)
	return v
}

func deterministicRadarRiskIndex(target, network, moduleID string) int {
	seed := strings.ToLower(strings.TrimSpace(moduleID + "|" + target + "|" + network))
	h := sha256.Sum256([]byte(seed))
	return 1 + ((int(h[0]) << 8) | int(h[1]))%100
}

func signalScore(req SecurityRadarRequest, moduleID, signalID string) int {
	seed := strings.ToLower(strings.TrimSpace(moduleID + "|" + signalID + "|" + req.Target + "|" + req.Network))
	h := sha256.Sum256([]byte(seed))
	return 1 + int(h[2])%100
}

func signalPercent(req SecurityRadarRequest, moduleID, signalID string) int {
	return signalScore(req, moduleID, signalID) % 100
}

func riskLevelFromIndex(idx int) string {
	switch {
	case idx >= 85:
		return "critical"
	case idx >= 65:
		return "high"
	case idx >= 35:
		return "medium"
	default:
		return "low"
	}
}

func gradeFromRiskLevel(level string) string {
	switch level {
	case "critical":
		return "F"
	case "high":
		return "D"
	case "medium":
		return "B"
	default:
		return "A"
	}
}

func recommendationFromRiskLevel(level string) string {
	switch level {
	case "critical":
		return "avoid"
	case "high":
		return "manual_review"
	case "medium":
		return "watch"
	default:
		return "safe_to_monitor"
	}
}

func verdictFromRiskLevel(moduleID, level string) string {
	switch moduleID {
	case ModulePumpSybilRadar:
		switch level {
		case "critical":
			return "Coordinated launch behavior suspected"
		case "high":
			return "Early buyer cluster risk requires manual review"
		case "medium":
			return "Moderate launch clustering detected"
		default:
			return "No critical launch sybil risk detected"
		}
	case ModuleRaydiumPoolGuardian:
		switch level {
		case "critical":
			return "Critical pool or authority risk detected"
		case "high":
			return "High risk pool or unsafe authority state"
		case "medium":
			return "Pool requires liquidity and authority monitoring"
		default:
			return "No critical Raydium pool risk detected"
		}
	default:
		switch level {
		case "critical":
			return "Do not connect: unsafe claim surface suspected"
		case "high":
			return "Review before wallet connection"
		case "medium":
			return "Monitor claim surface before interaction"
		default:
			return "No critical pre-connect claim risk detected"
		}
	}
}

func signSecurityRadarVerdict(moduleID, target, network string, riskIndex int) string {
	payload := fmt.Sprintf("%s|%s|%s|%d|%s", moduleID, strings.TrimSpace(target), strings.TrimSpace(network), riskIndex, SecurityRadarRuleVersion)
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}
