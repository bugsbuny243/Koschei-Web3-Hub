package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeEVMObservesEIP7702DelegationIndicator(t *testing.T) {
	const delegate = "2222222222222222222222222222222222222222"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		result := "0x1"
		if req.Method == "eth_getCode" {
			result = "0xef0100" + delegate
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()

	resolution, err := Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ProbeEVM(context.Background(), server.Client(), server.URL, resolution)
	if err != nil {
		t.Fatal(err)
	}
	if result.DelegationState != EVMDelegationStateObserved {
		t.Fatalf("delegation state=%q", result.DelegationState)
	}
	if result.DelegationTarget != "0x"+delegate {
		t.Fatalf("delegation target=%q", result.DelegationTarget)
	}
	if result.EvidenceStatus != "observed" {
		t.Fatalf("evidence status=%q", result.EvidenceStatus)
	}
}

func TestProbeEVMDoesNotMisclassifyOrdinaryCodeAsEIP7702(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := "0x1"
		if req.Method == "eth_getCode" {
			result = "0x60016000"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()

	resolution, _ := Resolve("ethereum-mainnet", "0x3333333333333333333333333333333333333333")
	result, err := ProbeEVM(context.Background(), server.Client(), server.URL, resolution)
	if err != nil {
		t.Fatal(err)
	}
	if result.DelegationState != EVMDelegationStateNotObserved || result.DelegationTarget != "" {
		t.Fatalf("ordinary code misclassified as delegation: %+v", result)
	}
}
