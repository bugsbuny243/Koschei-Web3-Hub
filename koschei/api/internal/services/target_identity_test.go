package services

import (
	"testing"
	"time"
)

func TestBuildTargetIdentityUsesCanonicalMarketPair(t *testing.T) {
	when := time.Date(2026, 7, 17, 23, 55, 0, 0, time.UTC)
	market := TokenMarketSnapshot{Available: true, Name: "Example Coin", Symbol: "EXAMPLE", LogoURL: "https://cdn.example/token.png", Provider: "dexscreener", BestPairAddress: "Pair111", BestPairDEX: "pumpswap", BestPairBaseSymbol: "EXAMPLE", BestPairQuoteSymbol: "SOL", PriceUSD: .2, MarketCapUSD: 255410, BestPairVolume24hUSD: 221370.34, BestPairLiquidityUSD: 37401.74}
	got := BuildTargetIdentity("Mint111", market, when)
	if got.MetadataStatus != "observed" || got.MetadataSource != "DexScreener" || !got.Pair.Canonical || got.Pair.QuoteSymbol != "SOL" {
		t.Fatalf("identity did not preserve observed canonical pair: %#v", got)
	}
	if !got.ScannedAt.Equal(when) || got.Market.Volume24hUSD != 221370.34 {
		t.Fatalf("identity lost scan time or canonical market values: %#v", got)
	}
}

func TestBuildTargetIdentityUnknownFallback(t *testing.T) {
	got := BuildTargetIdentity("MintUnknown", TokenMarketSnapshot{}, time.Now())
	if got.MetadataStatus != "unknown" || got.Name != "" || got.Pair.Canonical {
		t.Fatalf("unresolved metadata must stay unknown: %#v", got)
	}
}
