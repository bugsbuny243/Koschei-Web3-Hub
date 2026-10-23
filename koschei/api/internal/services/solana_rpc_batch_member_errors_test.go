package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSolanaRPCBatchReportsMemberFailuresWithoutDiscardingEvidence(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requests []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			t.Errorf("decode batch: %v", err)
			return
		}
		responses := make([]map[string]any, 0, len(requests))
		for i, request := range requests {
			if i == 0 {
				responses = append(responses, map[string]any{
					"jsonrpc": "2.0", "id": request.ID,
					"result": map[string]any{"slot": 123},
				})
			} else {
				responses = append(responses, map[string]any{
					"jsonrpc": "2.0", "id": request.ID,
					"error": map[string]any{"code": -32015, "message": "unsupported transaction version"},
				})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"supported", "unsupported"})
	if err == nil || !strings.Contains(err.Error(), "first_rpc_code=-32015") {
		t.Fatalf("per-member provider error must reach caller: %v", err)
	}
	if len(out) != 1 || out["supported"] == nil || out["unsupported"] != nil {
		t.Fatalf("batch must keep verified successful member and omit rejected member: %#v", out)
	}
}

func TestSolanaRPCBatchReportsAllMembersRejected(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"jsonrpc": "2.0", "id": 1, "error": map[string]any{"code": -32015, "message": "unsupported version"}},
			{"jsonrpc": "2.0", "id": 2, "error": map[string]any{"code": -32015, "message": "unsupported version"}},
		})
	}))
	defer server.Close()
	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"a", "b"})
	if err == nil || !strings.Contains(err.Error(), "2/2 transaction") || len(out) != 0 {
		t.Fatalf("all rejected batch must be incomplete: out=%#v err=%v", out, err)
	}
}

func TestSolanaRPCBatchReportsLaterChunkFailureAfterEarlierSuccess(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requests []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			t.Errorf("decode batch: %v", err)
			return
		}
		params := requests[0].Params.([]any)
		signature := params[0].(string)
		if signature == "first" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"slot": 123}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"jsonrpc": "2.0", "id": 1, "error": map[string]any{"code": -32015, "message": "unsupported version"}},
		})
	}))
	defer server.Close()
	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"first", "second"})
	if err == nil || len(out) != 1 || out["first"] == nil {
		t.Fatalf("later chunk failure must preserve evidence and error: out=%#v err=%v", out, err)
	}
}

func TestSolanaRPCSingleFallbackReportsPartialFailures(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "single")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode single: %v", err)
			return
		}
		params := request.Params.([]any)
		if params[0] == "first" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{"slot": 123},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": request.ID,
			"error": map[string]any{"code": -32015, "message": "unsupported version"},
		})
	}))
	defer server.Close()
	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"first", "second"})
	if err == nil || len(out) != 1 || out["first"] == nil {
		t.Fatalf("single fallback partial failure must not disappear: out=%#v err=%v", out, err)
	}
}

func TestSolanaRPCBatchMissingEnvelopeReportsIncompleteEvidence(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"slot": 123}},
		})
	}))
	defer server.Close()
	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"first", "missing"})
	if err == nil || !strings.Contains(err.Error(), "missing_responses=1") {
		t.Fatalf("missing response envelope must be reported: %v", err)
	}
	if len(out) != 1 || out["first"] == nil || out["missing"] != nil {
		t.Fatalf("must retain only unambiguous evidence: %#v", out)
	}
}

func TestSolanaRPCBatchDuplicateEnvelopeDiscardsAmbiguousMember(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"slot": 123}},
			{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"slot": 999}},
			{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"slot": 456}},
		})
	}))
	defer server.Close()
	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"ambiguous", "valid"})
	if err == nil || !strings.Contains(err.Error(), "duplicate_ids=1") {
		t.Fatalf("duplicate response envelope must be reported: %v", err)
	}
	if len(out) != 1 || out["valid"] == nil || out["ambiguous"] != nil {
		t.Fatalf("duplicate member was treated as trusted evidence: %#v", out)
	}
}

func TestSolanaRPCBatchNullResultNeverCountsAsVerifiedEvidence(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"slot": 123}},
			{"jsonrpc": "2.0", "id": 2, "result": nil},
		})
	}))
	defer server.Close()
	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"verified", "unavailable"})
	if err == nil || !strings.Contains(err.Error(), "null_results=1") {
		t.Fatalf("null transaction evidence must be explicitly incomplete: %v", err)
	}
	if len(out) != 1 || out["verified"] == nil || out["unavailable"] != nil {
		t.Fatalf("must preserve only verified evidence: %#v", out)
	}
}

func TestSolanaRPCSingleFallbackNullResultReportsMissingEvidence(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "single")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		params := request.Params.([]any)
		if params[0] == "verified" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{"slot": 123},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": nil})
	}))
	defer server.Close()
	out, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"verified", "unavailable"})
	if err == nil || !strings.Contains(err.Error(), "evidence unavailable") {
		t.Fatalf("null result from single fallback must fail closed: %v", err)
	}
	if len(out) != 1 || out["verified"] == nil || out["unavailable"] != nil {
		t.Fatalf("single fallback must keep only verified evidence: %#v", out)
	}
}
