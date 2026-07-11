package services

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ModuleIntelligenceGraph      = "intelligence_graph"
	ModuleMEVShield              = "mev_shield"
	ModuleTokenAuthorityScanner  = "token_authority_scanner"
	ModuleHolderConcentration    = "holder_concentration"
	ModuleLiquidityMovement      = "liquidity_movement"
	ModuleCreatorLinkAnalysis    = "creator_link_analysis"
	ModuleFundingClusterDetector = "funding_cluster_detector"
	ModuleSniperTimingDetector   = "sniper_timing_detector"
	ModuleClaimSurfaceRisk       = "claim_surface_risk"
	ModuleProgramRelationScan    = "program_relation_scan"
	ModuleFinalVerdictEngine     = "final_verdict_engine"
)

type ArvisAnalysis struct {
	Bundle SecurityRadarBundle       `json:"bundle"`
	Arms   []SecurityRadarVerdict    `json:"arms"`
	Final  SecurityRadarFinalVerdict `json:"final_verdict"`
}

func AnalyzeArvisRadars(req SecurityRadarRequest) ArvisAnalysis {
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
	profile := collectRadarEvidence(req)
	sourceModule := arvisSourceModule(req.Mode)

	pump := unavailableArm("Pump.fun Sybil Radar", ModulePumpSybilRadar, req, generatedAt, "Verified Pump program or parsed launch transaction evidence is required.")
	if sourceModule == ModulePumpSybilRadar && profile.LiveRPC && (profile.RecentSignatureCount > 0 || profile.LargestAccounts > 0) {
		pump = buildPumpSybilVerdict(req, profile, generatedAt)
		pump.Signals["source_module"] = sourceModule
		pump.Signals["source_verified_program_event"] = true
		pump.Evidence = append(pump.Evidence, "Target was resolved from a verified Pump program stream event.")
	}

	raydium := unavailableArm("Raydium Pool Guardian", ModuleRaydiumPoolGuardian, req, generatedAt, "Verified Raydium program or parsed pool transaction evidence is required.")
	if sourceModule == ModuleRaydiumPoolGuardian && profile.LiveRPC && profile.AccountExists {
		raydium = buildRaydiumPoolVerdict(req, profile, generatedAt)
		raydium.Signals["source_module"] = sourceModule
		raydium.Signals["source_verified_program_event"] = true
		raydium.Evidence = append(raydium.Evidence, "Target was resolved from a verified Raydium program stream event.")
	}

	claimShield := unavailableArm("Walletless Claim Shield", ModuleWalletlessClaimShield, req, generatedAt, "Claim instruction and web-surface collection is not available for this target.")
	graph := buildIntelligenceGraphArm(req, profile, generatedAt)
	mev := unavailableArm("MEV Shield", ModuleMEVShield, req, generatedAt, "A signed transaction, route and pool context are required for MEV analysis.")
	authority := buildAuthorityArm(req, profile, generatedAt)
	holders := buildHolderArm(req, profile, generatedAt)
	liquidity := unavailableArm("Liquidity Movement", ModuleLiquidityMovement, req, generatedAt, "Historical LP balance and pool reserve snapshots are required.")
	creator := unavailableArm("Creator Link Analysis", ModuleCreatorLinkAnalysis, req, generatedAt, "Creator identity and parsed launch transaction evidence are required.")
	funding := unavailableArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, generatedAt, "Parsed funding transactions and counterparty graph evidence are required.")
	sniper := buildSniperTimingArm(req, profile, generatedAt)
	claimSurface := unavailableArm("Claim Surface Risk", ModuleClaimSurfaceRisk, req, generatedAt, "URL fetch, instruction simulation and domain evidence are required.")
	program := buildProgramRelationArm(req, profile, generatedAt)

	arms := []SecurityRadarVerdict{pump, raydium, claimShield, graph, mev, authority, holders, liquidity, creator, funding, sniper, claimSurface, program}
	finalArm := buildFinalArm(req, arms, generatedAt)
	arms = append(arms, finalArm)
	final := finalVerdictFromArm(finalArm)
	verified := verifiedArvisEvidenceCount(arms)

	summary := SecurityRadarInsufficientEvidenceMessage
	if final.Signed {
		summary = fmt.Sprintf("ARVIS verified %d of 13 evidence arms for this target and produced one evidence-backed verdict.", verified)
	}

	bundle := SecurityRadarBundle{
		Target: req.Target, Network: req.Network, Provider: SecurityRadarProvider, WatchMode: req.Mode,
		PumpSybilRadar: pump, RaydiumPoolGuardian: raydium, WalletlessClaimShield: claimShield,
		CustomerSummary: summary, CustomerRecommendation: final.Recommendation,
		Metadata: map[string]any{
			"brand": "KOSCHEİ WEB3", "sub_product": "ARVIS", "mode": req.Mode,
			"provider": SecurityRadarProvider, "watch_mode": req.Mode, "rule_version": SecurityRadarRuleVersion,
			"architecture_arm_count": 14, "evidence_arm_count": 13, "verified_arm_count": verified,
			"runtime_arm_count": verified, "arvis_arms": arms, "source_module": sourceModule,
			"final_grade": final.Grade, "final_risk_index": final.RiskIndex,
			"final_risk_level": final.RiskLevel, "final_recommendation": final.Recommendation,
			"data_quality": profile.DataQuality, "evidence_status": profile.EvidenceStatus,
		},
	}
	return ArvisAnalysis{Bundle: bundle, Arms: arms, Final: final}
}

func ArvisArmsFromBundle(bundle SecurityRadarBundle) []SecurityRadarVerdict {
	if bundle.Metadata == nil {
		return nil
	}
	switch raw := bundle.Metadata["arvis_arms"].(type) {
	case []SecurityRadarVerdict:
		return raw
	case []any:
		out := make([]SecurityRadarVerdict, 0, len(raw))
		for _, item := range raw {
			if verdict, ok := item.(SecurityRadarVerdict); ok {
				out = append(out, verdict)
			}
		}
		return out
	default:
		return nil
	}
}

func ArvisFinalFromBundle(bundle SecurityRadarBundle) SecurityRadarFinalVerdict {
	for _, arm := range ArvisArmsFromBundle(bundle) {
		if arm.ModuleID == ModuleFinalVerdictEngine {
			return finalVerdictFromArm(arm)
		}
	}
	return EvidenceBackedFinalSecurityRadarVerdict(bundle)
}

func arvisSourceModule(mode string) string {
	mode = strings.TrimSpace(mode)
	const prefix = "live_stream:"
	if !strings.HasPrefix(mode, prefix) {
		return ""
	}
	moduleID := strings.TrimSpace(strings.TrimPrefix(mode, prefix))
	switch moduleID {
	case ModulePumpSybilRadar, ModuleRaydiumPoolGuardian:
		return moduleID
	default:
		return ""
	}
}

func buildIntelligenceGraphArm(req SecurityRadarRequest, p radarEvidenceProfile, generatedAt string) SecurityRadarVerdict {
	if !p.LiveRPC || !p.AccountExists || (p.AccountOwner == "" && p.LatestSignature == "") {
		return unavailableArm("Intelligence Graph", ModuleIntelligenceGraph, req, generatedAt, "Account owner and transaction relation evidence are required.")
	}
	risk := 8 + concentrationRisk(p.LargestHolderPct, p.Top10HolderPct)/2
	if p.FailedSignatureCount >= 10 {
		risk += 8
	}
	s := armSignals(req, p, ModuleIntelligenceGraph)
	s["account_owner"] = p.AccountOwner
	s["latest_signature"] = p.LatestSignature
	s["largest_accounts"] = p.LargestAccounts
	e := []string{fmt.Sprintf("Account owner observed: %s.", firstRadarValue(p.AccountOwner, "unknown")), fmt.Sprintf("Latest signature: %s at slot %d.", firstRadarValue(p.LatestSignature, "none"), p.LatestSlot), fmt.Sprintf("Holder relation inputs: %d largest accounts.", p.LargestAccounts)}
	return evidenceArm("Intelligence Graph", ModuleIntelligenceGraph, req, risk, s, e, generatedAt)
}

func buildAuthorityArm(req SecurityRadarRequest, p radarEvidenceProfile, generatedAt string) SecurityRadarVerdict {
	if !p.LiveRPC || !p.IsTokenMint || !p.AccountExists {
		return unavailableArm("Token Authority Scanner", ModuleTokenAuthorityScanner, req, generatedAt, "A parsed token mint account is required.")
	}
	risk := 5
	if p.MintAuthorityPresent {
		risk += 38
	}
	if p.FreezeAuthorityPresent {
		risk += 38
	}
	s := armSignals(req, p, ModuleTokenAuthorityScanner)
	s["mint_authority_present"] = p.MintAuthorityPresent
	s["freeze_authority_present"] = p.FreezeAuthorityPresent
	s["account_owner"] = p.AccountOwner
	e := []string{fmt.Sprintf("Mint authority present: %t.", p.MintAuthorityPresent), fmt.Sprintf("Freeze authority present: %t.", p.FreezeAuthorityPresent), fmt.Sprintf("Mint account owner: %s.", firstRadarValue(p.AccountOwner, "unknown"))}
	return evidenceArm("Token Authority Scanner", ModuleTokenAuthorityScanner, req, risk, s, e, generatedAt)
}

func buildHolderArm(req SecurityRadarRequest, p radarEvidenceProfile, generatedAt string) SecurityRadarVerdict {
	if !p.LiveRPC || !p.IsTokenMint || p.LargestAccounts == 0 {
		return unavailableArm("Holder Concentration", ModuleHolderConcentration, req, generatedAt, "Token largest-account evidence is required.")
	}
	if p.HolderRoles.BlockingEvidenceGap {
		v := unavailableArm("Holder Concentration", ModuleHolderConcentration, req, generatedAt, "Dominant token-account role is unresolved; raw concentration cannot be converted into a wallet-control verdict.")
		v.Signals["blocking_final_verdict"] = true
		v.Signals["holder_role_analysis"] = p.HolderRoles
		v.Signals["raw_largest_holder_percentage"] = p.RawLargestHolderPct
		v.Signals["raw_top_10_holder_percentage"] = p.RawTop10HolderPct
		return v
	}
	risk := 5 + concentrationRisk(p.LargestHolderPct, p.Top10HolderPct)
	s := armSignals(req, p, ModuleHolderConcentration)
	s["largest_holder_percentage"] = p.LargestHolderPct
	s["top_10_holder_percentage"] = p.Top10HolderPct
	s["raw_largest_holder_percentage"] = p.RawLargestHolderPct
	s["raw_top_10_holder_percentage"] = p.RawTop10HolderPct
	s["holder_role_adjusted"] = p.HolderRoles.RoleAdjusted
	s["holder_role_analysis"] = p.HolderRoles
	s["largest_accounts"] = p.LargestAccounts
	s["token_supply"] = p.TokenSupply
	basis := "raw total supply"
	if p.HolderRoles.RoleAdjusted {
		basis = "role-adjusted circulating holder supply"
	}
	e := []string{
		fmt.Sprintf("Holder concentration basis: %s.", basis),
		fmt.Sprintf("Risk-bearing largest holder controls %d%%; Top 10 control %d%%.", p.LargestHolderPct, p.Top10HolderPct),
		fmt.Sprintf("Raw token-account concentration before role classification: Top 1=%d%% Top 10=%d%%.", p.RawLargestHolderPct, p.RawTop10HolderPct),
		fmt.Sprintf("Protocol-controlled inventory=%.4f%%; burn sinks=%.4f%%; unresolved=%.4f%%.", p.HolderRoles.ProtocolControlledPercentage, p.HolderRoles.BurnPercentage, p.HolderRoles.UnresolvedPercentage),
	}
	return evidenceArm("Holder Concentration", ModuleHolderConcentration, req, risk, s, e, generatedAt)
}

func buildSniperTimingArm(req SecurityRadarRequest, p radarEvidenceProfile, generatedAt string) SecurityRadarVerdict {
	if !p.LiveRPC || p.RecentSignatureCount == 0 || p.SignatureWindowSeconds <= 0 {
		return unavailableArm("Sniper Timing Detector", ModuleSniperTimingDetector, req, generatedAt, "Timestamped signature activity is required.")
	}
	risk := 8 + burstRisk(p.RecentSignatureCount, p.SignatureWindowSeconds)
	s := armSignals(req, p, ModuleSniperTimingDetector)
	s["recent_signature_count"] = p.RecentSignatureCount
	s["signature_window_seconds"] = p.SignatureWindowSeconds
	s["failed_signature_count"] = p.FailedSignatureCount
	s["scope_note"] = "activity burst signal; sniper confirmation requires parsed launch transactions"
	e := []string{fmt.Sprintf("Observed %d signatures in a %d second window.", p.RecentSignatureCount, p.SignatureWindowSeconds), "This arm reports timing concentration only; it does not claim confirmed sniper wallets without parsed launch transactions."}
	return evidenceArm("Sniper Timing Detector", ModuleSniperTimingDetector, req, risk, s, e, generatedAt)
}

func buildProgramRelationArm(req SecurityRadarRequest, p radarEvidenceProfile, generatedAt string) SecurityRadarVerdict {
	if !p.LiveRPC || !p.AccountExists || p.AccountOwner == "" {
		return unavailableArm("Program Relation Scan", ModuleProgramRelationScan, req, generatedAt, "Account owner evidence is required.")
	}
	risk := 6
	if p.AccountExecutable {
		risk += 22
	}
	if !p.IsTokenMint {
		risk += 8
	}
	s := armSignals(req, p, ModuleProgramRelationScan)
	s["account_owner"] = p.AccountOwner
	s["account_executable"] = p.AccountExecutable
	s["is_token_mint"] = p.IsTokenMint
	e := []string{fmt.Sprintf("Account owner program: %s.", p.AccountOwner), fmt.Sprintf("Executable account: %t.", p.AccountExecutable), fmt.Sprintf("Confirmed token mint: %t.", p.IsTokenMint)}
	return evidenceArm("Program Relation Scan", ModuleProgramRelationScan, req, risk, s, e, generatedAt)
}

func buildFinalArm(req SecurityRadarRequest, arms []SecurityRadarVerdict, generatedAt string) SecurityRadarVerdict {
	for _, arm := range arms {
		if blocked, _ := arm.Signals["blocking_final_verdict"].(bool); blocked {
			v := unavailableArm("Final Verdict Engine", ModuleFinalVerdictEngine, req, generatedAt, "A dominant holder role is unresolved; Unavailable is not Low and no final token-risk score is issued.")
			v.Signals["blocking_final_verdict"] = true
			v.Signals["blocking_module"] = arm.ModuleID
			return v
		}
	}
	verified := make([]SecurityRadarVerdict, 0, len(arms))
	for _, arm := range arms {
		if !arm.Signed || arm.Signals == nil {
			continue
		}
		if ok, _ := arm.Signals["real_onchain_evidence"].(bool); ok {
			verified = append(verified, arm)
		}
	}
	if len(verified) == 0 {
		return unavailableArm("Final Verdict Engine", ModuleFinalVerdictEngine, req, generatedAt, SecurityRadarInsufficientEvidenceMessage)
	}
	sort.SliceStable(verified, func(i, j int) bool { return verified[i].RiskIndex > verified[j].RiskIndex })
	winner := verified[0]
	s := map[string]any{"real_onchain_evidence": true, "evidence_status": "verified_multi_arm", "verified_arm_count": len(verified), "winning_arm": winner.ModuleID, "score_source": "highest_verified_arvis_arm"}
	if sourceModule := arvisSourceModule(req.Mode); sourceModule != "" {
		s["source_module"] = sourceModule
	}
	e := []string{fmt.Sprintf("Verified evidence arms: %d.", len(verified)), fmt.Sprintf("Highest-risk verified arm: %s (%d/100).", winner.Module, winner.RiskIndex), "Final verdict uses only signed arms with live on-chain evidence."}
	v := evidenceArm("Final Verdict Engine", ModuleFinalVerdictEngine, req, winner.RiskIndex, s, e, generatedAt)
	v.Verdict = winner.Verdict
	v.Recommendation = winner.Recommendation
	return v
}

func armSignals(req SecurityRadarRequest, p radarEvidenceProfile, moduleID string) map[string]any {
	s := baseEvidenceSignals(p)
	s["module_id"] = moduleID
	s["arm_evidence_available"] = true
	if sourceModule := arvisSourceModule(req.Mode); sourceModule != "" {
		s["source_module"] = sourceModule
	}
	return s
}

func evidenceArm(module, moduleID string, req SecurityRadarRequest, risk int, signals map[string]any, evidence []string, generatedAt string) SecurityRadarVerdict {
	risk = clampRisk(risk)
	level := riskLevelFromIndex(risk)
	if signals == nil {
		signals = map[string]any{}
	}
	if sourceModule := arvisSourceModule(req.Mode); sourceModule != "" {
		signals["source_module"] = sourceModule
	}
	v := SecurityRadarVerdict{Module: module, ModuleID: moduleID, Target: req.Target, Network: req.Network, Grade: gradeFromRiskLevel(level), RiskIndex: risk, RiskLevel: level, Verdict: verdictFromRiskLevel(moduleID, level, signals), Recommendation: recommendationFromRiskLevel(level), Signals: signals, Evidence: evidence, GeneratedAt: generatedAt, RuleVersion: SecurityRadarRuleVersion, Signed: true}
	v.Signature = signSecurityRadarVerdict(v.ModuleID, v.Target, v.Network, v.RiskIndex)
	return v
}

func unavailableArm(module, moduleID string, req SecurityRadarRequest, generatedAt, reason string) SecurityRadarVerdict {
	signals := map[string]any{"module_id": moduleID, "real_onchain_evidence": false, "arm_evidence_available": false, "evidence_status": "insufficient_evidence", "score_source": "none"}
	if sourceModule := arvisSourceModule(req.Mode); sourceModule != "" {
		signals["source_module"] = sourceModule
	}
	return SecurityRadarVerdict{Module: module, ModuleID: moduleID, Target: req.Target, Network: req.Network, Grade: "-", RiskIndex: 0, RiskLevel: "unknown", Verdict: SecurityRadarInsufficientEvidenceMessage, Recommendation: "insufficient_evidence", Signals: signals, Evidence: []string{reason}, GeneratedAt: generatedAt, RuleVersion: SecurityRadarRuleVersion, Signed: false}
}

func finalVerdictFromArm(arm SecurityRadarVerdict) SecurityRadarFinalVerdict {
	return SecurityRadarFinalVerdict{Grade: arm.Grade, RiskIndex: arm.RiskIndex, RiskLevel: arm.RiskLevel, Verdict: arm.Verdict, Recommendation: arm.Recommendation, RuleVersion: arm.RuleVersion, Signed: arm.Signed, Signature: arm.Signature}
}
