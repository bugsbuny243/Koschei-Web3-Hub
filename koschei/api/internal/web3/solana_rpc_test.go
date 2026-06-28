package web3

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestUniqueRPCURLsRemovesDuplicates(t *testing.T) {
	got := uniqueRPCURLs("https://rpc.test", " https://rpc.test ", "", "https://fallback.test")
	if len(got) != 2 {
		t.Fatalf("expected two unique endpoints, got %v", got)
	}
}
