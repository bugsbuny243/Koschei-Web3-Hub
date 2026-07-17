package services

import (
	"fmt"
	"strings"
	"time"
)

// ApplyLPControlEvidenceToAnalysis replaces only the pool/liquidity evidence
// view. It does not recalculate or cap a grade and cannot alter signing.
func ApplyLPControlEvidenceToAnalysis(analysis ArvisAnalysis, req SecurityRadarRequest, lp LPControlEvidence) ArvisAnalysis {
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	poolArm := lpControlPoolArm(req, lp, generatedAt)
	liquidityArm := lpControlLiquidityArm(req, lp, generatedAt)
	replacements := map[string]SecurityRadarVerdict{
		ModuleRaydiumPoolGuardian: poolArm,
		ModuleLiquidityMovement:   liquidityArm,
	}
	arms := ArvisArmsFromBundle(analysis.Bundle)
	if len(arms) == 0 {
		arms = append([]SecurityRadarVerdict{}, analysis.Arms...)
	}
	updated := make([]SecurityRadarVerdict, 0, len(arms))
	seen := map[string]bool{}
	for _, arm := range arms {
		if replacement, ok := replacements[arm.ModuleID]; ok {
			updated = append(updated, replacement)
			seen[arm.ModuleID] = true
			continue
		}
		updated = append(updated, arm)
	}
	for moduleID, replacement := range replacements {
		if !seen[moduleID] {
			updated = append(updated, replacement)
		}
	}
	analysis.Arms = updated
	analysis.Final = arvisCompatibilityFinal()
	if analysis.Bundle.Metadata == nil {
		analysis.Bundle.Metadata = map[string]any{}
	}
	analysis.Bundle.Metadata["arvis_arms"] = updated
	analysis.Bundle.Metadata["lp_control"] = lp
	analysis.Bundle.Metadata["raydium_pool_guardian"] = poolArm
	analysis.Bundle.Metadata["liquidity_movement"] = liquidityArm
	analysis.Bundle.Metadata["verified_arm_count"] = verifiedArvisEvidenceCount(updated)
	analysis.Bundle.Metadata["runtime_arm_count"] = verifiedArvisEvidenceCount(updated)
	return ApplyArvisInvestigationCoverage(analysis)
}

func lpControlPoolArm(req SecurityRadarRequest, lp LPControlEvidence, generatedAt string) SecurityRadarVerdict {
	if lp.Status == LPControlNotApplicable {
		return notApplicableArm("Raydium Pool Guardian", ModuleRaydiumPoolGuardian, req, generatedAt, lpControlReason(lp), lp.ReasonCode)
	}
	if lp.Status == LPControlSourceUnavailable || strings.TrimSpace(lp.PoolAddress) == "" {
		arm := evidencePendingArm("Raydium Pool Guardian", ModuleRaydiumPoolGuardian, req, generatedAt, lpControlReason(lp), firstRadarValue(lp.ReasonCode, "pool_control_unavailable"))
		arm.Signals["execution_status"] = ArvisExecutionSourceUnavailable
		arm.Signals["lp_control"] = lp
		return arm
	}
	status := "observed"
	verified := lp.Status == LPControlVerifiedBurned || lp.Status == LPControlVerifiedLocked
	if verified {
		status = "verified"
	}
	signals := map[string]any{
		"module_id": ModuleRaydiumPoolGuardian,
		"execution_status": ArvisExecutionCompleted,
		"collector_attempted": true,
		"applicable": true,
		"finding_observed": true,
		"real_onchain_evidence": true,
		"evidence_status": status,
		"pool_address": lp.PoolAddress,
		"pool_program": lp.PoolProgram,
		"pool_type": lp.PoolType,
		"lp_mint": lp.LPMint,
		"token_vault": lp.TokenVault,
		"quote_vault": lp.QuoteVault,
		"read_slot": lp.ReadSlot,
		"token_reserve": lp.TokenReserve,
		"quote_reserve": lp.QuoteReserve,
		"reserve_liquidity_usd": lp.ReserveLiquidityUSD,
		"lp_supply": lp.LPSupply,
		"lp_lock_status": lp.Status,
		"burned_share_pct": lp.BurnedSharePct,
		"creator_lp_share_pct": lp.CreatorLPSharePct,
		"locker_program": lp.LockerProgram,
		"locker_account": lp.LockerAccount,
		"locked_until": lp.LockedUntil,
		"evidence_keys": append([]string{}, lp.EvidenceKeys...),
		"numeric_score_disabled": true,
		"grade_effect": "none_at_arm_layer",
	}
	evidence := []string{
		fmt.Sprintf("Raydium pool %s is owned by %s and was read at slot %d.", lp.PoolAddress, lp.PoolProgram, lp.ReadSlot),
		fmt.Sprintf("LP mint %s supply %.8f; token reserve %.8f; quote reserve %.8f.", lp.LPMint, lp.LPSupply, lp.TokenReserve, lp.QuoteReserve),
		fmt.Sprintf("LP control status %s; burn-address share %.4f%%; creator-owned share %.4f%%.", lp.Status, lp.BurnedSharePct, lp.CreatorLPSharePct),
	}
	for _, key := range lp.EvidenceKeys {
		evidence = append(evidence, "Evidence key: "+key)
	}
	for _, limitation := range lp.Limitations {
		evidence = append(evidence, "Limitation: "+limitation)
	}
	arm := evidenceArm("Raydium Pool Guardian", ModuleRaydiumPoolGuardian, req, 0, signals, evidence, generatedAt)
	arm.Verdict = "Pool reserves and LP control surfaces were collected directly from Solana accounts; the status describes observed capability, not intent."
	arm.Recommendation = "none"
	return arm
}

func lpControlLiquidityArm(req SecurityRadarRequest, lp LPControlEvidence, generatedAt string) SecurityRadarVerdict {
	if lp.Status == LPControlNotApplicable {
		return notApplicableArm("Liquidity Movement", ModuleLiquidityMovement, req, generatedAt, lpControlReason(lp), lp.ReasonCode)
	}
	if lp.Status == LPControlSourceUnavailable || strings.TrimSpace(lp.PoolAddress) == "" {
		arm := evidencePendingArm("Liquidity Movement", ModuleLiquidityMovement, req, generatedAt, lpControlReason(lp), firstRadarValue(lp.ReasonCode, "reserve_collection_unavailable"))
		arm.Signals["execution_status"] = ArvisExecutionSourceUnavailable
		arm.Signals["lp_control"] = lp
		return arm
	}
	arm := lpControlPoolArm(req, lp, generatedAt)
	arm.Module = "Liquidity Movement"
	arm.ModuleID = ModuleLiquidityMovement
	arm.Signals["module_id"] = ModuleLiquidityMovement
	arm.Signals["liquidity_movement_transaction_verified"] = false
	arm.Signals["reserve_snapshot_verified"] = lp.ReadSlot > 0 && lp.TokenVault != "" && lp.QuoteVault != ""
	arm.Verdict = "Pool reserve balances were read directly at the reported slot. This is a reserve snapshot, not proof that liquidity was added or removed by a specific wallet."
	return arm
}

func lpControlReason(lp LPControlEvidence) string {
	if len(lp.Limitations) > 0 && strings.TrimSpace(lp.Limitations[0]) != "" {
		return lp.Limitations[0]
	}
	if lp.ReasonCode != "" {
		return lp.ReasonCode
	}
	return "LP control evidence was not available for this scan."
}
