package services

import "testing"

func TestHolderClusterFlowSummarizesCommonExitAndCircularTransfers(t *testing.T) {
	wallets := []HolderClusterWallet{
		{
			Wallet: "WalletA", HolderPercentage: 10,
			FlowObservations: []HolderClusterFlowObservation{
				{SourceWallet: "WalletA", Destination: "ExitOwner", Kind: "external_token_recipient", Amount: 4, Signature: "sig-a-exit"},
				{SourceWallet: "WalletA", Destination: "WalletB", Kind: "holder_to_holder", Amount: 1, Signature: "sig-a-b"},
			},
		},
		{
			Wallet: "WalletB", HolderPercentage: 12,
			FlowObservations: []HolderClusterFlowObservation{
				{SourceWallet: "WalletB", Destination: "ExitOwner", Kind: "external_token_recipient", Amount: 5, Signature: "sig-b-exit"},
				{SourceWallet: "WalletB", Destination: "WalletA", Kind: "holder_to_holder", Amount: 1, Signature: "sig-b-a"},
			},
		},
		{Wallet: "WalletC", HolderPercentage: 8, FlowObservations: []HolderClusterFlowObservation{}},
	}

	flow := summarizeHolderClusterFlow(wallets)
	if !flow.Available {
		t.Fatalf("expected available flow analysis: %#v", flow)
	}
	if flow.CommonExitGroupCount != 1 || flow.LargestCommonExitGroup != 2 {
		t.Fatalf("unexpected common exit groups: %#v", flow.CommonExitGroups)
	}
	if flow.InternalTransferCount != 2 {
		t.Fatalf("internal transfer count = %d", flow.InternalTransferCount)
	}
	if flow.CircularWalletCount != 2 {
		t.Fatalf("circular wallet count = %d (%v)", flow.CircularWalletCount, flow.CircularWallets)
	}
	if flow.LinkedHolderPercentage != 22 {
		t.Fatalf("linked holder percentage = %f", flow.LinkedHolderPercentage)
	}
	if flow.RiskContribution < 30 || flow.Confidence != "high" {
		t.Fatalf("unexpected flow risk/confidence: %d %s", flow.RiskContribution, flow.Confidence)
	}
}

func TestHolderClusterFlowDoesNotScoreSharedDEXProgramAsCommonExit(t *testing.T) {
	wallets := []HolderClusterWallet{
		{Wallet: "WalletA", HolderPercentage: 10, FlowObservations: []HolderClusterFlowObservation{{SourceWallet: "WalletA", Destination: pumpLiquidityProgramID, Kind: "dex_program_exit_context", Amount: 2, Signature: "sig-a", ProgramIDs: []string{pumpLiquidityProgramID}}}},
		{Wallet: "WalletB", HolderPercentage: 12, FlowObservations: []HolderClusterFlowObservation{{SourceWallet: "WalletB", Destination: pumpLiquidityProgramID, Kind: "dex_program_exit_context", Amount: 3, Signature: "sig-b", ProgramIDs: []string{pumpLiquidityProgramID}}}},
	}
	flow := summarizeHolderClusterFlow(wallets)
	if !flow.Available {
		t.Fatal("expected route context to be available")
	}
	if flow.CommonExitGroupCount != 0 || flow.LargestCommonExitGroup != 0 {
		t.Fatalf("DEX program identity must not be common-exit evidence: %#v", flow.CommonExitGroups)
	}
	if flow.RiskContribution != 0 {
		t.Fatalf("program-only route context must not increase risk: %d", flow.RiskContribution)
	}
}

func TestHolderClusterFlowDoesNotScoreSharedDEXRecipientOwner(t *testing.T) {
	wallets := []HolderClusterWallet{
		{Wallet: "WalletA", HolderPercentage: 10, FlowObservations: []HolderClusterFlowObservation{{SourceWallet: "WalletA", Destination: "PoolAuthority", Kind: "external_token_recipient", Amount: 2, Signature: "sig-a", ProgramIDs: []string{pumpLiquidityProgramID}}}},
		{Wallet: "WalletB", HolderPercentage: 12, FlowObservations: []HolderClusterFlowObservation{{SourceWallet: "WalletB", Destination: "PoolAuthority", Kind: "external_token_recipient", Amount: 3, Signature: "sig-b", ProgramIDs: []string{pumpLiquidityProgramID}}}},
	}
	flow := summarizeHolderClusterFlow(wallets)
	if flow.CommonExitGroupCount != 0 || flow.RiskContribution != 0 {
		t.Fatalf("shared DEX recipient owner must remain route context only: %#v", flow)
	}
}

func TestHolderClusterTokenOwnerDeltas(t *testing.T) {
	tx := map[string]any{
		"meta": map[string]any{
			"preTokenBalances": []any{
				map[string]any{"mint": "Mint", "owner": "WalletA", "uiTokenAmount": map[string]any{"uiAmountString": "10"}},
				map[string]any{"mint": "Mint", "owner": "WalletB", "uiTokenAmount": map[string]any{"uiAmountString": "2"}},
			},
			"postTokenBalances": []any{
				map[string]any{"mint": "Mint", "owner": "WalletA", "uiTokenAmount": map[string]any{"uiAmountString": "6"}},
				map[string]any{"mint": "Mint", "owner": "WalletB", "uiTokenAmount": map[string]any{"uiAmountString": "6"}},
			},
		},
	}
	deltas := holderClusterTokenOwnerDeltas(tx, "Mint")
	if deltas["WalletA"] != -4 || deltas["WalletB"] != 4 {
		t.Fatalf("unexpected deltas: %#v", deltas)
	}
}

func TestHolderClusterTransactionIndexesCoverNewestOldestAndLaunch(t *testing.T) {
	newestTime := int64(300)
	launchTime := int64(200)
	oldestTime := int64(100)
	signatures := []SolanaSignatureInfo{
		{Signature: "newest", BlockTime: &newestTime},
		{Signature: "launch", BlockTime: &launchTime},
		{Signature: "oldest", BlockTime: &oldestTime},
	}
	indexes := holderClusterTransactionIndexes(signatures, launchTime)
	if len(indexes) != 3 {
		t.Fatalf("indexes = %v", indexes)
	}
	seen := map[int]bool{}
	for _, index := range indexes {
		seen[index] = true
	}
	for _, expected := range []int{0, 1, 2} {
		if !seen[expected] {
			t.Fatalf("missing index %d in %v", expected, indexes)
		}
	}
}
