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

func TestHolderScanRPCBudgetReserveUpToNeverExceedsLimit(t *testing.T) {
	budget := newHolderScanRPCBudget(5)
	if got := budget.ReserveUpTo(3); got != 3 {
		t.Fatalf("first grant=%d", got)
	}
	if got := budget.ReserveUpTo(10); got != 2 {
		t.Fatalf("bounded grant=%d", got)
	}
	if got := budget.ReserveUpTo(1); got != 0 {
		t.Fatalf("exhausted grant=%d", got)
	}
	if budget.Used() != 5 || budget.Remaining() != 0 {
		t.Fatalf("used=%d remaining=%d", budget.Used(), budget.Remaining())
	}
}

func TestTraceLaunchFundingMarksDirectCreatorWithoutRPC(t *testing.T) {
	profile := LaunchActorProfile{OwnerWallet: "CreatorWallet", Evidence: []string{}}
	cfg := loadLaunchForensicsConfig()
	budget := newHolderScanRPCBudget(10)
	traceLaunchFunding(context.Background(), "", "CreatorWallet", "", &profile, cfg, budget)
	if !profile.CreatorLinked || profile.FundingStatus != "creator_linked" || profile.FundingHops != 0 {
		t.Fatalf("direct creator link not preserved: %#v", profile)
	}
	if budget.Used() != 0 {
		t.Fatalf("direct link should not spend RPC budget: %d", budget.Used())
	}
}

func TestLaunchATAFairShareBudgetsDistributeWithoutStarvation(t *testing.T) {
	got := launchATAFairShareBudgets(100, 18)
	if len(got) != 18 {
		t.Fatalf("owners=%d want=18", len(got))
	}
	total := 0
	minimum, maximum := got[0], got[0]
	for index, quota := range got {
		if quota <= 0 {
			t.Fatalf("owner %d received no quota: %v", index, got)
		}
		total += quota
		if quota < minimum {
			minimum = quota
		}
		if quota > maximum {
			maximum = quota
		}
	}
	if total != 100 {
		t.Fatalf("allocated=%d want=100 quotas=%v", total, got)
	}
	if minimum != 5 || maximum != 6 || maximum-minimum > 1 {
		t.Fatalf("unexpected fair-share spread min=%d max=%d quotas=%v", minimum, maximum, got)
	}
	for index, quota := range got {
		want := 5
		if index < 10 {
			want = 6
		}
		if quota != want {
			t.Fatalf("quota[%d]=%d want=%d all=%v", index, quota, want, got)
		}
	}
}

func TestLaunchATAFairShareBudgetsNeverExceedSmallBudget(t *testing.T) {
	got := launchATAFairShareBudgets(5, 8)
	total := 0
	for _, quota := range got {
		total += quota
	}
	if total != 5 {
		t.Fatalf("allocated=%d want=5 quotas=%v", total, got)
	}
	for index, quota := range got {
		want := 0
		if index < 5 {
			want = 1
		}
		if quota != want {
			t.Fatalf("quota[%d]=%d want=%d all=%v", index, quota, want, got)
		}
	}
}

func TestLaunchATAFairShareBudgetLeavesParsedTransactionCapacity(t *testing.T) {
	budget := newHolderScanRPCBudget(5)
	if !budget.Reserve(1) {
		t.Fatal("first signature page reservation failed")
	}
	if !budget.Reserve(1) {
		t.Fatal("second signature page reservation failed")
	}
	if !budget.Reserve(1) {
		t.Fatal("third signature page reservation failed")
	}
	if budget.Remaining() != launchATAMinTransactionReserve {
		t.Fatalf("remaining=%d want=%d", budget.Remaining(), launchATAMinTransactionReserve)
	}
	if got := budget.ReserveUpTo(launchTransactionBatchSize); got != launchATAMinTransactionReserve {
		t.Fatalf("parsed transaction grant=%d want=%d", got, launchATAMinTransactionReserve)
	}
	if budget.Used() != 5 {
		t.Fatalf("used=%d want=5", budget.Used())
	}
}
