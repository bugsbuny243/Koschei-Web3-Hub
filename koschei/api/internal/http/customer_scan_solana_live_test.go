package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/cache"
	"koschei/api/internal/services"
	"koschei/api/internal/web3"
)

func TestCustomerScanEndpointReturnsObservedSolanaAccountEvidence(t *testing.T) {
	const address = "11111111111111111111111111111111"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Method  string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Method != "getAccountInfo" {
			t.Fatalf("unexpected method %q", req.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"context": map[string]any{"slot": 345678901},
				"value": map[string]any{
					"data":       []any{"", "base64"},
					"executable": true,
					"lamports":   1,
					"owner":      address,
					"rentEpoch":  0,
					"space":      0,
				},
			},
		})
	}))
	defer server.Close()

	t.Setenv("SOLANA_RPC_URL", server.URL)
	rpc := web3.NewSolanaRPC(cache.NewNoop())
	rpc.Client = server.Client()

	mux := http.NewServeMux()
	registerCustomerScanRoutes(mux, rpc)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scan", strings.NewReader(`{"target":"`+address+`","network":"solana-mainnet"}`))
	request.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope customerScanEnvelope
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	result := envelope.Result
	if result.Status != services.CustomerScanStatusObserved || result.EvidenceStatus != services.Web3TrustEvidenceObserved {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.Trust.Observed || result.Trust.Authorized || result.Trust.Verified || result.Trust.Finalized {
		t.Fatalf("Solana observation was over-promoted: %+v", result.Trust)
	}
	foundObservation := false
	foundExecutable := false
	for _, reason := range result.Reasons {
		switch reason {
		case "READ_ONLY_SOLANA_ACCOUNT_OBSERVATION":
			foundObservation = true
		case "SOLANA_EXECUTABLE_ACCOUNT_OBSERVED":
			foundExecutable = true
		}
	}
	if !foundObservation || !foundExecutable {
		t.Fatalf("missing Solana evidence reasons: %v", result.Reasons)
	}
	if len(result.EvidenceRefs) != 1 {
		t.Fatalf("evidence refs=%v", result.EvidenceRefs)
	}
}
