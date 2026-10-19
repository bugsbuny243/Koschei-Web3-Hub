package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

const nativeJobEVMFixture = "0x1111111111111111111111111111111111111111"
const nativeJobBitcoinFixture = "1BoatSLRHtKNngkdXEeobR76b53LETtpyT"
const nativeJobBitcoinGenesis = "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"

func TestCanonicalNativeEVMCustomerJobPersistsFullEvidenceWithoutGrade(t *testing.T) {
	observedCalls := []string{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode test RPC: %v", err)
			return
		}
		observedCalls = append(observedCalls, req.Method)
		value := "0x1"
		switch req.Method {
		case "eth_chainId":
		case "eth_getCode":
			value = "0x"
		case "eth_getStorageAt":
			value = "0x" + strings.Repeat("0", 64)
		default:
			t.Errorf("unexpected native RPC call %q", req.Method)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": value})
	}))
	defer server.Close()
	t.Setenv("ETHEREUM_RPC_URL", server.URL)
	report, err := canonicalNativeNetworkJobWithClient(t.Context(), server.Client(), nativeJobEVMFixture, "ethereum-mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if report["status"] != "evidence_only_partial" || report["investigation_kind"] != "native_network_address_observation" {
		t.Fatalf("native probe overclaimed scope: %#v", report)
	}
	result, ok := report["customer_scan"].(services.CustomerScanResult)
	if !ok || result.NetworkEvidence == nil || result.NetworkEvidence.Status != services.IntelligenceEvidenceObserved {
		t.Fatalf("native observation was dropped: %#v", report["customer_scan"])
	}
	if result.NetworkEvidence.Source == "" || result.NetworkEvidence.Provenance == "" {
		t.Fatalf("native provenance missing: %#v", result.NetworkEvidence)
	}
	if result.Trust.Verified || result.Trust.Finalized || result.Verdict == "safe" {
		t.Fatalf("read-only evidence was upgraded: %+v", result)
	}
	final := report["final_verdict"].(map[string]any)
	if final["signed"] != false || final["withheld"] != true || final["risk_level"] != "unknown" {
		t.Fatalf("customer job fabricated a grade: %#v", final)
	}
	if len(observedCalls) < 2 || observedCalls[0] != "eth_chainId" || observedCalls[1] != "eth_getCode" {
		t.Fatalf("unexpected native evidence order: %v", observedCalls)
	}
}

func TestCanonicalNativeBitcoinCustomerJobPersistsObservedProvenance(t *testing.T) {
	paths := []string{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/block-height/0":
			_, _ = w.Write([]byte(nativeJobBitcoinGenesis))
		case "/address/" + nativeJobBitcoinFixture:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"address": nativeJobBitcoinFixture,
				"chain_stats": map[string]any{"funded_txo_count": 2, "funded_txo_sum": 1500, "spent_txo_count": 1, "spent_txo_sum": 500, "tx_count": 2},
				"mempool_stats": map[string]any{"funded_txo_count": 0, "funded_txo_sum": 0, "spent_txo_count": 0, "spent_txo_sum": 0, "tx_count": 0},
			})
		default:
			t.Errorf("unexpected Esplora path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("BITCOIN_ESPLORA_URL", server.URL)
	report, err := canonicalNativeNetworkJobWithClient(t.Context(), server.Client(), nativeJobBitcoinFixture, "bitcoin-mainnet")
	if err != nil {
		t.Fatal(err)
	}
	result, ok := report["customer_scan"].(services.CustomerScanResult)
	if !ok || result.NetworkEvidence == nil || result.NetworkEvidence.Chain != "bitcoin" {
		t.Fatalf("Bitcoin evidence was dropped: %#v", report["customer_scan"])
	}
	if report["status"] != "evidence_only_partial" || result.Trust.Verified {
		t.Fatalf("Bitcoin observation upgraded to authority: %#v", report)
	}
	if len(paths) != 2 || paths[0] != "/block-height/0" {
		t.Fatalf("Esplora chain check not enforced: %v", paths)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "network_evidence") || !strings.Contains(string(encoded), "address_response_sha256") {
		t.Fatalf("persisted customer report lost native evidence hashes: %s", encoded)
	}
}

func TestCanonicalNativeCustomerJobFailClosedNetworks(t *testing.T) {
	for _, tc := range []struct{ network, target string }{
		{"bitcoin-mainnet", nativeJobEVMFixture},
		{"ethereum-mainnet", nativeJobBitcoinFixture},
		{"sui-mainnet", "0x" + strings.Repeat("a", 64)},
		{"unknown-mainnet", nativeJobEVMFixture},
	} {
		t.Run(tc.network+"/"+tc.target[:3], func(t *testing.T) {
			if _, err := canonicalNativeNetworkJobWithClient(context.Background(), nil, tc.target, tc.network); err == nil {
				t.Fatalf("unsupported/mismatched network unexpectedly created report: %s", tc.network)
			}
		})
	}
}
