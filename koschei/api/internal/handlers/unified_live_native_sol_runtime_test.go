package handlers

import (
	"testing"

	"koschei/api/internal/services"
)

func TestParseUnifiedLiveTransactionCarriesNativeSOLDelta(t *testing.T) {
	mint := "Mint111"
	wallet := "Owner111"
	tx := unifiedLiveTestTransaction(wallet, "Pool111", mint, 100, 60, 0, 40, "swap")
	meta := tx["meta"].(map[string]any)
	meta["preBalances"] = []any{float64(2_000_000_000)}
	meta["postBalances"] = []any{float64(2_750_000_000)}

	row, ok := parseUnifiedLiveTransaction(mint, unifiedLiveWalletTarget{Wallet: wallet, Role: "risk_bearing_holder"}, services.SolanaSignatureInfo{Signature: "SigNative111", Slot: 200}, tx)
	if !ok {
		t.Fatal("transaction row was not returned")
	}
	if !row.NetNativeSOLAvailable {
		t.Fatalf("native SOL delta unavailable: %#v", row)
	}
	if row.NetNativeSOLDelta != 0.75 {
		t.Fatalf("native SOL delta = %v, want 0.75", row.NetNativeSOLDelta)
	}
	if row.NetNativeSOLStatus != "transaction_backed_net_wallet_balance_change" {
		t.Fatalf("native SOL status = %q", row.NetNativeSOLStatus)
	}
	if row.NetNativeSOLSource != "solana_jsonparsed_pre_post_balances" {
		t.Fatalf("native SOL source = %q", row.NetNativeSOLSource)
	}
}

func TestParseUnifiedLiveTransactionKeepsNativeSOLUnavailableWithoutBalances(t *testing.T) {
	mint := "Mint111"
	wallet := "Owner111"
	tx := unifiedLiveTestTransaction(wallet, "Pool111", mint, 100, 60, 0, 40, "swap")

	row, ok := parseUnifiedLiveTransaction(mint, unifiedLiveWalletTarget{Wallet: wallet, Role: "risk_bearing_holder"}, services.SolanaSignatureInfo{Signature: "SigNoNative111", Slot: 201}, tx)
	if !ok {
		t.Fatal("transaction row was not returned")
	}
	if row.NetNativeSOLAvailable {
		t.Fatalf("native SOL delta unexpectedly available: %#v", row)
	}
	if row.NetNativeSOLDelta != 0 || row.NetNativeSOLStatus != "unavailable" || row.NetNativeSOLSource != "" {
		t.Fatalf("unavailable native SOL provenance = %#v", row)
	}
}

func TestParseUnifiedLiveTransactionDistinguishesZeroDeltaFromUnavailable(t *testing.T) {
	mint := "Mint111"
	wallet := "Owner111"
	tx := unifiedLiveTestTransaction(wallet, "Pool111", mint, 100, 60, 0, 40, "swap")
	meta := tx["meta"].(map[string]any)
	meta["preBalances"] = []any{float64(2_000_000_000)}
	meta["postBalances"] = []any{float64(2_000_000_000)}

	row, ok := parseUnifiedLiveTransaction(mint, unifiedLiveWalletTarget{Wallet: wallet, Role: "risk_bearing_holder"}, services.SolanaSignatureInfo{Signature: "SigZeroNative111", Slot: 202}, tx)
	if !ok {
		t.Fatal("transaction row was not returned")
	}
	if !row.NetNativeSOLAvailable || row.NetNativeSOLDelta != 0 {
		t.Fatalf("zero native SOL delta lost availability: %#v", row)
	}
	if row.NetNativeSOLStatus != "transaction_backed_net_wallet_balance_change" {
		t.Fatalf("zero-delta status = %q", row.NetNativeSOLStatus)
	}
}
