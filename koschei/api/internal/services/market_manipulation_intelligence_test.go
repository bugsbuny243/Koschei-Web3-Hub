package services

import (
	"context"
	"testing"
	"time"
)

func TestMarketManipulationUnavailableMakesNoClaim(t *testing.T) {
	report, err := LoadMarketManipulationIntelligence(context.Background(), nil, "mint-a", time.Unix(1700000000, 0).UTC(), 25)
	if err != nil {
		t.Fatal(err)
	}
	if report.Available || report.Complete || report.CandidateCount != 0 {
		t.Fatalf("unavailable ledger must remain unavailable: %#v", report)
	}
	if report.WashTradingProven || report.ManipulationClaim || report.SameOperatorClaim || report.VerdictAuthority {
		t.Fatal("missing evidence must not manufacture manipulation or identity claims")
	}
}

func TestQualifiesMarketRoundTripChurnRequiresAllThresholds(t *testing.T) {
	if !QualifiesMarketRoundTripChurn(3, 3, 3.0, 2.5) {
		t.Fatal("expected balanced, sufficiently active round trip to qualify as an investigation candidate")
	}
	if QualifiesMarketRoundTripChurn(2, 3, 3.0, 2.5) {
		t.Fatal("buy-count threshold must be enforced")
	}
	if QualifiesMarketRoundTripChurn(3, 2, 3.0, 2.5) {
		t.Fatal("sell-count threshold must be enforced")
	}
	if QualifiesMarketRoundTripChurn(3, 3, 2.0, 2.0) {
		t.Fatal("gross-volume threshold must be enforced")
	}
	if QualifiesMarketRoundTripChurn(3, 3, 8.0, 1.0) {
		t.Fatal("highly imbalanced flow must not qualify as balanced round-trip churn")
	}
}
