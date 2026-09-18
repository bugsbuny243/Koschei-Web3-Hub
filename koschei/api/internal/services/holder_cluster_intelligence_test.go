package services

import (
	"context"
	"testing"
	"time"
)

func TestSummarizeHolderClusterHighConfidence(t *testing.T) {
	wallets := []HolderClusterWallet{
		{Wallet: "A", HolderPercentage: 4, Status: "verified_bounded_observation", ParsedTransactions: 1, FreshNearLaunch: true, FundingSource: "F", FundingAmountSOL: .1, AcquisitionSlot: 100},
		{Wallet: "B", HolderPercentage: 3, Status: "verified_bounded_observation", ParsedTransactions: 1, FreshNearLaunch: true, FundingSource: "F", FundingAmountSOL: .1, AcquisitionSlot: 101},
		{Wallet: "C", HolderPercentage: 2, Status: "verified_bounded_observation", ParsedTransactions: 1, FreshNearLaunch: true, FundingSource: "F", FundingAmountSOL: .1, AcquisitionSlot: 102},
	}
	out := summarizeHolderCluster(HolderClusterAnalysis{WalletsRequested: 3, Wallets: wallets, SharedFundingGroups: []HolderClusterGroup{}, SameAmountGroups: []HolderClusterGroup{}, SynchronizedWallets: []string{}, Findings: []string{}, Limitations: []string{}})
	if !out.Available || out.Confidence != "high" || out.RiskIndex < 70 {
		t.Fatalf("expected high-confidence cluster, got %+v", out)
	}
	if out.LinkedHolderPercentage != 9 {
		t.Fatalf("expected linked holder percentage 9, got %v", out.LinkedHolderPercentage)
	}
}

func TestSummarizeHolderClusterInsufficientIsNotLow(t *testing.T) {
	out := summarizeHolderCluster(HolderClusterAnalysis{WalletsRequested: 2, Wallets: []HolderClusterWallet{{Wallet: "A", Status: "verified_bounded_observation", ParsedTransactions: 1}, {Wallet: "B", Status: "signature_history_unavailable"}}, SharedFundingGroups: []HolderClusterGroup{}, SameAmountGroups: []HolderClusterGroup{}, SynchronizedWallets: []string{}, Findings: []string{}, Limitations: []string{}})
	if out.Available || out.RiskLevel == "low" {
		t.Fatalf("insufficient evidence must not become low: %+v", out)
	}
}

func TestHolderClusterScanCandidatesConcurrentPreservesOrderAndUsesBoundedWorkers(t *testing.T) {
	candidates := []HolderRoleAccount{
		{OwnerWallet: "wallet-1"},
		{OwnerWallet: "wallet-2"},
		{OwnerWallet: "wallet-3"},
		{OwnerWallet: "wallet-4"},
		{OwnerWallet: "wallet-5"},
	}
	plans := make([]holderScanPlan, len(candidates))
	for index := range plans {
		plans[index] = holderScanPlan{Tier: "shallow", SignatureLimit: 20, TransactionLimit: 2}
	}

	started := make(chan string, len(candidates))
	release := make(chan struct{})
	type scanResult struct {
		rows    []HolderClusterWallet
		stopped bool
	}
	done := make(chan scanResult, 1)
	go func() {
		rows, stopped := holderClusterScanCandidatesConcurrent(context.Background(), candidates, plans, func(_ context.Context, account HolderRoleAccount, plan holderScanPlan) HolderClusterWallet {
			started <- account.OwnerWallet
			<-release
			return HolderClusterWallet{Wallet: account.OwnerWallet, Tier: plan.Tier}
		})
		done <- scanResult{rows: rows, stopped: stopped}
	}()

	for index := 0; index < holderDeepConcurrencyMax; index++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("expected %d holder workers to start concurrently; only %d started", holderDeepConcurrencyMax, index)
		}
	}
	close(release)

	var result scanResult
	select {
	case result = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bounded holder scan did not complete")
	}
	if result.stopped {
		t.Fatal("background scan unexpectedly reported deadline stop")
	}
	if len(result.rows) != len(candidates) {
		t.Fatalf("rows=%d want=%d", len(result.rows), len(candidates))
	}
	for index, row := range result.rows {
		if row.Wallet != candidates[index].OwnerWallet {
			t.Fatalf("row order changed at %d: got %q want %q", index, row.Wallet, candidates[index].OwnerWallet)
		}
	}
}


func TestHolderClusterParallelScanAllowedRequiresDeterministicBudget(t *testing.T) {
	t.Setenv("KOSCHEI_HELIUS_ENHANCED_HISTORY_ENABLED", "false")
	plans := []holderScanPlan{
		{Tier: "deep", TransactionLimit: 10},
		{Tier: "shallow", TransactionLimit: 2},
	}
	if !holderClusterParallelScanAllowed(plans, 14) {
		t.Fatal("fully reserved standard-RPC plans should allow bounded parallel scan")
	}
	if holderClusterParallelScanAllowed(plans, 13) {
		t.Fatal("parallel scan must be disabled when planned RPC units exceed the scan budget")
	}
	plans[1].BudgetDegraded = true
	if holderClusterParallelScanAllowed(plans, 100) {
		t.Fatal("budget-degraded plans must remain sequential to preserve deterministic evidence allocation")
	}
}

func TestHolderClusterParallelScanDisabledForEnhancedHistory(t *testing.T) {
	t.Setenv("KOSCHEI_HELIUS_ENHANCED_HISTORY_ENABLED", "true")
	plans := []holderScanPlan{
		{Tier: "deep", TransactionLimit: 10},
		{Tier: "shallow", TransactionLimit: 2},
	}
	if holderClusterParallelScanAllowed(plans, 100) {
		t.Fatal("enhanced-history scans use variable provider units and must remain sequential")
	}
}

func TestHolderClusterScanCandidatesSequentialPreservesPrefixOnDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	candidates := []HolderRoleAccount{{OwnerWallet: "wallet-1"}, {OwnerWallet: "wallet-2"}, {OwnerWallet: "wallet-3"}}
	plans := []holderScanPlan{{}, {}, {}}
	seen := 0
	rows, stopped := holderClusterScanCandidatesSequential(ctx, candidates, plans, func(_ context.Context, account HolderRoleAccount, _ holderScanPlan) HolderClusterWallet {
		seen++
		if seen == 2 {
			cancel()
		}
		return HolderClusterWallet{Wallet: account.OwnerWallet}
	})
	if !stopped {
		t.Fatal("cancelled sequential scan must report a deadline/cancellation stop")
	}
	if len(rows) != 2 || rows[0].Wallet != "wallet-1" || rows[1].Wallet != "wallet-2" {
		t.Fatalf("sequential deadline prefix changed: %+v", rows)
	}
}
