package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDexScreenerPumpVolumeClientAggregatesUniqueSolanaPairs(t *testing.T) {
	mint := "Mint111111111111111111111111111111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, mint) {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"chainId":"solana","dexId":"pumpswap","pairAddress":"pair-a","baseToken":{"address":"` + mint + `","name":"Token","symbol":"TOK"},"quoteToken":{"address":"SOL"},"volume":{"h24":320000},"liquidity":{"usd":100000},"marketCap":900000,"fdv":1000000},
			{"chainId":"solana","dexId":"raydium","pairAddress":"pair-b","baseToken":{"address":"` + mint + `","name":"Token","symbol":"TOK"},"quoteToken":{"address":"USDC"},"volume":{"h24":210000},"liquidity":{"usd":50000},"marketCap":920000,"fdv":1020000},
			{"chainId":"solana","dexId":"raydium","pairAddress":"pair-b","baseToken":{"address":"` + mint + `","name":"Token","symbol":"TOK"},"quoteToken":{"address":"USDC"},"volume":{"h24":210000},"liquidity":{"usd":50000}},
			{"chainId":"ethereum","dexId":"uniswap","pairAddress":"pair-c","baseToken":{"address":"` + mint + `"},"quoteToken":{"address":"ETH"},"volume":{"h24":999999}}
		]`))
	}))
	defer server.Close()

	client := &DexScreenerPumpVolumeClient{Endpoint: server.URL, Client: server.Client()}
	markets, err := client.Fetch24hVolumes(context.Background(), []string{mint})
	if err != nil {
		t.Fatal(err)
	}
	market := markets[mint]
	if market.Volume24hUSD != 530000 {
		t.Fatalf("volume = %.2f", market.Volume24hUSD)
	}
	if market.PairCount != 2 {
		t.Fatalf("pair count = %d", market.PairCount)
	}
	if market.BestPairAddress != "pair-a" || market.BestPairVolume24hUSD != 320000 {
		t.Fatalf("best pair = %#v", market)
	}
	if market.LiquidityUSD != 150000 {
		t.Fatalf("liquidity = %.2f", market.LiquidityUSD)
	}
	if market.MarketCapUSD != 920000 || market.FDVUSD != 1020000 {
		t.Fatalf("market surface = %#v", market)
	}
}

func TestPumpHighVolumeSignalsUse24hUSDGate(t *testing.T) {
	observed := time.Date(2026, 7, 12, 4, 0, 0, 0, time.UTC)
	signals := pumpHighVolumeSignals(
		PumpRadarCandidate{Mint: "Mint", Name: "Candidate", Symbol: "CND", Creator: "Creator"},
		PumpTokenMarket{Mint: "Mint", Volume24hUSD: 500001.25, PairCount: 3, Provider: "dexscreener", ObservedAt: observed},
		500000,
	)
	if signals["volume_window"] != "24h" || signals["volume_currency"] != "USD" {
		t.Fatalf("wrong volume scope: %#v", signals)
	}
	if signals["auto_volume_gate"] != true || signals["source_verified_pump_event"] != true {
		t.Fatalf("missing gate evidence: %#v", signals)
	}
	if got := pumpSignalFloat(signals, "volume_24h_usd"); got != 500001.25 {
		t.Fatalf("volume = %.2f", got)
	}
	if got := pumpSignalFloat(signals, "volume_threshold_usd"); got != 500000 {
		t.Fatalf("threshold = %.2f", got)
	}
}

func TestPumpHighVolumeObservationSignatureIsHourlyAndMintScoped(t *testing.T) {
	at := time.Date(2026, 7, 12, 4, 12, 0, 0, time.UTC)
	a := pumpHighVolumeObservationSignature("MintA", at)
	b := pumpHighVolumeObservationSignature("MintA", at.Add(30*time.Minute))
	c := pumpHighVolumeObservationSignature("MintA", at.Add(time.Hour))
	d := pumpHighVolumeObservationSignature("MintB", at)
	if a != b {
		t.Fatalf("same hour must dedupe: %s != %s", a, b)
	}
	if a == c || a == d {
		t.Fatalf("signature must change by hour and mint")
	}
}

func TestPumpHighVolumeThresholdDefaultsTo500K(t *testing.T) {
	t.Setenv("PUMP_HIGH_VOLUME_MIN_24H_USD", "")
	if got := PumpHighVolumeThresholdUSD(); got != 500000 {
		t.Fatalf("threshold = %.2f", got)
	}
}
