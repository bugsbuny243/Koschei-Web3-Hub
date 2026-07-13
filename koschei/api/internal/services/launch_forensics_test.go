package services

import (
	"context"
	"strings"
	"testing"
)

func TestLaunchOwnerCandidatesExcludeProtocolAndMergeTokenAccounts(t *testing.T) {
	accounts := []HolderRoleAccount{
		{OwnerWallet: "LP", TokenAccount: "LP-ATA", Balance: 1000, Role: "pump_liquidity_vault", ExcludedFromHolderRisk: true},
		{OwnerWallet: "A", TokenAccount: "A1", Balance: 10, Role: "externally_owned_wallet"},
		{OwnerWallet: "A", TokenAccount: "A2", Balance: 5, Role: "externally_owned_wallet"},
		{OwnerWallet: "B", TokenAccount: "B1", Balance: 8, Role: "externally_owned_wallet"},
	}
	got := launchOwnerCandidates(accounts, 20)
	if len(got) != 2 {
		t.Fatalf("candidates=%d", len(got))
	}
	if got[0].OwnerWallet != "A" || len(got[0].TokenAccounts) != 2 || got[0].Balance != 15 {
		t.Fatalf("unexpected aggregate: %#v", got[0])
	}
}

func TestLaunchForensicsPredatingCaptureDegradesHonestly(t *testing.T) {
	roles := HolderRoleAnalysis{Available: true, Accounts: []HolderRoleAccount{{OwnerWallet: "A", TokenAccount: "A1", Balance: 10, Role: "externally_owned_wallet"}}}
	result := AnalyzeLaunchForensics(context.Background(), nil, "", "Mint", "", roles, 0, 0)
	if result.Available {
		t.Fatal("missing history must not be available")
	}
	if !strings.Contains(result.Summary, "Launch window not captured") {
		t.Fatalf("summary=%q", result.Summary)
	}
	if result.StructuralFloor != 0 {
		t.Fatalf("floor=%d", result.StructuralFloor)
	}
}
