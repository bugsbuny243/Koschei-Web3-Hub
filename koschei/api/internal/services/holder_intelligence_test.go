package services

import (
	"testing"
	"time"
)

func TestHolderIntelligenceAggregatesTokenAccountsByOwnerAndKeepsPendingFactsVisible(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	usd := 0.50
	roles := HolderRoleAnalysis{
		Available: true, Status: "dominant_holder_role_unresolved", BlockingEvidenceGap: true,
		Supply: 1000, CirculatingSupply: 900,
		Accounts: []HolderRoleAccount{
			{Rank: 1, TokenAccount: "UnknownTokenAccount", Balance: 600, Role: "owner_unresolved", Confidence: "low", ExcludedFromHolderRisk: false},
			{Rank: 2, TokenAccount: "OwnerAAccount1", OwnerWallet: "OwnerA", Balance: 100, Role: "externally_owned_wallet", Confidence: "high", ExcludedFromHolderRisk: false},
			{Rank: 3, TokenAccount: "OwnerAAccount2", OwnerWallet: "OwnerA", Balance: 50, Role: "externally_owned_wallet", Confidence: "high", ExcludedFromHolderRisk: false},
			{Rank: 4, TokenAccount: "ProtocolAccount", OwnerWallet: "ProtocolPDA", Balance: 100, Role: "pump_liquidity_vault", Confidence: "high", ExcludedFromHolderRisk: true},
			{Rank: 5, TokenAccount: "OwnerBAccount", OwnerWallet: "OwnerB", Balance: 50, Role: "externally_owned_wallet", Confidence: "high", ExcludedFromHolderRisk: false},
		},
	}
	cluster := HolderClusterAnalysis{
		Available: true,
		Wallets: []HolderClusterWallet{
			{Wallet: "OwnerA", SignaturesObserved: 20, ParsedTransactions: 3, AcquisitionSlot: 10, AcquisitionObservedAt: "2026-07-02T12:00:00Z", OldestObservedAt: "2026-06-30T12:00:00Z", NewestObservedAt: "2026-07-11T12:00:00Z", FlowObservations: []HolderClusterFlowObservation{{SourceWallet: "OwnerA", Destination: "Exit", Amount: 5, Signature: "sig-out"}}},
			{Wallet: "OwnerB", SignaturesObserved: 10, ParsedTransactions: 1, OldestObservedAt: "2026-07-10T12:00:00Z", NewestObservedAt: "2026-07-11T12:00:00Z"},
		},
		Flow: HolderClusterFlowAnalysis{CommonExitGroupCount: 1, CommonExitGroups: []HolderClusterGroup{{Key: "Exit", Wallets: []string{"OwnerA", "OwnerB"}, MemberCount: 2}}},
	}
	market := TokenMarketSnapshot{Available: true, Status: "verified_market_snapshot", PriceUSD: usd, Volume24hUSD: 500000, LiquidityUSD: 100000}

	result := BuildHolderIntelligence(roles, cluster, market, now)
	if !result.Available || !result.FinalVerdictBlocked || result.Status != "verified_holdings_final_pending" {
		t.Fatalf("unexpected result status: %#v", result)
	}
	if result.OwnerCount != 4 {
		t.Fatalf("owner count = %d", result.OwnerCount)
	}
	if len(result.Rows) != 4 || result.Rows[1].OwnerWallet != "OwnerA" {
		t.Fatalf("rows = %#v", result.Rows)
	}
	ownerA := result.Rows[1]
	if ownerA.Balance != 150 || ownerA.TokenAccountCount != 2 {
		t.Fatalf("owner aggregation failed: %#v", ownerA)
	}
	if ownerA.ReferenceUSDValue == nil || *ownerA.ReferenceUSDValue != 75 {
		t.Fatalf("owner USD value = %#v", ownerA.ReferenceUSDValue)
	}
	if ownerA.ObservedHoldingDays != 10 || ownerA.HoldingDurationScope != "bounded_first_observed_acquisition" {
		t.Fatalf("holding observation = %#v", ownerA)
	}
	if ownerA.OutflowTransactions != 1 || !ownerA.CommonExitObserved || ownerA.Behavior != "common_exit_recipient_observed" {
		t.Fatalf("behavior = %#v", ownerA)
	}
	if result.TopOwnerBalance != 600 || result.TopOwnerPercentage == 0 || result.TopOwnerReferenceUSDValue == nil || *result.TopOwnerReferenceUSDValue != 300 {
		t.Fatalf("top owner summary = %#v", result)
	}
	if len(result.Findings) < 3 {
		t.Fatalf("missing human findings: %#v", result.Findings)
	}
}

func TestHolderIntelligenceDoesNotCallWalletActivityHoldingDuration(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	roles := HolderRoleAnalysis{Available: true, Supply: 100, CirculatingSupply: 100, Accounts: []HolderRoleAccount{{Rank: 1, TokenAccount: "A", OwnerWallet: "WalletA", Balance: 10, Role: "externally_owned_wallet", Confidence: "high"}}}
	cluster := HolderClusterAnalysis{Wallets: []HolderClusterWallet{{Wallet: "WalletA", ParsedTransactions: 1, OldestObservedAt: "2026-07-01T12:00:00Z", NewestObservedAt: "2026-07-11T12:00:00Z"}}}
	result := BuildHolderIntelligence(roles, cluster, TokenMarketSnapshot{}, now)
	row := result.Rows[0]
	if row.ObservedHoldingDays != 0 || row.HoldingDurationScope != "wallet_activity_only_not_token_holding" {
		t.Fatalf("wallet age mislabeled as holding duration: %#v", row)
	}
	if row.ObservedActivityAgeDays != 11 {
		t.Fatalf("activity age = %d", row.ObservedActivityAgeDays)
	}
}
