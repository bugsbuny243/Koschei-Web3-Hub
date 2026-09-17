package handlers

import "testing"

func TestUnifiedWalletNativeSOLDeltaUsesAccountIndex(t *testing.T) {
	message := map[string]any{
		"accountKeys": []any{
			map[string]any{"pubkey": "Creator111", "signer": true},
			map[string]any{"pubkey": "Other222", "signer": false},
		},
	}
	meta := map[string]any{
		"preBalances":  []any{float64(2_000_000_000), float64(500_000_000)},
		"postBalances": []any{float64(3_250_000_000), float64(450_000_000)},
	}

	delta, ok := unifiedWalletNativeSOLDelta(message, meta, "Creator111")
	if !ok {
		t.Fatal("native SOL delta unavailable")
	}
	if delta != 1.25 {
		t.Fatalf("delta = %v, want 1.25", delta)
	}
}

func TestUnifiedWalletNativeSOLDeltaCanBeNegative(t *testing.T) {
	message := map[string]any{
		"accountKeys": []any{map[string]any{"pubkey": "Creator111", "signer": true}},
	}
	meta := map[string]any{
		"preBalances":  []any{float64(5_000_000_000)},
		"postBalances": []any{float64(4_900_000_000)},
	}

	delta, ok := unifiedWalletNativeSOLDelta(message, meta, "Creator111")
	if !ok {
		t.Fatal("native SOL delta unavailable")
	}
	if delta != -0.1 {
		t.Fatalf("delta = %v, want -0.1", delta)
	}
}

func TestUnifiedWalletNativeSOLDeltaWithholdsOnIncompleteBalances(t *testing.T) {
	message := map[string]any{
		"accountKeys": []any{
			map[string]any{"pubkey": "Creator111", "signer": true},
			map[string]any{"pubkey": "Other222", "signer": false},
		},
	}
	meta := map[string]any{
		"preBalances":  []any{float64(2_000_000_000), float64(500_000_000)},
		"postBalances": []any{float64(2_100_000_000)},
	}

	if delta, ok := unifiedWalletNativeSOLDelta(message, meta, "Creator111"); ok {
		t.Fatalf("incomplete balances produced delta %v", delta)
	}
}

func TestUnifiedWalletNativeSOLDeltaDoesNotClaimMissingWallet(t *testing.T) {
	message := map[string]any{
		"accountKeys": []any{map[string]any{"pubkey": "Other222", "signer": true}},
	}
	meta := map[string]any{
		"preBalances":  []any{float64(1_000_000_000)},
		"postBalances": []any{float64(1_500_000_000)},
	}

	if delta, ok := unifiedWalletNativeSOLDelta(message, meta, "Creator111"); ok {
		t.Fatalf("missing wallet produced delta %v", delta)
	}
}
