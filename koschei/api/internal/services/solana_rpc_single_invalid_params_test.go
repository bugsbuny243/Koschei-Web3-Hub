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

// singleInvalidParamsServer answers getTransaction singles: signatures listed
// in invalid get JSON-RPC -32602, every other signature gets a transaction.
func singleInvalidParamsServer(t *testing.T, calls *atomic.Int32, invalid map[string]bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode single RPC request: %v", err)
			return
		}
		params, _ := request.Params.([]any)
		signature := ""
		if len(params) > 0 {
			signature, _ = params[0].(string)
		}
		if invalid[signature] {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": request.ID,
				"error": map[string]any{"code": -32602, "message": "Invalid param: WrongSize"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{"slot": 42}})
	}))
}

func TestARVISSingleModeMalformedSignatureKeepsLaterEvidence(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "single")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "2")
	var calls atomic.Int32
	server := singleInvalidParamsServer(t, &calls, map[string]bool{"malformed": true})
	defer server.Close()

	result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"good-a", "malformed", "good-b", "good-c"})
	var partial *solanaRPCBatchPartialEvidenceError
	if !errors.As(err, &partial) || !strings.Contains(err.Error(), "-32602") {
		t.Fatalf("malformed signature must stay an explicit evidence gap: %v", err)
	}
	if len(result) != 3 || result["good-a"] == nil || result["good-b"] == nil || result["good-c"] == nil {
		t.Fatalf("later independent evidence was dropped: %#v", result)
	}
	if result["malformed"] != nil {
		t.Fatalf("rejected signature was promoted to evidence: %#v", result)
	}
	if got := calls.Load(); got != 4 {
		t.Fatalf("single mode stopped after one malformed signature: calls=%d want 4", got)
	}
}

func TestARVISSingleModeRepeatedInvalidParamsBeforeAnyAnswerStops(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "single")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "2")
	var calls atomic.Int32
	server := singleInvalidParamsServer(t, &calls, map[string]bool{"first": true, "second": true, "third": true, "fourth": true})
	defer server.Close()

	result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"first", "second", "third", "fourth"})
	if err == nil || len(result) != 0 {
		t.Fatalf("request-shape fault must fail closed: result=%#v err=%v", result, err)
	}
	var partial *solanaRPCBatchPartialEvidenceError
	if errors.As(err, &partial) {
		t.Fatalf("repeated invalid params with no answer must not permit continuation: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("request-shape fault was not bounded: calls=%d want 2", got)
	}
}

func TestARVISSingleModeProvenShapeCarriesAcrossOuterChunks(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "single")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "2")
	var calls atomic.Int32
	server := singleInvalidParamsServer(t, &calls, map[string]bool{"malformed-a": true, "malformed-b": true})
	defer server.Close()

	result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"good-a", "good-b", "malformed-a", "malformed-b", "good-c"})
	var partial *solanaRPCBatchPartialEvidenceError
	if !errors.As(err, &partial) {
		t.Fatalf("malformed signatures after a proven shape must stay partial: %v", err)
	}
	if len(result) != 3 || result["good-c"] == nil {
		t.Fatalf("later-chunk evidence was dropped: %#v", result)
	}
	if got := calls.Load(); got != 5 {
		t.Fatalf("outer chunk forgot the proven request shape: calls=%d want 5", got)
	}
}

func TestARVISSingleModeInvalidParamsGuardHoldsWithChunkSizeOne(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "single")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "1")
	var calls atomic.Int32
	server := singleInvalidParamsServer(t, &calls, map[string]bool{"first": true, "second": true, "third": true, "fourth": true})
	defer server.Close()

	// The first chunk's gap stays joined into the error; the second
	// unproven rejection is terminal, so later chunks are never requested.
	result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"first", "second", "third", "fourth"})
	if err == nil || !strings.Contains(err.Error(), "-32602") || len(result) != 0 {
		t.Fatalf("request-shape fault must fail closed across chunks: result=%#v err=%v", result, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("invalid-params guard did not hold across chunks of one: calls=%d want 2", got)
	}
}
