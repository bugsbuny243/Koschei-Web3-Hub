package services

import (
	"strings"
	"testing"
)

func TestLPControlPoolArmExposesPermanentFungibleLPLockEvidence(t *testing.T) {
	lp := LPControlEvidence{
		Available: true,
		Status: LPControlVerifiedPermanentLocked,
		ReasonCode: "raydium_burn_and_earn_permanent_lock_observed",
		PoolAddress: "Pool111",
		PoolProgram: "LockAwarePoolProgram",
		PoolType: "raydium_cpmm",
		ControlModel: "lp_token",
		PositionModel: "fungible_lp_token",
		LPMint: "LPMint111",
		LPSupply: 1000,
		LockedLPAmount: 750,
		LockedLPSharePct: 75,
		LockedLPTokenAccounts: []string{"LockedA", "LockedB"},
		LockerAccount: "BurnAndEarnPDA",
		LockerProgram: "LockrWmn6K5twhz3y9w1dQERbmgSaRkfnTeTKbpofwE",
		ReadSlot: 777,
		LargestLPHolders: []LPHolderEvidence{},
		LiquidityMovements: []LiquidityMovementEvidence{},
		EvidenceKeys: []string{"raydium_permanent_locked_lp_share:75.0000@777"},
		Limitations: []string{},
	}
	arm := lpControlPoolArm(SecurityRadarRequest{}, lp, "2026-07-22T00:00:00Z")
	if arm.Signals["evidence_status"] != "verified" || arm.Signals["lp_lock_status"] != LPControlVerifiedPermanentLocked {
		t.Fatalf("arm did not expose verified permanent lock status: %#v", arm.Signals)
	}
	if arm.Signals["locked_lp_amount"] != float64(750) || arm.Signals["locked_lp_share_pct"] != float64(75) {
		t.Fatalf("arm lost locked LP amount/share: %#v", arm.Signals)
	}
	accounts, ok := arm.Signals["locked_lp_token_accounts"].([]string)
	if !ok || strings.Join(accounts, ",") != "LockedA,LockedB" {
		t.Fatalf("arm locked token accounts = %#v", arm.Signals["locked_lp_token_accounts"])
	}
	joined := strings.Join(arm.Evidence, "\n")
	if !strings.Contains(joined, "Pinned Raydium Burn & Earn custody was resolved") || !strings.Contains(joined, "75.0000%") {
		t.Fatalf("human evidence omitted the permanent lock proof: %s", joined)
	}
}
