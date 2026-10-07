package handlers

import (
	"testing"
	"time"
)

func TestBoundedMarketBehaviorIndicatorsRequireCoverage(t *testing.T) {
	now := time.Now().UTC()
	rows := []unifiedTransactionEvidence{
		{Signature: "A", Trader: "Wallet1", Direction: "buy", BlockTime: &now},
		{Signature: "B", Trader: "Wallet1", Direction: "sell", BlockTime: &now},
		{Signature: "C", Trader: "Wallet2", Direction: "buy", BlockTime: &now},
	}
	out := boundedMarketBehaviorIndicators(rows)
	if out["market_behavior_status"] != "insufficient_evidence" {
		t.Fatalf("status=%#v", out["market_behavior_status"])
	}
	if out["wash_claim"] != false || out["market_manipulation_claim"] != false {
		t.Fatalf("claims must stay false: %#v", out)
	}
}

func TestBoundedMarketBehaviorIndicatorsObserveRapidRoundTrips(t *testing.T) {
	base := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	at := func(offset time.Duration) *time.Time {
		value := base.Add(offset)
		return &value
	}
	rows := []unifiedTransactionEvidence{
		{Signature: "A1", Trader: "Wallet1", Direction: "buy", BlockTime: at(0)},
		{Signature: "A2", Trader: "Wallet1", Direction: "sell", BlockTime: at(2 * time.Minute)},
		{Signature: "B1", Trader: "Wallet2", Direction: "sell", BlockTime: at(10 * time.Second)},
		{Signature: "B2", Trader: "Wallet2", Direction: "buy", BlockTime: at(3 * time.Minute)},
		{Signature: "C1", Trader: "Wallet3", Direction: "buy", BlockTime: at(20 * time.Second)},
		{Signature: "C2", Trader: "Wallet3", Direction: "buy", BlockTime: at(4 * time.Minute)},
	}
	out := boundedMarketBehaviorIndicators(rows)
	if out["market_behavior_status"] != "bounded_pattern_observed" {
		t.Fatalf("status=%#v out=%#v", out["market_behavior_status"], out)
	}
	if out["rapid_round_trip_wallet_count"] != int64(2) {
		t.Fatalf("rapid_round_trip_wallet_count=%#v", out["rapid_round_trip_wallet_count"])
	}
	if out["wash_like_indicator"] != "observed_requires_corroboration" {
		t.Fatalf("indicator=%#v", out["wash_like_indicator"])
	}
	if out["wash_claim"] != false || out["market_manipulation_claim"] != false || out["same_operator_claim"] != false {
		t.Fatalf("bounded indicators must never become attribution claims: %#v", out)
	}
}

func TestBoundedMarketBehaviorIndicatorsObserveCoordinatedOppositeSideBurst(t *testing.T) {
	base := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	at := func(seconds int) *time.Time {
		value := base.Add(time.Duration(seconds) * time.Second)
		return &value
	}
	rows := []unifiedTransactionEvidence{
		{Signature: "A", Trader: "Wallet1", Direction: "buy", BlockTime: at(1)},
		{Signature: "B", Trader: "Wallet2", Direction: "buy", BlockTime: at(8)},
		{Signature: "C", Trader: "Wallet3", Direction: "sell", BlockTime: at(16)},
		{Signature: "D", Trader: "Wallet4", Direction: "sell", BlockTime: at(24)},
		{Signature: "E", Trader: "Wallet5", Direction: "buy", BlockTime: at(70)},
		{Signature: "F", Trader: "Wallet6", Direction: "sell", BlockTime: at(130)},
	}
	out := boundedMarketBehaviorIndicators(rows)
	if out["coordinated_opposite_side_bursts"] != int64(1) {
		t.Fatalf("coordinated_opposite_side_bursts=%#v out=%#v", out["coordinated_opposite_side_bursts"], out)
	}
	if out["market_behavior_status"] != "bounded_pattern_observed" {
		t.Fatalf("status=%#v", out["market_behavior_status"])
	}
}
