package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func prepareSolanaRPCBatchTest(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	t.Setenv("SOLANA_RPC_MAX_429_RETRIES", "0")
	t.Setenv("SOLANA_RPC_BATCH_ENABLED", "true")
	resetSolanaRPCCachesForTest()
	resetSolanaRPCBatchCircuitForTest()
}

func TestSolanaRPCBatchForbiddenTripsCircuit(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"batch forbidden"}`))
	}))
	defer server.Close()

	_, err := SolanaGetTransactionsJSONParsedBatch(t.Context(), server.URL, []string{"sig-1", "sig-2"})
	if !IsSolanaRPCBatchUnavailable(err) {
		t.Fatalf("expected batch-unavailable error, got %v", err)
	}
	_, err = SolanaGetTransactionsJSONParsedBatch(t.Context(), server.URL, []string{"sig-3"})
	if !IsSolanaRPCBatchUnavailable(err) {
		t.Fatalf("expected open circuit on second call, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("open circuit sent repeated HTTP requests: calls=%d", got)
	}
}

func TestSolanaRPCBatchConcurrentForbiddenSendsOneRequest(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := SolanaGetTransactionsJSONParsedBatch(t.Context(), server.URL, []string{"sig"})
			if !IsSolanaRPCBatchUnavailable(err) {
				t.Errorf("expected batch-unavailable error, got %v", err)
			}
		}()
	}
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("concurrent workers should discover rejection once, calls=%d", got)
	}
}

func TestSolanaRPCBatchSplitsRequestEntityTooLarge(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "8")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var requests []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(requests) > 2 {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		responses := make([]map[string]any, 0, len(requests))
		for _, request := range requests {
			responses = append(responses, map[string]any{
				"jsonrpc": "2.0", "id": request.ID,
				"result": map[string]any{"slot": request.ID},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	out, err := SolanaGetTransactionsJSONParsedBatch(t.Context(), server.URL, []string{"sig-1", "sig-2", "sig-3", "sig-4"})
	if err != nil {
		t.Fatalf("split batch failed: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected all split results, got %d", len(out))
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected one rejected batch plus two halves, calls=%d", got)
	}
}

func TestSolanaRPCBatchPreservesSuccessfulChunksBeforeCircuit(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "2")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		var requests []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if call == 2 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		responses := make([]map[string]any, 0, len(requests))
		for _, request := range requests {
			responses = append(responses, map[string]any{
				"jsonrpc": "2.0", "id": request.ID,
				"result": map[string]any{"slot": request.ID},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	out, err := SolanaGetTransactionsJSONParsedBatch(t.Context(), server.URL, []string{"sig-1", "sig-2", "sig-3", "sig-4"})
	if !IsSolanaRPCBatchUnavailable(err) {
		t.Fatalf("expected second chunk to open circuit, got %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("successful first chunk was not preserved: %d", len(out))
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("unexpected call count: %d", got)
	}
}
