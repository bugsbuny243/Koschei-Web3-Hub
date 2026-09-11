package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeEVMVerifiesChainBeforeCode(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_getCode":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x60016000"})
		default:
			t.Fatalf("unexpected rpc method %q", req.Method)
		}
	}))
	defer server.Close()

	resolution, err := Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	result, err := ProbeEVM(context.Background(), server.Client(), server.URL, resolution)
	if err != nil {
		t.Fatalf("probe EVM: %v", err)
	}
	if requests != 2 {
		t.Fatalf("rpc requests = %d; want 2", requests)
	}
	if result.ChainID != "0x1" || result.ContractCodeState != "contract_code_observed" || len(result.ContractCodeHash) != 64 {
		t.Fatalf("unexpected evidence result: %+v", result)
	}
	if result.EvidenceStatus != "observed" || !result.AnalysisPerformed || result.Resolution.EvidenceStatus != "observed" {
		t.Fatalf("probe must expose observed evidence state: %+v", result)
	}
}

func TestProbeEVMRejectsMisconfiguredNetworkBeforeCodeLookup(t *testing.T) {
	methods := []string{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		methods = append(methods, req.Method)
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
	}))
	defer server.Close()

	resolution, err := Resolve("base-mainnet", "0x2222222222222222222222222222222222222222")
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	_, err = ProbeEVM(context.Background(), server.Client(), server.URL, resolution)
	if err == nil || !strings.Contains(err.Error(), "evm_rpc_network_mismatch") {
		t.Fatalf("expected network mismatch, got %v", err)
	}
	if len(methods) != 1 || methods[0] != "eth_chainId" {
		t.Fatalf("probe continued after network mismatch: %v", methods)
	}
}

func TestProbeEVMDoesNotTreatEmptyCodeAsAccountExistence(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := "0x1"
		if req.Method == "eth_getCode" {
			result = "0x"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()

	resolution, _ := Resolve("ethereum-mainnet", "0x3333333333333333333333333333333333333333")
	result, err := ProbeEVM(context.Background(), server.Client(), server.URL, resolution)
	if err != nil {
		t.Fatalf("probe EVM: %v", err)
	}
	if result.ContractCodeState != "no_contract_code_observed" || result.ContractCodeHash != "" {
		t.Fatalf("empty code was overstated: %+v", result)
	}
}

func TestProbeEVMRequiresHTTPSConfiguredEndpoint(t *testing.T) {
	resolution, _ := Resolve("ethereum-mainnet", "0x4444444444444444444444444444444444444444")
	if _, err := ProbeEVM(context.Background(), http.DefaultClient, "http://127.0.0.1:8545", resolution); err == nil {
		t.Fatal("non-HTTPS endpoint was accepted")
	}
}
