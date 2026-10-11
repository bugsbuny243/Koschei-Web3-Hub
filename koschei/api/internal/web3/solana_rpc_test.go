package web3

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"koschei/api/internal/cache"
)

func TestCacheKeyIncludesMethod(t *testing.T) {
	rpc := &SolanaRPC{Cache: cache.NewMemory(), KeyPrefix: "test"}
	a := rpc.CacheKey("solana-mainnet", "getTransaction", []any{"sig"})
	b := rpc.CacheKey("solana-mainnet", "getTokenSupply", []any{"sig"})
	if a == b {
		t.Fatalf("cache keys should differ by method")
	}
}

func TestTTLForKnownMethods(t *testing.T) {
	if TTLFor("getTransaction", nil) != 24*time.Hour {
		t.Fatalf("unexpected tx ttl")
	}
	if TTLFor("getTokenSupply", nil) != time.Minute {
		t.Fatalf("unexpected supply ttl")
	}
	if TTLFor("getTokenLargestAccounts", nil) != 5*time.Minute {
		t.Fatalf("unexpected holders ttl")
	}
}

func TestSolanaRPCUsesConfiguredURL(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "https://rpc.example.test")
	got := SolanaRPCURL("solana-mainnet", "alchemy-key")
	if got != "https://rpc.example.test" {
		t.Fatalf("configured rpc URL should win, got %q", got)
	}
}

func TestSolanaRPCFallsBackAfterProviderRateLimit(t *testing.T) {
	var primaryCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		http.Error(w, "capacity exceeded", http.StatusTooManyRequests)
	}))
	defer primary.Close()

	var fallbackCalls atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"value":"ok"}}`)
	}))
	defer fallback.Close()

	t.Setenv("SOLANA_RPC_URL", primary.URL)
	t.Setenv("SOLANA_RPC_FALLBACK_URL", fallback.URL)
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	rpc := &SolanaRPC{Client: fallback.Client(), Cache: cache.NewNoop(), KeyPrefix: "test"}
	var out struct {
		Value string `json:"value"`
	}
	if err := rpc.Call(context.Background(), "solana-mainnet", "getVersion", []any{}, &out, time.Second); err != nil {
		t.Fatalf("expected fallback success, got %v", err)
	}
	if out.Value != "ok" {
		t.Fatalf("unexpected fallback result %q", out.Value)
	}
	if primaryCalls.Load() != 1 || fallbackCalls.Load() != 1 {
		t.Fatalf("expected one call per endpoint, primary=%d fallback=%d", primaryCalls.Load(), fallbackCalls.Load())
	}
}

func TestSolanaRPCCoolsDownRateLimitedPrimary(t *testing.T) {
	var primaryCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		w.Header().Set("Retry-After", "1")
		http.Error(w, "capacity exceeded", http.StatusTooManyRequests)
	}))
	defer primary.Close()

	var fallbackCalls atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"value":"fallback-ok"}}`)
	}))
	defer fallback.Close()

	t.Setenv("SOLANA_RPC_URL", primary.URL)
	t.Setenv("SOLANA_RPC_FALLBACK_URL", fallback.URL)
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	t.Setenv("SOLANA_RPC_429_COOLDOWN_SECONDS", "30")
	rpc := &SolanaRPC{Client: fallback.Client(), Cache: cache.NewNoop(), KeyPrefix: "test"}

	for i := 0; i < 2; i++ {
		var out struct {
			Value string `json:"value"`
		}
		if err := rpc.Call(context.Background(), "solana-mainnet", "getVersion", []any{i}, &out, time.Second); err != nil {
			t.Fatalf("call %d expected fallback success, got %v", i, err)
		}
		if out.Value != "fallback-ok" {
			t.Fatalf("call %d unexpected fallback result %q", i, out.Value)
		}
	}
	if primaryCalls.Load() != 1 {
		t.Fatalf("rate-limited primary should be cooled down, calls=%d", primaryCalls.Load())
	}
	if fallbackCalls.Load() != 2 {
		t.Fatalf("fallback calls=%d want=2", fallbackCalls.Load())
	}
}

func TestSolanaRPCFallsBackAfterEndpointTimeout(t *testing.T) {
	var primaryCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		time.Sleep(200 * time.Millisecond)
		http.Error(w, "late primary", http.StatusGatewayTimeout)
	}))
	defer primary.Close()

	var fallbackCalls atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"value":"fallback-ok"}}`)
	}))
	defer fallback.Close()

	t.Setenv("SOLANA_RPC_URL", primary.URL)
	t.Setenv("SOLANA_RPC_FALLBACK_URL", fallback.URL)
	t.Setenv("SOLANA_RPC_ENDPOINT_TIMEOUT_MS", "75")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	rpc := &SolanaRPC{Client: fallback.Client(), Cache: cache.NewNoop(), KeyPrefix: "test"}
	var out struct {
		Value string `json:"value"`
	}
	started := time.Now()
	if err := rpc.Call(context.Background(), "solana-mainnet", "getVersion", []any{}, &out, time.Second); err != nil {
		t.Fatalf("expected timeout fallback success, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("fallback took too long: %s", elapsed)
	}
	if out.Value != "fallback-ok" || primaryCalls.Load() != 1 || fallbackCalls.Load() != 1 {
		t.Fatalf("unexpected fallback result=%q primary=%d fallback=%d", out.Value, primaryCalls.Load(), fallbackCalls.Load())
	}
}

func TestSolanaRPCSharesParentDeadlineAcrossFailoverEndpoints(t *testing.T) {
	var primaryCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		time.Sleep(250 * time.Millisecond)
		http.Error(w, "late primary", http.StatusGatewayTimeout)
	}))
	defer primary.Close()

	var fallbackCalls atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		time.Sleep(60 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"value":"fair-fallback"}}`)
	}))
	defer fallback.Close()

	t.Setenv("SOLANA_RPC_URL", primary.URL)
	t.Setenv("SOLANA_RPC_FALLBACK_URL", fallback.URL)
	t.Setenv("SOLANA_RPC_ENDPOINT_TIMEOUT_MS", "180")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	rpc := &SolanaRPC{Client: fallback.Client(), Cache: cache.NewNoop(), KeyPrefix: "test"}
	ctx, cancel := context.WithTimeout(context.Background(), 220*time.Millisecond)
	defer cancel()
	var out struct {
		Value string `json:"value"`
	}
	if err := rpc.Call(ctx, "solana-mainnet", "getVersion", []any{"deadline-share"}, &out, time.Second); err != nil {
		t.Fatalf("expected fallback to retain deadline budget, got %v", err)
	}
	if out.Value != "fair-fallback" || primaryCalls.Load() != 1 || fallbackCalls.Load() != 1 {
		t.Fatalf("unexpected result=%q primary=%d fallback=%d", out.Value, primaryCalls.Load(), fallbackCalls.Load())
	}
}

func TestSolanaRPCMinIntervalConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "")
	if got := solanaRPCMinInterval(); got != 500*time.Millisecond {
		t.Fatalf("production default min interval=%s", got)
	}
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "125")
	if got := solanaRPCMinInterval(); got != 125*time.Millisecond {
		t.Fatalf("configured min interval=%s", got)
	}
}

func TestUniqueRPCURLsRemovesDuplicates(t *testing.T) {
	got := uniqueRPCURLs("https://rpc.test", " https://rpc.test ", "", "https://fallback.test")
	if len(got) != 2 {
		t.Fatalf("expected two unique endpoints, got %v", got)
	}
}

func TestSolanaRPCFallbackPrefersDifferentConfiguredProvider(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "https://mainnet.helius-rpc.com/?api-key=primary-secret")
	t.Setenv("SOLANA_RPC_FALLBACK_URL", "")
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", "https://solana-mainnet.g.alchemy.com/v2/fallback-secret")
	t.Setenv("HELIUS_SOLANA_RPC_URL", "")
	t.Setenv("QUICKNODE_SOLANA_RPC_URL", "")
	t.Setenv("ALCHEMY_API_KEY", "")
	got := SolanaRPCFallbackURL("solana-mainnet")
	if RPCProviderHost(got) != "solana-mainnet.g.alchemy.com" {
		t.Fatalf("fallback host=%q url=%q", RPCProviderHost(got), got)
	}
}

func TestSolanaRPCBlankNetworkUsesSOLANANetworkAndProviderPreference(t *testing.T) {
	t.Setenv("SOLANA_NETWORK", "mainnet")
	t.Setenv("SOLANA_RPC_URL", "")
	t.Setenv("WEB3_PROVIDER", "helius")
	t.Setenv("KOSCHEI_SECURITY_PROVIDER", "auto")
	t.Setenv("HELIUS_SOLANA_RPC_URL", "https://helius.example.test")
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", "https://alchemy.example.test")
	t.Setenv("QUICKNODE_SOLANA_RPC_URL", "https://quicknode.example.test")
	if got := SolanaRPCURL("", ""); got != "https://helius.example.test" {
		t.Fatalf("blank network/provider preference got %q", got)
	}
}

func TestSecurityProviderDrivesRPCWhenWeb3ProviderAuto(t *testing.T) {
	t.Setenv("SOLANA_NETWORK", "mainnet")
	t.Setenv("SOLANA_RPC_URL", "")
	t.Setenv("WEB3_PROVIDER", "auto")
	t.Setenv("KOSCHEI_SECURITY_PROVIDER", "quicknode")
	t.Setenv("QUICKNODE_SOLANA_RPC_URL", "https://quicknode.example.test")
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", "https://alchemy.example.test")
	if got := SolanaRPCURL("", ""); got != "https://quicknode.example.test" {
		t.Fatalf("security provider preference got %q", got)
	}
}

func TestBoundedCooldownRetryRequiresRemainingRequestBudget(t *testing.T) {
	ctxShort, cancelShort := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancelShort()
	if waitForBoundedSolanaRPCCooldown(ctxShort, time.Now().Add(100*time.Millisecond)) {
		t.Fatal("cooldown wait must not consume a request budget shorter than the cooldown")
	}

	ctxLong, cancelLong := context.WithTimeout(t.Context(), time.Second)
	defer cancelLong()
	if !waitForBoundedSolanaRPCCooldown(ctxLong, time.Now().Add(5*time.Millisecond)) {
		t.Fatal("bounded cooldown shorter than remaining request budget should be retried once")
	}
	retryCtx := markSolanaRPCCooldownRetryUsed(ctxLong)
	if !solanaRPCCooldownRetryUsed(retryCtx) {
		t.Fatal("retry marker was not preserved")
	}
}

func TestSolanaRPCUnconfiguredFailsClosed(t *testing.T) {
	for _, key := range strings.Fields(
		"SOLANA_RPC_URL SOLANA_RPC_FALLBACK_URL ALCHEMY_SOLANA_RPC_URL " +
			"HELIUS_SOLANA_RPC_URL QUICKNODE_SOLANA_RPC_URL ALCHEMY_API_KEY " +
			"SOLANA_DEVNET_RPC_URL ALCHEMY_SOLANA_DEVNET_RPC_URL SOLANA_DEVNET_RPC_FALLBACK_URL",
	) {
		t.Setenv(key, "")
	}
	if got := SolanaRPCURL("solana-mainnet", ""); got != "" {
		t.Fatalf("unconfigured mainnet must not use public RPC, got %q", got)
	}
	if got := SolanaRPCURL("solana-devnet", ""); got != "" {
		t.Fatalf("unconfigured devnet must not use public RPC, got %q", got)
	}
	rpc := &SolanaRPC{Client: http.DefaultClient, Cache: cache.NewNoop(), KeyPrefix: "test"}
	var out any
	if err := rpc.Call(context.Background(), "solana-mainnet", "getVersion", []any{}, &out, time.Second); err == nil || !strings.Contains(err.Error(), "provider_not_configured") {
		t.Fatalf("expected provider_not_configured, got %v", err)
	}
}

func TestSolanaRPCUnconfiguredRejectsCachedEvidence(t *testing.T) {
	for _, key := range strings.Fields(
		"SOLANA_RPC_URL SOLANA_RPC_FALLBACK_URL ALCHEMY_SOLANA_RPC_URL " +
			"HELIUS_SOLANA_RPC_URL QUICKNODE_SOLANA_RPC_URL ALCHEMY_API_KEY",
	) {
		t.Setenv(key, "")
	}
	rpcCache := cache.NewMemory()
	rpc := &SolanaRPC{Client: http.DefaultClient, Cache: rpcCache, KeyPrefix: "test"}
	params := []any{"cached-signature"}
	key := rpc.CacheKey("solana-mainnet", "getTransaction", params)
	if err := rpcCache.SetJSON(context.Background(), key, map[string]any{"cached": true}, time.Hour); err != nil {
		t.Fatalf("seed RPC cache: %v", err)
	}
	var out map[string]any
	if err := rpc.Call(context.Background(), "solana-mainnet", "getTransaction", params, &out, time.Hour); err == nil || !strings.Contains(err.Error(), "provider_not_configured") {
		t.Fatalf("cached response bypassed provider validation: %v", err)
	}
}
