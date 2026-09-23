package services

import (
	"testing"
	"time"
)

func TestBuildSolanaLiquidityRadarEventIsDescriptiveOnly(t *testing.T) {
	mint := "So11111111111111111111111111111111111111112"
	previous := TokenMarketSnapshot{
		Available: true, Status: "verified_market_snapshot", Provider: "dexscreener",
		Mint: mint, LiquidityUSD: 100000, Volume24hUSD: 50000, PriceUSD: 1,
		BestPairAddress: "Pair111", BestPairDEX: "raydium",
		ObservedAt:     time.Date(2026, 9, 23, 17, 0, 0, 0, time.UTC),
		ValuationScope: "most_liquid_solana_pair_reference_price",
	}
	current := previous
	current.LiquidityUSD = 25000
	current.Volume24hUSD = 90000
	current.PriceUSD = 0.7
	current.ObservedAt = time.Date(2026, 9, 23, 18, 0, 0, 0, time.UTC)

	got, err := BuildSolanaLiquidityRadarEvent(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if got.ObservationKind != GlobalRadarObservationLiquidity ||
		got.Evidence.Status != IntelligenceEvidenceObserved ||
		got.DecisionState != "evidence_only_no_verdict_created" {
		t.Fatalf("unexpected liquidity radar event: %#v", got)
	}
	if got.Evidence.Attributes["liquidity_delta_usd"] != -75000.0 ||
		got.Evidence.Attributes["market_data_can_issue_verdict"] != false ||
		got.Evidence.Attributes["rug_or_drain_claim"] != false {
		t.Fatalf("liquidity event crossed verdict boundary: %#v", got.Evidence.Attributes)
	}
}

func TestBuildSolanaLiquidityRadarEventRejectsSingleOrMismatchedContext(t *testing.T) {
	base := TokenMarketSnapshot{
		Available: true, Status: "verified_market_snapshot", Provider: "dexscreener",
		Mint:         "So11111111111111111111111111111111111111112",
		LiquidityUSD: 100, ObservedAt: time.Date(2026, 9, 23, 17, 0, 0, 0, time.UTC),
	}
	current := base
	current.ObservedAt = time.Date(2026, 9, 23, 18, 0, 0, 0, time.UTC)

	mismatch := current
	mismatch.Mint = "11111111111111111111111111111111"
	if _, err := BuildSolanaLiquidityRadarEvent(base, mismatch); err == nil {
		t.Fatal("mismatched mint snapshots were accepted")
	}

	unavailable := current
	unavailable.Available = false
	if _, err := BuildSolanaLiquidityRadarEvent(base, unavailable); err == nil {
		t.Fatal("unavailable market snapshot was accepted")
	}

	sameTime := current
	sameTime.ObservedAt = base.ObservedAt
	if _, err := BuildSolanaLiquidityRadarEvent(base, sameTime); err == nil {
		t.Fatal("non-increasing market observation time was accepted")
	}
}
