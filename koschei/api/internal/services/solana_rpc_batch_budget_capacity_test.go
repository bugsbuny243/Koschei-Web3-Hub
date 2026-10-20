package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func configureSolanaBatchBudgetTest(t *testing.T) {
	t.Helper()
	resetSolanaRPCCachesForTest()
	resetSolanaRPCBatchCircuitForTest()
	t.Setenv("APP_ENV", "test")
	t.Setenv("KOSCHEI_OWNER_UNLIMITED_AUTOSCAN_ENABLED", "")
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "true")
	t.Setenv("SOLANA_RPC_BUDGET_MAX_REQUESTS", "3")
	t.Setenv("SOLANA_RPC_BUDGET_WINDOW_SECONDS", "3600")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	t.Setenv("SOLANA_RPC_MAX_429_RETRIES", "0")
	t.Setenv("SOLANA_RPC_BATCH_ENABLED", "true")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "8")
	t.Cleanup(resetSolanaRPCCachesForTest)
}

func TestSolanaRPCBatchBudgetUsesAtomicMemberCount(t *testing.T) {
	configureSolanaBatchBudgetTest(t)
	if err := reserveSolanaRPCBudgetUnits(context.Background(), "getTransactionBatch", 2); err != nil {
		t.Fatal(err)
	}
	err := reserveSolanaRPCBudgetUnits(context.Background(), "getTransactionBatch", 2)
	var exceeded *solanaRPCBudgetExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("two-member batch must not fit one remaining unit: %v", err)
	}
	if err := reserveSolanaRPCBudgetUnits(context.Background(), "getTransactionBatch", 1); err != nil {
		t.Fatalf("failed reservation must not spend leftover unit: %v", err)
	}
	if err := reserveSolanaRPCBudget(context.Background(), "getTransaction"); err == nil {
		t.Fatal("budget must now be exhausted")
	}
}

func TestSolanaRPCBatchBudgetBlocksBeforeProviderRequest(t *testing.T) {
	configureSolanaBatchBudgetTest(t)
	t.Setenv("SOLANA_RPC_BUDGET_MAX_REQUESTS", "2")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	_, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"sig1", "sig2", "sig3"})
	var exceeded *solanaRPCBudgetExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("oversized batch should fail before RPC: %v", err)
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("oversized batch performed %d provider calls", n)
	}
}

func TestSolanaRPCBatchBudgetConcurrentReservationsNeverExceedLimit(t *testing.T) {
	configureSolanaBatchBudgetTest(t)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := reserveSolanaRPCBudgetUnits(context.Background(), "getTransactionBatch", 2); err == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := ok.Load(); got != 1 {
		t.Fatalf("concurrent batch reservations admitted %d batches, want 1", got)
	}
}

func TestSolanaRPCBatchBudgetPreservesInteractiveScans(t *testing.T) {
	configureSolanaBatchBudgetTest(t)
	ctx := WithInteractiveSolanaRPCBudget(context.Background())
	if err := reserveSolanaRPCBudgetUnits(ctx, "getTransactionBatch", 8); err != nil {
		t.Fatalf("foreground ARVIS scan must use its own per-scan bound: %v", err)
	}
	// Foreground exemption must not consume background window.
	if err := reserveSolanaRPCBudgetUnits(context.Background(), "getTransactionBatch", 3); err != nil {
		t.Fatalf("background budget unexpectedly reduced by foreground scan: %v", err)
	}
}

func TestSolanaRPCBatchBudgetRejectsInvalidUnits(t *testing.T) {
	configureSolanaBatchBudgetTest(t)
	for _, units := range []int{-1, 0, 21} {
		if err := reserveSolanaRPCBudgetUnits(context.Background(), "getTransactionBatch", units); err == nil {
			t.Fatalf("must reject %d reservation units", units)
		}
	}
}
