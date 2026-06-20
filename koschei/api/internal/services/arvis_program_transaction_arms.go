package services

import (
	"fmt"
	"math"
)

func buildPumpTransactionArm(req SecurityRadarRequest, tx arvisTransactionEvidence, generatedAt string) SecurityRadarVerdict {
	if !tx.Available || !tx.PumpRelated {
		return unavailableArm("Pump.fun Sybil Radar", ModulePumpSybilRadar, req, generatedAt, "A parsed transaction containing the verified Pump program is required.")
	}
	risk := 12
	if tx.InitializeMint || tx.CreateAccount {
		risk += 10
	}
	if len(tx.Signers) >= 3 {
		risk += 8
	}
	if len(tx.FundingAccounts) >= 2 {
		risk += 14
	}
	if len(tx.FundingAccounts) >= 4 {
		risk += 10
	}
	if tx.InnerInstructionCount >= 12 {
		risk += 6
	}
	signals := transactionArmSignals(tx, ModulePumpSybilRadar)
	signals["pump_related"] = true
	signals["program_relation_verified"] = true
	signals["source_verified_program_event"] = arvisSourceModule(req.Mode) == ModulePumpSybilRadar
	signals["signer_count"] = len(tx.Signers)
	signals["funding_account_count"] = len(tx.FundingAccounts)
	signals["creator_candidate"] = tx.CreatorCandidate
	signals["buyer_cluster_graph_available"] = false
	signals["scope_note"] = "verified Pump program relation; confirmed sybil clusters require multi-transaction buyer and funding graph evidence"
	evidence := []string{
		"The parsed transaction contains the verified Pump program relation.",
		fmt.Sprintf("Observed signers: %d; initialization funding accounts: %d.", len(tx.Signers), len(tx.FundingAccounts)),
		fmt.Sprintf("Mint/account initialization evidence: %t; inner instructions: %d.", tx.InitializeMint || tx.CreateAccount, tx.InnerInstructionCount),
		"ARVIS does not claim confirmed sybil wallets without multi-transaction buyer and funding graph evidence.",
	}
	return evidenceArm("Pump.fun Sybil Radar", ModulePumpSybilRadar, req, risk, signals, evidence, generatedAt)
}

func buildRaydiumTransactionArm(req SecurityRadarRequest, tx arvisTransactionEvidence, generatedAt string) SecurityRadarVerdict {
	if !tx.Available || !tx.RaydiumRelated {
		return unavailableArm("Raydium Pool Guardian", ModuleRaydiumPoolGuardian, req, generatedAt, "A parsed transaction containing the verified Raydium program is required.")
	}
	changed := 0
	for _, delta := range tx.TokenBalanceChanges {
		if math.Abs(delta) > 0 {
			changed++
		}
	}
	risk := 10
	if len(tx.TokenMints) >= 2 {
		risk += 8
	}
	if changed >= 2 {
		risk += 14
	}
	if tx.InnerInstructionCount >= 10 {
		risk += 7
	}
	if tx.WritableCount >= 12 {
		risk += 7
	}
	if tx.FeeLamports >= 1_000_000 {
		risk += 6
	}
	signals := transactionArmSignals(tx, ModuleRaydiumPoolGuardian)
	signals["raydium_related"] = true
	signals["program_relation_verified"] = true
	signals["source_verified_program_event"] = arvisSourceModule(req.Mode) == ModuleRaydiumPoolGuardian
	signals["token_mint_count"] = len(tx.TokenMints)
	signals["changed_token_surfaces"] = changed
	signals["writable_account_count"] = tx.WritableCount
	signals["token_balance_changes"] = tx.TokenBalanceChanges
	signals["scope_note"] = "verified Raydium program relation; historical reserve snapshots are required to confirm liquidity drain"
	evidence := []string{
		"The parsed transaction contains the verified Raydium program relation.",
		fmt.Sprintf("Token mint surfaces: %d; changed token balance surfaces: %d.", len(tx.TokenMints), changed),
		fmt.Sprintf("Writable accounts: %d; inner instructions: %d.", tx.WritableCount, tx.InnerInstructionCount),
		"This verifies pool interaction, not a confirmed liquidity drain; reserve history is required for that conclusion.",
	}
	return evidenceArm("Raydium Pool Guardian", ModuleRaydiumPoolGuardian, req, risk, signals, evidence, generatedAt)
}
