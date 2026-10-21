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

func TestARVISSingleModePreservesLaterChunkAfterPerSignatureEvidenceGaps(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "single")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "2")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode single RPC: %v", err)
			return
		}
		if req.Method != "getTransaction" {
			t.Errorf("unexpected method %q", req.Method)
			return
		}
		signature := req.Params.([]any)[0]
		response := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		switch signature {
		case "unsupported-v1":
			response["error"] = map[string]any{"code": -32015, "message": "Transaction version (1) is not supported"}
		case "missing-history":
			response["result"] = nil
		default:
			response["result"] = map[string]any{"slot": 42}
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{
		"unsupported-v1", "missing-history", "verified-later-a", "verified-later-b",
	})
	var partial *solanaRPCBatchPartialEvidenceError
	if !errors.As(err, &partial) {
		t.Fatalf("single mode must report partial evidence: err=%v", err)
	}
	if len(result) != 2 || result["verified-later-a"] == nil || result["verified-later-b"] == nil {
		t.Fatalf("later verified chunks must survive single-signature gaps: %#v", result)
	}
	if result["unsupported-v1"] != nil || result["missing-history"] != nil {
		t.Fatalf("unavailable signatures were promoted to evidence: %#v", result)
	}
	if got := calls.Load(); got != 4 {
		t.Fatalf("single mode stopped before later chunk: calls=%d want 4", got)
	}
}

func TestARVISBatchStopsOnRequestWideMemberErrorButKeepsSiblingEvidence(t *testing.T) {
	for _, code := range []int{-32600, -32601, -32602, -32603, -32005} {
		t.Run(strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(
			map[int]string{-32600: "invalid-request", -32601: "method-not-found", -32602: "invalid-params", -32603: "internal-error", -32005: "node-behind"}[code],
		), " "), " ", "-"), func(t *testing.T) {
			prepareSolanaRPCBatchTest(t)
			t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
			t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "2")
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var batch []solanaRPCRequest
				if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
					t.Errorf("decode RPC batch: %v", err)
					return
				}
				if len(batch) != 2 {
					t.Errorf("unexpected batch size %d", len(batch))
					return
				}
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"jsonrpc": "2.0", "id": batch[0].ID, "error": map[string]any{"code": code, "message": "provider request unavailable"}},
					{"jsonrpc": "2.0", "id": batch[1].ID, "result": map[string]any{"slot": 42}},
				})
			}))
			defer server.Close()

			result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"bad", "good", "do-not-fetch"})
			if err == nil || !strings.Contains(err.Error(), "request-wide failure") {
				t.Fatalf("request-wide RPC error %d must be terminal: %v", code, err)
			}
			var partial *solanaRPCBatchPartialEvidenceError
			if errors.As(err, &partial) {
				t.Fatalf("request-wide error must not permit continuation: %v", err)
			}
			if len(result) != 1 || result["good"] == nil {
				t.Fatalf("independent sibling evidence lost: %#v", result)
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("request-wide failure sent later batch: calls=%d", got)
			}
		})
	}
}

func TestARVISSingleModeProviderRateLimitStopsLaterSignatures(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "single")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "2")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"error": map[string]any{"code": 429, "message": "provider over capacity"},
		})
	}))
	defer server.Close()

	result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"rate-limited", "must-not-fetch", "later-chunk"})
	if err == nil || !strings.Contains(err.Error(), "429") || len(result) != 0 {
		t.Fatalf("rate limited singles must fail closed: result=%#v err=%v", result, err)
	}
	var partial *solanaRPCBatchPartialEvidenceError
	if errors.As(err, &partial) {
		t.Fatalf("rate limit cannot be classified as recoverable partial evidence: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("rate limit amplified provider traffic: %d calls", got)
	}
}
