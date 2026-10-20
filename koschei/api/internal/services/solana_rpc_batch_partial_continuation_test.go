package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestARVISBatchContinuesIndependentSubchunksAfterMemberError(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "8")
	t.Setenv("SOLANA_RPC_BATCH_CHUNK_SIZE", "1")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var batch []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		params := batch[0].Params.([]any)
		if params[0] == "unsupported" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"jsonrpc": "2.0", "id": batch[0].ID, "error": map[string]any{
					"code": -32015, "message": "Transaction version (1) is not supported",
				}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"jsonrpc": "2.0", "id": batch[0].ID, "result": map[string]any{"slot": 123}},
		})
	}))
	defer server.Close()

	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"unsupported", "good-a", "good-b"})
	if err == nil || !strings.Contains(err.Error(), "first_rpc_code=-32015") {
		t.Fatalf("member-level failure must remain visible: %v", err)
	}
	if len(out) != 2 || out["good-a"] == nil || out["good-b"] == nil || out["unsupported"] != nil {
		t.Fatalf("independent subchunks lost or unsupported member trusted: %#v", out)
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("member-level error prematurely aborted subchunks: provider calls=%d", got)
	}
}

func TestARVISBatchContinuesIndependentOuterChunksAfterMemberError(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "2")
	t.Setenv("SOLANA_RPC_BATCH_CHUNK_SIZE", "2")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var batch []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		responses := make([]map[string]any, 0, len(batch))
		for _, item := range batch {
			params := item.Params.([]any)
			if params[0] == "bad-v1" {
				responses = append(responses, map[string]any{
					"jsonrpc": "2.0", "id": item.ID,
					"error": map[string]any{"code": -32015, "message": "Transaction version (1) is not supported"},
				})
			} else {
				responses = append(responses, map[string]any{
					"jsonrpc": "2.0", "id": item.ID, "result": map[string]any{"slot": 123},
				})
			}
		}
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"bad-v1", "good-a", "good-b", "good-c"})
	if err == nil || !strings.Contains(err.Error(), "-32015") {
		t.Fatalf("partial evidence must remain explicit: %v", err)
	}
	if len(out) != 3 || out["good-a"] == nil || out["good-b"] == nil || out["good-c"] == nil {
		t.Fatalf("later independent chunk evidence missing: %#v", out)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("member error must not stop next bounded chunk: got %d calls", got)
	}
}

func TestARVISBatchStopsOnProviderWideRateLimit(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "1")
	t.Setenv("SOLANA_RPC_429_COOLDOWN_SECONDS", "30")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"provider rate limited"}`))
	}))
	defer server.Close()

	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"first", "second"})
	if err == nil || !strings.Contains(err.Error(), "429") || len(out) != 0 {
		t.Fatalf("provider-wide rate limit must fail closed without evidence: out=%#v err=%v", out, err)
	}
	var partial *solanaRPCBatchPartialEvidenceError
	if errors.As(err, &partial) {
		t.Fatal("provider-wide 429 must not be classified as recoverable member error")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("429 must stop later chunks to avoid a request storm: requests=%d", got)
	}
}

func TestARVISBatchStopsOnJSONRPCMemberRateLimitInHTTP200(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "2")
	t.Setenv("SOLANA_RPC_429_COOLDOWN_SECONDS", "30")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var batch []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"jsonrpc": "2.0", "id": batch[0].ID, "result": map[string]any{"slot": 123}},
			{"jsonrpc": "2.0", "id": batch[1].ID, "error": map[string]any{"code": 429, "message": "compute capacity exceeded"}},
		})
	}))
	defer server.Close()

	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"verified", "rate-limited", "must-not-request"})
	if err == nil || !strings.Contains(err.Error(), "provider rate limited") {
		t.Fatalf("HTTP 200 member code 429 must throttle the provider: err=%v", err)
	}
	var partial *solanaRPCBatchPartialEvidenceError
	if errors.As(err, &partial) {
		t.Fatalf("provider rate-limit incorrectly treated as recoverable partial evidence: %v", err)
	}
	if len(out) != 1 || out["verified"] == nil {
		t.Fatalf("independent successful evidence must remain available: %#v", out)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider throttling should stop remaining chunks: calls=%d", got)
	}
}

func TestARVISBatch413SplitContinuesAfterOneRejectedMember(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "auto")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "4")
	t.Setenv("SOLANA_RPC_BATCH_CHUNK_SIZE", "4")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var batch []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if len(batch) > 2 {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		responses := make([]map[string]any, 0, len(batch))
		for _, item := range batch {
			sig := item.Params.([]any)[0]
			if sig == "rejected-v1" {
				responses = append(responses, map[string]any{
					"jsonrpc": "2.0", "id": item.ID,
					"error": map[string]any{"code": -32015, "message": "Transaction version (1) is not supported"},
				})
			} else {
				responses = append(responses, map[string]any{
					"jsonrpc": "2.0", "id": item.ID, "result": map[string]any{"slot": 123},
				})
			}
		}
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{
		"rejected-v1", "good-a", "good-b", "good-c",
	})
	if err == nil || !strings.Contains(err.Error(), "-32015") {
		t.Fatalf("split-member error must remain visible: %v", err)
	}
	if len(out) != 3 || out["good-a"] == nil || out["good-b"] == nil || out["good-c"] == nil {
		t.Fatalf("413 split lost independent verified members: %#v", out)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected one 413 and two partial/successful half-batches: calls=%d", got)
	}
}
