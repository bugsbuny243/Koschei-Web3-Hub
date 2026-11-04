package web3

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/cache"
)

func guardTestRPC(t *testing.T, primary, backup string) *SolanaRPC {
	t.Helper()
	t.Setenv("SOLANA_RPC_URL", primary+"/?label=primary")
	t.Setenv("SOLANA_RPC_FALLBACK_URL", backup+"/?label=backup")
	for _, key := range []string{"ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL", "ALCHEMY_API_KEY"} {
		t.Setenv(key, "")
	}
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	t.Setenv("SOLANA_RPC_GOVERNOR_ENABLED", "false")
	t.Setenv("SOLANA_RPC_SIGNATURE_GUARD_ENABLED", "false")
	return &SolanaRPC{Client: &http.Client{Timeout: 3 * time.Second}, Cache: cache.NewNoop(), KeyPrefix: t.Name()}
}

// A caller that gives up is not a provider failure and is not logged as one.
func TestSolanaRPCCallerCancellationIsNotLoggedAsProviderFailure(t *testing.T) {
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer slow.Close()
	defer close(release)
	rpc := guardTestRPC(t, slow.URL, slow.URL)

	var buffer bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buffer)
	t.Cleanup(func() { log.SetOutput(previous) })

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	var out map[string]any
	if err := rpc.Call(ctx, "solana-mainnet", "getSlot", []any{}, &out, time.Second); err == nil {
		t.Fatal("a canceled call must return an error")
	}
	if strings.Contains(buffer.String(), "solana rpc failure") {
		t.Fatalf("caller cancellation was logged as a provider failure: %s", buffer.String())
	}
}
