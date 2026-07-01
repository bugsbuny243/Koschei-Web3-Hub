package handlers

import (
	"math/big"
	"testing"
)

func TestTokenAccessAmountMath(t *testing.T) {
	raw, err := parseTokenAmount("12.3456", 6)
	if err != nil || raw.String() != "12345600" {
		t.Fatalf("unexpected parse result: %v %v", raw, err)
	}
	if got := formatTokenAmount(raw, 6); got != "12.3456" {
		t.Fatalf("unexpected format: %s", got)
	}
}

func TestTokenAccessTierOrder(t *testing.T) {
	thresholds := map[string]*big.Int{
		"basic":      big.NewInt(5),
		"pro":        big.NewInt(20),
		"enterprise": big.NewInt(100),
	}
	if got := evaluateTokenTier(big.NewInt(25), thresholds); got != "pro" {
		t.Fatalf("tier = %s", got)
	}
	if tokenTierRank("enterprise") <= tokenTierRank("pro") {
		t.Fatal("invalid tier order")
	}
}

func TestTokenAccessBasicPremiumThreshold(t *testing.T) {
	thresholds := map[string]*big.Int{
		"basic":      big.NewInt(5),
		"pro":        big.NewInt(20),
		"enterprise": big.NewInt(100),
	}
	if got := evaluateTokenTier(big.NewInt(4), thresholds); tokenTierRank(got) >= tokenTierRank("basic") {
		t.Fatalf("amount below basic should not unlock premium, tier=%s", got)
	}
	if got := evaluateTokenTier(big.NewInt(5), thresholds); tokenTierRank(got) < tokenTierRank("basic") {
		t.Fatalf("basic threshold should unlock premium, tier=%s", got)
	}
}
