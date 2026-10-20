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
