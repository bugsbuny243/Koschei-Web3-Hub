package handlers

import (
	"math/big"
	"testing"

	"koschei/api/internal/services"
)

func TestKOSCRawAmountForUSDUsesCeiling(t *testing.T) {
	raw, err := koscRawAmountForUSD("199", "0.00000341155", 6)
	if err != nil {
		t.Fatal(err)
	}
	if raw.String() != "58331257053246" {
		t.Fatalf("raw quote amount = %s", raw.String())
	}
	if got := formatKOSCRawAmount(raw, 6); got != "58331257.053246" {
		t.Fatalf("formatted quote amount = %s", got)
	}
}

func TestKOSCDecimalRatAcceptsScientificNotation(t *testing.T) {
	got, err := koscDecimalRat("3.41155e-6")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := new(big.Rat).SetString("68231/20000000000")
	if got.Cmp(want) != 0 {
		t.Fatalf("decimal rational = %s want %s", got.RatString(), want.RatString())
	}
}

func TestKOSCCheckoutConfigFailsClosedWithoutExplicitEnablement(t *testing.T) {
	t.Setenv("KOSCHEI_KOSC_CHECKOUT_ENABLED", "false")
	if _, err := loadKOSCCheckoutConfig(); err == nil {
		t.Fatal("expected disabled checkout")
	}
}

func TestKOSCCheckoutConfigAcceptsCanonicalThirtyDayTerm(t *testing.T) {
	t.Setenv("KOSCHEI_KOSC_CHECKOUT_ENABLED", "true")
	t.Setenv("KOSCHEI_TOKEN_NETWORK", "solana-mainnet")
	t.Setenv("KOSCHEI_TOKEN_MINT", canonicalKOSCMint)
	t.Setenv("KOSCHEI_TOKEN_TREASURY", "So11111111111111111111111111111111111111112")
	t.Setenv("JUPITER_API_KEY", "test-key")
	t.Setenv("KOSCHEI_KOSC_ACCESS_DAYS", "30")
	cfg, err := loadKOSCCheckoutConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AccessDays != 30 {
		t.Fatalf("access days = %d want 30", cfg.AccessDays)
	}
}

func TestKOSCCheckoutConfigRejectsNonCanonicalMint(t *testing.T) {
	t.Setenv("KOSCHEI_KOSC_CHECKOUT_ENABLED", "true")
	t.Setenv("KOSCHEI_TOKEN_NETWORK", "solana-mainnet")
	t.Setenv("KOSCHEI_TOKEN_MINT", "11111111111111111111111111111111")
	t.Setenv("KOSCHEI_TOKEN_TREASURY", "So11111111111111111111111111111111111111112")
	t.Setenv("JUPITER_API_KEY", "test-key")
	t.Setenv("KOSCHEI_KOSC_ACCESS_DAYS", "30")
	if _, err := loadKOSCCheckoutConfig(); err == nil {
		t.Fatal("expected non-canonical mint rejection")
	}
}

func TestVerifyKOSCSettlementTransactionBindsSignerTreasuryAndRawAmount(t *testing.T) {
	wallet := "11111111111111111111111111111111"
	treasury := "So11111111111111111111111111111111111111112"
	mint := canonicalKOSCMint
	tx := services.SolanaTransactionResult{
		"slot":      float64(999),
		"blockTime": float64(1700000000),
		"transaction": map[string]any{
			"message": map[string]any{
				"accountKeys": []any{
					map[string]any{"pubkey": wallet, "signer": true},
					map[string]any{"pubkey": "SourceToken111", "signer": false},
					map[string]any{"pubkey": "TreasuryToken111", "signer": false},
				},
			},
		},
		"meta": map[string]any{
			"err": nil,
			"preTokenBalances": []any{
				map[string]any{"accountIndex": float64(1), "mint": mint, "owner": wallet, "uiTokenAmount": map[string]any{"amount": "60000000", "decimals": float64(6)}},
				map[string]any{"accountIndex": float64(2), "mint": mint, "owner": treasury, "uiTokenAmount": map[string]any{"amount": "10000000", "decimals": float64(6)}},
			},
			"postTokenBalances": []any{
				map[string]any{"accountIndex": float64(1), "mint": mint, "owner": wallet, "uiTokenAmount": map[string]any{"amount": "10000000", "decimals": float64(6)}},
				map[string]any{"accountIndex": float64(2), "mint": mint, "owner": treasury, "uiTokenAmount": map[string]any{"amount": "60000000", "decimals": float64(6)}},
			},
		},
	}
	required := big.NewInt(50000000)
	evidence, err := verifyKOSCSettlementTransaction(tx, wallet, mint, treasury, required)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Slot != 999 || evidence.TreasuryRawDelta.Cmp(required) != 0 || evidence.WalletRawDecrease.Cmp(required) != 0 {
		t.Fatalf("unexpected settlement evidence: %+v", evidence)
	}
}

func TestVerifyKOSCSettlementTransactionRejectsNonSigner(t *testing.T) {
	wallet := "11111111111111111111111111111111"
	treasury := "So11111111111111111111111111111111111111112"
	tx := services.SolanaTransactionResult{
		"slot":        float64(1),
		"transaction": map[string]any{"message": map[string]any{"accountKeys": []any{map[string]any{"pubkey": wallet, "signer": false}}}},
		"meta":        map[string]any{"err": nil, "preTokenBalances": []any{}, "postTokenBalances": []any{}},
	}
	if _, err := verifyKOSCSettlementTransaction(tx, wallet, canonicalKOSCMint, treasury, big.NewInt(1)); err == nil {
		t.Fatal("expected non-signer settlement rejection")
	}
}

func TestValidateKOSCFinalizedStatusRequiresFinalizedSuccessfulSignature(t *testing.T) {
	slot, err := validateKOSCFinalizedStatus(koscSignatureStatusesResult{Value: []*koscSignatureStatus{{
		Slot: 991, ConfirmationStatus: "finalized",
	}}})
	if err != nil || slot != 991 {
		t.Fatalf("finalized status rejected: slot=%d err=%v", slot, err)
	}
	for _, tc := range []koscSignatureStatusesResult{
		{},
		{Value: []*koscSignatureStatus{nil}},
		{Value: []*koscSignatureStatus{{Slot: 991, ConfirmationStatus: "confirmed"}}},
		{Value: []*koscSignatureStatus{{Slot: 0, ConfirmationStatus: "finalized"}}},
		{Value: []*koscSignatureStatus{{Slot: 991, Err: map[string]any{"InstructionError": []any{0, "Custom"}}, ConfirmationStatus: "finalized"}}},
	} {
		if _, err := validateKOSCFinalizedStatus(tc); err == nil {
			t.Fatalf("non-finalized/failed status accepted: %#v", tc)
		}
	}
}
