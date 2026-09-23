package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeEVMNodeTelemetryAcrossRegisteredEVMNetworks(t *testing.T) {
	cases := []struct {
		network string
		chainID string
	}{
		{"ethereum-mainnet", "0x1"},
		{"base-mainnet", "0x2105"},
		{"arbitrum-mainnet", "0xa4b1"},
		{"optimism-mainnet", "0xa"},
		{"polygon-mainnet", "0x89"},
		{"bnb-mainnet", "0x38"},
		{"avalanche-mainnet", "0xa86a"},
	}
	for _, tc := range cases {
		t.Run(tc.network, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req evmRPCRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				var result any
				switch req.Method {
				case "eth_chainId":
					result = tc.chainID
				case "web3_clientVersion":
					result = "test-client/v1.2.3"
				case "net_peerCount":
					result = "0x2a"
				case "eth_blockNumber":
					result = "0x123"
				case "eth_syncing":
					result = false
				default:
					t.Fatalf("unexpected method %q", req.Method)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
			}))
			defer server.Close()

			got, err := ProbeEVMNodeTelemetry(context.Background(), server.Client(), server.URL, tc.network, time.Date(2026, 9, 23, 3, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			if got.ChainID != tc.chainID || got.ExpectedChainID != tc.chainID {
				t.Fatalf("chain identity mismatch: %#v", got)
			}
			if got.PeerCount != 42 || got.HeadBlock != 0x123 || got.Syncing {
				t.Fatalf("unexpected endpoint telemetry: %#v", got)
			}
			if got.EndpointScope != "single_rpc_endpoint_only" {
				t.Fatalf("endpoint scope=%q", got.EndpointScope)
			}
			if got.Observation.Network.ID != tc.network || got.Observation.SubjectKind != "node" {
				t.Fatalf("unexpected observation: %#v", got.Observation)
			}
			if got.Observation.StakeSharePct != nil || got.Observation.HashSharePct != nil {
				t.Fatalf("endpoint probe invented consensus contribution: %#v", got.Observation)
			}
		})
	}
}

func TestProbeEVMNodeTelemetryRejectsNetworkMismatch(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
	}))
	defer server.Close()

	if _, err := ProbeEVMNodeTelemetry(context.Background(), server.Client(), server.URL, "base-mainnet", time.Now().UTC()); err == nil {
		t.Fatal("mismatched Base endpoint was accepted")
	}
}