package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTokenMarketSparsePrimaryCrossChecksDeepPYUSDUSDCWhirlpool(t *testing.T) {
	const mint = "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo"
	const shallow = "9Vh6fqJjDkqSTZ8bDXseVxGb2yQEMkEhhtte2anQCHSf"
	const deep = "9tXiuRRw7kbejLhZXtxDxYs2REe43uH2e7k1kocgdM9B"
	const thinPair = `{"chainId":"solana","dexId":"orca","pairAddress":"` + shallow + `","baseToken":{"address":"` + mint + `","symbol":"PYUSD"},"quoteToken":{"address":"SOL","symbol":"SOL"},"priceUsd":"1.0005","volume":{"h24":140000},"liquidity":{"usd":50647.37},"marketCap":2869032993}`
	const largePair = `{"chainId":"solana","dexId":"orca","pairAddress":"` + deep + `","baseToken":{"address":"` + mint + `","symbol":"PYUSD"},"quoteToken":{"address":"USDC","symbol":"USDC"},"priceUsd":"1.0002","volume":{"h24":951000},"liquidity":{"usd":16600000},"marketCap":2869032993}`
	var primary, secondary atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/tokens/v1/solana/"):
			primary.Add(1)
			_, _ = w.Write([]byte("[" + thinPair + "]"))
		case strings.HasPrefix(r.URL.Path, "/token-pairs/v1/solana/"):
			secondary.Add(1)
			// Include a duplicate shallow pool, an unrelated spoof and an
			// otherwise eligible quote-side pool. Never reuse the base price.
			_, _ = w.Write([]byte("[" + thinPair + "," + largePair + `,{"chainId":"solana","pairAddress":"malicious-unrelated","baseToken":{"address":"NOT_PYUSD"},"quoteToken":{"address":"USDC"},"liquidity":{"usd":900000000}},{"chainId":"solana","pairAddress":"quote-only","baseToken":{"address":"USDC"},"quoteToken":{"address":"` + mint + `"},"priceUsd":"1234","volume":{"h24":2000},"liquidity":{"usd":1000}}]`))
		default:
			t.Errorf("unexpected market path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := &TokenMarketClient{Endpoint: server.URL + "/tokens/v1/solana", PairsEndpoint: server.URL + "/token-pairs/v1/solana", Client: server.Client()}
	got := client.Fetch(context.Background(), mint)
	if !got.Available || got.Status != "verified_market_snapshot" {
		t.Fatalf("market evidence was lost: %#v", got)
	}
	if primary.Load() != 1 || secondary.Load() != 1 {
		t.Fatalf("sparse market should consult both endpoints once: %d %d", primary.Load(), secondary.Load())
	}
	if got.BestPairAddress != deep || got.PriceUSD != 1.0002 || got.BestPairLiquidityUSD != 16600000 {
		t.Fatalf("largest verified-returned mint-side pool not selected: %#v", got)
	}
	if got.PairCount != 3 || got.LiquidityUSD != 16651647.37 {
		t.Fatalf("duplicate/unrelated pool contaminated market totals: %#v", got)
	}
}

func TestTokenMarketSparseSecondary403PreservesLimitedPrimaryEvidence(t *testing.T) {
	const mint = "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/token-pairs/") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"chainId":"solana","dexId":"orca","pairAddress":"thin","baseToken":{"address":"` + mint + `"},"quoteToken":{"address":"SOL"},"priceUsd":"1","liquidity":{"usd":50647.37}}]`))
	}))
	defer server.Close()
	client := &TokenMarketClient{Endpoint: server.URL + "/tokens/v1/solana", PairsEndpoint: server.URL + "/token-pairs/v1/solana", Client: server.Client()}
	got := client.Fetch(context.Background(), mint)
	if !got.Available || got.PairCount != 1 || got.BestPairAddress != "thin" || got.LiquidityUSD != 50647.37 {
		t.Fatalf("secondary 403 erased real primary observation: %#v", got)
	}
	allLimitations := strings.Join(got.Limitations, " ")
	if !strings.Contains(allLimitations, "Secondary token-pairs observations unavailable") ||
		!strings.Contains(allLimitations, "not complete chain-wide liquidity coverage") {
		t.Fatalf("incomplete market pair evidence was not disclosed: %v", got.Limitations)
	}
}
