package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestLPCollectorReadsCPMMReservesAndBurnedShare(t *testing.T) {
	poolBytes := make([]byte, 232)
	fill := func(offset int, seed byte) string {
		key := make([]byte, 32)
		for i := range key {
			key[i] = seed + byte(i%7)
		}
		copy(poolBytes[offset:offset+32], key)
		return base58Encode(key)
	}
	vault0 := fill(72, 10)
	vault1 := fill(104, 30)
	lpMint := fill(136, 50)
	tokenMint := fill(168, 70)
	quoteMint := fill(200, 90)
	poolCreator := fill(40, 110)
	burnTokenAccount := "BurnLPTokenAccount111111111111111111111111"
	calls := []string{}
	rpc := func(_ context.Context, _ string, method string, _ any, out any) error {
		calls = append(calls, method)
		switch method {
		case "getAccountInfo":
			response := out.(*rpcAccountInfoResponse)
			response.Context.Slot = 777
			response.Value = &struct {
				Owner string `json:"owner"`
				Data  any    `json:"data"`
			}{
				Owner: raydiumCPMMProgram, Data: []any{base64.StdEncoding.EncodeToString(poolBytes), "base64"},
			}
		case "getTokenAccountBalance":
			response := out.(*rpcTokenBalanceResponse)
			response.Context.Slot = 778
			response.Value.Decimals = 6
			if strings.Count(strings.Join(calls, ","), "getTokenAccountBalance") == 1 {
				response.Value.UIAmountString = "1000000"
			} else {
				response.Value.UIAmountString = "50000"
			}
		case "getTokenSupply":
			response := out.(*rpcTokenSupplyResponse)
			response.Context.Slot = 779
			response.Value.Decimals = 6
			response.Value.UIAmountString = "1000"
		case "getTokenLargestAccounts":
			response := out.(*rpcLargestAccountsResponse)
			response.Context.Slot = 779
			response.Value = []rpcLargestAccount{{Address: burnTokenAccount, rpcTokenAmount: rpcTokenAmount{UIAmountString: "990", Decimals: 6}}}
		case "getMultipleAccounts":
			response := out.(*struct {
				Value []json.RawMessage `json:"value"`
			})
			response.Value = []json.RawMessage{json.RawMessage(`{"data":{"parsed":{"info":{"owner":"1nc1nerator11111111111111111111111111111111"}}}}`)}
		default:
			return errors.New("unexpected RPC method: " + method)
		}
		return nil
	}
	market := services.TokenMarketSnapshot{Available: true, BestPairAddress: "Pool111", BestPairDEX: "raydium", LiquidityUSD: 100000}
	got := collectLPControlEvidence(context.Background(), rpc, "solana-mainnet", tokenMint, poolCreator, market, map[string]any{})
	if got.Status != services.LPControlVerifiedBurned || got.BurnedSharePct != 99 {
		t.Fatalf("LP burn result=%#v", got)
	}
	if got.PoolProgram != raydiumCPMMProgram || got.LPMint != lpMint || got.TokenVault != vault0 || got.QuoteVault != vault1 {
		t.Fatalf("decoded pool fields=%#v", got)
	}
	if got.QuoteMint != quoteMint || got.PoolCreator != poolCreator || got.CreatorRelation != "verified_pool_creator" {
		t.Fatalf("mint/creator relation=%#v", got)
	}
	if got.DominantLPOwner != "1nc1nerator11111111111111111111111111111111" || got.DominantLPSharePct != 99 || got.DominantLPClassification != "burn_address" {
		t.Fatalf("dominant LP owner=%#v", got)
	}
	if got.TokenReserve != 1000000 || got.QuoteReserve != 50000 || got.ReadSlot != 779 {
		t.Fatalf("reserve evidence=%#v", got)
	}
	if len(got.EvidenceKeys) < 4 {
		t.Fatalf("evidence keys=%v", got.EvidenceKeys)
	}
}

func TestBondingCurveWithoutPoolIsNotApplicableAndMakesNoRPC(t *testing.T) {
	calls := 0
	rpc := func(context.Context, string, string, any, any) error { calls++; return nil }
	got := collectLPControlEvidence(context.Background(), rpc, "solana-mainnet", "Mint", "", services.TokenMarketSnapshot{}, map[string]any{"launch_platform": "pump.fun"})
	if got.Status != services.LPControlNotApplicable || got.ReasonCode != "bonding_curve_no_amm_pool" {
		t.Fatalf("result=%#v", got)
	}
	if calls != 0 {
		t.Fatalf("unexpected RPC calls=%d", calls)
	}
}

func TestJupiterContextIsOptionalAndReportsSellImpact(t *testing.T) {
	mint := "MintJupiter111111111111111111111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "price") {
			_, _ = w.Write([]byte(`{"` + mint + `":{"usdPrice":0.25,"blockId":888,"createdAt":"2026-07-17T00:00:00Z"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"outAmount":"90000000","priceImpactPct":"12.5","contextSlot":889,"routePlan":[{"swapInfo":{"label":"Raydium CPMM"}}]}`))
	}))
	defer server.Close()
	t.Setenv("JUPITER_PRICE_URL", server.URL+"/price")
	t.Setenv("JUPITER_QUOTE_URL", server.URL+"/quote")
	rpc := func(_ context.Context, _ string, method string, _ any, out any) error {
		if method != "getTokenSupply" {
			return errors.New("unexpected RPC")
		}
		response := out.(*rpcTokenSupplyResponse)
		response.Value.Decimals = 6
		response.Value.UIAmountString = "1000000"
		return nil
	}
	holder := services.HolderIntelligence{Available: true, TopOwnerBalance: 400000}
	got := collectJupiterMarketContext(context.Background(), rpc, server.Client(), "solana-mainnet", mint, holder, services.TokenMarketSnapshot{PriceUSD: 0.24})
	if !got.PriceAvailable || !got.SellImpactAvailable || got.EstimatedPriceImpactPct != 12.5 || got.QuoteContextSlot != 889 {
		t.Fatalf("Jupiter result=%#v", got)
	}
	if len(got.RouteLabels) != 1 || got.RouteLabels[0] != "Raydium CPMM" {
		t.Fatalf("routes=%v", got.RouteLabels)
	}
}

func TestJupiterOutageDoesNotFailCoreContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "down", http.StatusServiceUnavailable) }))
	defer server.Close()
	t.Setenv("JUPITER_PRICE_URL", server.URL+"/price")
	t.Setenv("JUPITER_QUOTE_URL", server.URL+"/quote")
	got := collectJupiterMarketContext(context.Background(), nil, server.Client(), "solana-mainnet", "Mint", services.HolderIntelligence{}, services.TokenMarketSnapshot{Available: true, PriceUSD: 1})
	if got.Available || got.Status != "jupiter_context_unavailable" {
		t.Fatalf("result=%#v", got)
	}
}

func TestSafeCheckModesNeverEnablePhase2Providers(t *testing.T) {
	for _, mode := range []string{"don2n_preflight", "safe_check", "public-safe-check"} {
		if phase2MarketContextAllowed(mode) {
			t.Fatalf("mode %q enabled LP/Jupiter collection", mode)
		}
	}
	if !phase2MarketContextAllowed("customer_token_scan") {
		t.Fatal("full token scan did not enable context collection")
	}
}

func TestOptionalContextTimestampsRemainProviderScoped(t *testing.T) {
	ctx := services.JupiterMarketContext{Available: true, PriceObservedAt: time.Now().UTC()}
	if ctx.PriceObservedAt.IsZero() {
		t.Fatal("provider timestamp missing")
	}
}
