package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestARVISIntegratedBatchReusesUnsupportedVersionEvidenceWithoutExtraRPC(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var requests []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		respond := make([]map[string]any, 0, len(requests))
		for _, req := range requests {
			params := req.Params.([]any)
			if params[0] == "unsupported-v1" {
				respond = append(respond, map[string]any{
					"jsonrpc": "2.0", "id": req.ID,
					"error": map[string]any{"code": -32015, "message": "Transaction version (1) is not supported by the requesting client"},
				})
			} else {
				respond = append(respond, map[string]any{
					"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"slot": 123},
				})
			}
		}
		_ = json.NewEncoder(w).Encode(respond)
	}))
	defer server.Close()

	ctx := context.Background()
	first, err := SolanaGetTransactionsJSONParsedBatch(ctx, server.URL, []string{"verified", "unsupported-v1"})
	if err == nil || !strings.Contains(err.Error(), "-32015") || len(first) != 1 || first["verified"] == nil {
		t.Fatalf("first call must preserve valid proof and reject unsupported: records=%v err=%v", first, err)
	}
	second, err := SolanaGetTransactionsJSONParsedBatch(ctx, server.URL, []string{"verified", "unsupported-v1"})
	if err == nil || !strings.Contains(err.Error(), "-32015") || len(second) != 1 || second["verified"] == nil {
		t.Fatalf("second call must preserve proof and cached failure: records=%v err=%v", second, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected one batch with two members and one batch excluding unsupported, HTTP calls=%d", got)
	}
	onlyUnsupported, err := SolanaGetTransactionsJSONParsedBatch(ctx, server.URL, []string{"unsupported-v1"})
	if err == nil || !strings.Contains(err.Error(), "-32015") || len(onlyUnsupported) != 0 {
		t.Fatalf("cached-only request must fail closed with no records: %v %v", onlyUnsupported, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("cached-only request unnecessarily contacted provider: %d", got)
	}
}

func TestARVISIntegratedBatchRejectsMissingResponseEvenWhenAnotherMemberIsCached(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Once the unsupported signature is cached, the provider silently
		// returns an empty JSON-RPC batch for the other signature.
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	cacheSolanaUnsupportedTransactionVersion(solanaTransactionFetchKey(server.URL, "unsupported-v1"))
	result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"unsupported-v1", "no-response"})
	if err == nil || !strings.Contains(err.Error(), "missing_responses=1") || !strings.Contains(err.Error(), "-32015") {
		t.Fatalf("cached unsupported and missing member must both fail closed: result=%v err=%v", result, err)
	}
	if len(result) != 0 {
		t.Fatalf("missing evidence appeared verified: %v", result)
	}
}

func TestARVISBatchDuplicateIDCannotPoisonUnsupportedVersionCache(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		var requests []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			t.Errorf("decode batch: %v", err)
			return
		}
		if call == 1 {
			// A provider that uses duplicate IDs is ambiguous, even when one
			// envelope claims a deterministic -32015 version error.
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"jsonrpc": "2.0", "id": 1, "error": map[string]any{
					"code": -32015, "message": "Transaction version (1) is not supported by the requesting client",
				}},
				{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"slot": 123}},
				{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"slot": 456}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"slot": 789}},
		})
	}))
	defer server.Close()

	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"ambiguous", "independent"})
	if err == nil || !strings.Contains(err.Error(), "duplicate_ids=1") || len(out) != 1 || out["independent"] == nil {
		t.Fatalf("duplicate response must preserve only independent evidence: %#v err=%v", out, err)
	}
	if cachedSolanaUnsupportedTransactionVersion(solanaTransactionFetchKey(server.URL, "ambiguous")) {
		t.Fatal("ambiguous -32015 response poisoned negative cache")
	}
	out, err = SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"ambiguous"})
	if err != nil || len(out) != 1 || out["ambiguous"] == nil {
		t.Fatalf("unpoisoned signature must be independently retrievable: %#v err=%v", out, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("ambiguous transaction must be retried: calls=%d", got)
	}
}
