package services

import "testing"

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
