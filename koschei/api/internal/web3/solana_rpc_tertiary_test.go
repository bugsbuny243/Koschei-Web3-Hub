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

func TestSolanaRPCTriesThirdConfiguredProviderAfterTwoForbiddenResponses(t *testing.T) {
	var primaryReads, backupReads, thirdReads atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryReads.Add(1)
		http.Error(w, "blocked by provider", http.StatusForbidden)
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupReads.Add(1)
		http.Error(w, "blocked by backup", http.StatusForbidden)
	}))
	defer backup.Close()
	third := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		thirdReads.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"solana-core":"1.18"}}`)
	}))
	defer third.Close()

	t.Setenv("SOLANA_RPC_URL", primary.URL+"/?label=primary")
	t.Setenv("SOLANA_RPC_FALLBACK_URL", backup.URL+"/?label=backup")
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", third.URL+"/?label=tertiary")
	t.Setenv("HELIUS_SOLANA_RPC_URL", "")
	t.Setenv("QUICKNODE_SOLANA_RPC_URL", "")
	t.Setenv("ALCHEMY_API_KEY", "")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	rpc := &SolanaRPC{Client: &http.Client{Timeout: 3 * time.Second}, Cache: cache.NewNoop(), KeyPrefix: "third-provider"}
	for i := 0; i < 2; i++ {
		var response map[string]any
		if err := rpc.Call(context.Background(), "solana-mainnet", "getVersion", []any{}, &response, time.Second); err != nil {
			t.Fatalf("configured tertiary provider was not used: %v", err)
		}
		if response["solana-core"] != "1.18" {
			t.Fatalf("unexpected fallback response: %v", response)
		}
	}
	if primaryReads.Load() != 1 || backupReads.Load() != 1 || thirdReads.Load() != 2 {
		t.Fatalf("sticky 403 quarantine and tertiary fallback failed: primary=%d backup=%d third=%d", primaryReads.Load(), backupReads.Load(), thirdReads.Load())
	}
}

func TestSolanaRPCConfiguredEndpointsPreserveOrderedExplicitBackupAndNoPublicFallback(t *testing.T) {
	for _, key := range []string{
		"SOLANA_RPC_URL", "SOLANA_RPC_FALLBACK_URL", "ALCHEMY_SOLANA_RPC_URL",
		"HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL", "ALCHEMY_API_KEY",
	} {
		t.Setenv(key, "")
	}
	if got := solanaRPCConfiguredEndpoints("solana-mainnet", ""); len(got) != 0 {
		t.Fatalf("unconfigured RPC must remain closed: %v", got)
	}
	t.Setenv("SOLANA_RPC_URL", "https://first.example.test/rpc?label=first")
	t.Setenv("SOLANA_RPC_FALLBACK_URL", "https://backup.example.test/rpc?label=backup")
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", "https://third.example.test/rpc?label=third")
	got := solanaRPCConfiguredEndpoints("solana-mainnet", "")
	if len(got) < 3 {
		t.Fatalf("tertiary candidate missing after explicit fallback: %v", got)
	}
	if !strings.Contains(got[0], "first.example.test") || !strings.Contains(got[1], "backup.example.test") || !strings.Contains(got[2], "third.example.test") {
		t.Fatalf("configured RPC endpoint priority changed: %v", got)
	}
}
