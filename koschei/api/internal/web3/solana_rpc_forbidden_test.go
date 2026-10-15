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

func TestSolanaRPCForbiddenProviderQuarantinedAndConfigReset(t *testing.T) {
	var primaryCalls, backupCalls, replacementCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"solana-core":"1.18"}}`)
	}))
	defer backup.Close()
	replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		replacementCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"solana-core":"1.19"}}`)
	}))
	defer replacement.Close()
	primaryURL := primary.URL + "/?api-key=very-secret-primary-key"
	t.Setenv("SOLANA_RPC_URL", primaryURL)
	t.Setenv("SOLANA_RPC_FALLBACK_URL", backup.URL)
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	rpc := &SolanaRPC{Client: &http.Client{Timeout: 3 * time.Second}, Cache: cache.NewNoop(), KeyPrefix: "forbidden-test"}
	for i := 0; i < 3; i++ {
		var result map[string]any
		if err := rpc.Call(context.Background(), "solana-mainnet", "getVersion", []any{}, &result, time.Second); err != nil {
			t.Fatalf("fallback request %d: %v", i, err)
		}
		if result["solana-core"] != "1.18" {
			t.Fatalf("unexpected fallback result: %v", result)
		}
	}
	if primaryCalls.Load() != 1 || backupCalls.Load() != 3 {
		t.Fatalf("forbidden quarantine failed: primary=%d backup=%d", primaryCalls.Load(), backupCalls.Load())
	}
	if !rpc.rpcEndpointForbidden(primaryURL, []string{primaryURL, backup.URL}) {
		t.Fatal("403 provider was not persistently quarantined")
	}

	t.Setenv("SOLANA_RPC_URL", replacement.URL)
	var result map[string]any
	if err := rpc.Call(context.Background(), "solana-mainnet", "getVersion", []any{}, &result, time.Second); err != nil {
		t.Fatalf("new configuration did not reset quarantine: %v", err)
	}
	if replacementCalls.Load() != 1 || result["solana-core"] != "1.19" {
		t.Fatalf("new provider not used after configuration change: count=%d result=%v", replacementCalls.Load(), result)
	}
}

func TestSolanaRPCForbiddenErrorNeverLeaksCredential(t *testing.T) {
	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer forbidden.Close()
	t.Setenv("SOLANA_RPC_URL", forbidden.URL+"/?api-key=do-not-print-this-secret")
	t.Setenv("SOLANA_RPC_FALLBACK_URL", "")
	t.Setenv("ALCHEMY_API_KEY", "")
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", "")
	t.Setenv("HELIUS_SOLANA_RPC_URL", "")
	t.Setenv("QUICKNODE_SOLANA_RPC_URL", "")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	rpc := &SolanaRPC{Client: &http.Client{Timeout: 2 * time.Second}, Cache: cache.NewNoop(), KeyPrefix: "forbidden-test"}
	var result map[string]any
	err := rpc.Call(context.Background(), "solana-mainnet", "getVersion", []any{}, &result, time.Second)
	if err == nil || !strings.Contains(err.Error(), "rpc status 403") {
		t.Fatalf("expected safe 403 status, got %v", err)
	}
	if strings.Contains(err.Error(), "do-not-print-this-secret") {
		t.Fatal("RPC credential leaked in error")
	}
}
