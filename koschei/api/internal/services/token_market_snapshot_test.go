package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTokenMarketSnapshotUsesMostLiquidBasePairAndAggregatesMarketActivity(t *testing.T) {
	mint := "Mint111111111111111111111111111111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, mint) {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"chainId":"solana","dexId":"thin","pairAddress":"pair-thin","baseToken":{"address":"` + mint + `","name":"Token","symbol":"TOK"},"quoteToken":{"address":"SOL"},"priceUsd":"9.99","txns":{"h24":{"buys":4,"sells":2}},"volume":{"h24":100000},"priceChange":{"h24":50},"liquidity":{"usd":1000},"marketCap":9990000,"fdv":9990000},
			{"chainId":"solana","dexId":"deep","pairAddress":"pair-deep","baseToken":{"address":"` + mint + `","name":"Token","symbol":"TOK"},"quoteToken":{"address":"USDC"},"priceUsd":"0.25","txns":{"h24":{"buys":40,"sells":20}},"volume":{"h24":500000},"priceChange":{"h24":-5},"liquidity":{"usd":250000},"marketCap":2500000,"fdv":3000000},
			{"chainId":"solana","dexId":"deep","pairAddress":"pair-deep","baseToken":{"address":"` + mint + `"},"quoteToken":{"address":"USDC"},"priceUsd":"0.25","volume":{"h24":500000},"liquidity":{"usd":250000}},
			{"chainId":"solana","dexId":"quote-only","pairAddress":"pair-quote","baseToken":{"address":"OTHER","name":"Other","symbol":"OTH"},"quoteToken":{"address":"` + mint + `"},"priceUsd":"1234","txns":{"h24":{"buys":1,"sells":1}},"volume":{"h24":1000},"liquidity":{"usd":500}},
			{"chainId":"ethereum","dexId":"wrong-chain","pairAddress":"pair-eth","baseToken":{"address":"` + mint + `"},"priceUsd":"777","volume":{"h24":999999}}
		]`))
	}))
	defer server.Close()

	client := &TokenMarketClient{Endpoint: server.URL, Client: server.Client()}
	market := client.Fetch(context.Background(), mint)
	if !market.Available || market.Status != "verified_market_snapshot" {
		t.Fatalf("market unavailable: %#v", market)
	}
	if market.PriceUSD != 0.25 || market.BestPairAddress != "pair-deep" {
		t.Fatalf("wrong price pair: %#v", market)
	}
	if market.Volume24hUSD != 601000 {
		t.Fatalf("volume = %.2f", market.Volume24hUSD)
	}
	if market.LiquidityUSD != 251500 {
		t.Fatalf("liquidity = %.2f", market.LiquidityUSD)
	}
	if market.Buys24h != 45 || market.Sells24h != 23 {
		t.Fatalf("txns = %d/%d", market.Buys24h, market.Sells24h)
	}
	if market.PairCount != 3 {
		t.Fatalf("pair count = %d", market.PairCount)
	}
	if market.MarketCapUSD != 2500000 || market.FDVUSD != 3000000 || market.PriceChange24hPct != -5 {
		t.Fatalf("best-pair metrics = %#v", market)
	}
}

func TestTokenMarketSnapshotDoesNotUseQuoteTokenBasePrice(t *testing.T) {
	mint := "MintQuote11111111111111111111111111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"chainId":"solana","dexId":"dex","pairAddress":"pair","baseToken":{"address":"OTHER"},"quoteToken":{"address":"` + mint + `"},"priceUsd":"42","volume":{"h24":10},"liquidity":{"usd":20}}]`))
	}))
	defer server.Close()

	market := (&TokenMarketClient{Endpoint: server.URL, Client: server.Client()}).Fetch(context.Background(), mint)
	if market.PriceUSD != 0 {
		t.Fatalf("quote token inherited base price: %.2f", market.PriceUSD)
	}
	if market.PairCount != 1 || market.Volume24hUSD != 10 {
		t.Fatalf("market context missing: %#v", market)
	}
}

func TestTokenMarketSnapshotPositiveLiquidityOutranksNoLiquidityVolume(t *testing.T) {
	mint := "MintLiquidity11111111111111111111111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"chainId":"solana","dexId":"no-liquidity","pairAddress":"a","baseToken":{"address":"` + mint + `"},"quoteToken":{"address":"SOL"},"priceUsd":"99","volume":{"h24":900000000}},
			{"chainId":"solana","dexId":"liquid","pairAddress":"b","baseToken":{"address":"` + mint + `"},"quoteToken":{"address":"USDC"},"priceUsd":"0.10","volume":{"h24":1000},"liquidity":{"usd":10}}
		]`))
	}))
	defer server.Close()
	market := (&TokenMarketClient{Endpoint: server.URL, Client: server.Client()}).Fetch(context.Background(), mint)
	if market.PriceUSD != 0.10 || market.BestPairAddress != "b" {
		t.Fatalf("no-liquidity pair became price reference: %#v", market)
	}
}
