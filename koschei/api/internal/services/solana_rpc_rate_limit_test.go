package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type rpcAttemptCounter struct {
	sync.Mutex
	value int
}

func (c *rpcAttemptCounter) increment() int {
	c.Lock()
	defer c.Unlock()
	c.value++
	return c.value
}

func (c *rpcAttemptCounter) count() int {
	c.Lock()
	defer c.Unlock()
	return c.value
}

func TestSolanaRPC429RetriesThenSucceeds(t *testing.T) {
	resetSolanaRPCCachesForTest()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	t.Setenv("SOLANA_RPC_MAX_429_RETRIES", "2")
	t.Setenv("SOLANA_RPC_429_BACKOFF_MS", "1")

	attempts := &rpcAttemptCounter{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.increment() == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":429,"message":"compute capacity exceeded"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[{"signature":"sig","slot":1,"err":null,"blockTime":1}]}`))
	}))
	defer server.Close()

	result, err := SolanaGetSignaturesForAddress(context.Background(), server.URL, "address", 1)
	if err != nil {
		t.Fatalf("request failed after retry: %v", err)
	}
	if len(result) != 1 || result[0].Signature != "sig" {
		t.Fatalf("unexpected retry result: %#v", result)
	}
	if got := attempts.count(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestSolanaRPC429StopsAtConfiguredRetryLimit(t *testing.T) {
	resetSolanaRPCCachesForTest()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	t.Setenv("SOLANA_RPC_MAX_429_RETRIES", "1")
	t.Setenv("SOLANA_RPC_429_BACKOFF_MS", "1")

	attempts := &rpcAttemptCounter{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.increment()
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":429,"message":"compute capacity exceeded"}}`))
	}))
	defer server.Close()

	_, err := SolanaGetSignaturesForAddress(context.Background(), server.URL, "address", 1)
	if err == nil || !strings.Contains(err.Error(), "http status 429") {
		t.Fatalf("expected final 429 error, got %v", err)
	}
	if got := attempts.count(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestProductionSolanaRPCDefaultPacing(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "")
	if got := solanaRPCMinInterval(); got != 250*time.Millisecond {
		t.Fatalf("default production interval = %s, want 250ms", got)
	}
}

func TestSolanaRPCPacingCanBeOverridden(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "80")
	if got := solanaRPCMinInterval(); got != 80*time.Millisecond {
		t.Fatalf("configured interval = %s, want 80ms", got)
	}
}
