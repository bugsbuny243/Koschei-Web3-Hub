package handlers

import (
	"math/big"
	"testing"
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
