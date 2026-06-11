package web3

import (
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
