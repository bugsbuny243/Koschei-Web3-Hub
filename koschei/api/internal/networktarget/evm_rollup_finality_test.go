package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeEVMRollupFinalityAcrossSupportedNetworks(t *testing.T) {
	cases := []struct {
		network string
		chainID string
	}{
		{"base-mainnet", "0x2105"},
		{"optimism-mainnet", "0xa"},
		{"arbitrum-mainnet", "0xa4b1"},
	}

	for _, tc := range cases {
		t.Run(tc.network, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req evmRPCRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				w.Header().Set("Content-Type", "application/json")
				switch req.Method {
				case "eth_chainId":
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": tc.chainID})
				case "eth_getBlockByNumber":
					tag := req.Params[0].(string)
					number := "0x64"
					hashByte := "11"
					if tag == "safe" {
						number = "0x62"
						hashByte = "22"
					}
					if tag == "finalized" {
						number = "0x60"
						hashByte = "33"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{
						"jsonrpc": "2.0",
						"id":      req.ID,
						"result": map[string]any{
							"number": number,
							"hash":   "0x" + repeatRollupHex(hashByte),
						},
					})
				default:
					t.Fatalf("unexpected method %q", req.Method)
				}
			}))
			defer server.Close()

			got, err := ProbeEVMRollupFinality(context.Background(), server.Client(), server.URL, tc.network, time.Date(2026, 9, 23, 4, 30, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			if got.ChainID != tc.chainID || got.Latest.Number != 100 || got.Safe.Number != 98 || got.Finalized.Number != 96 {
				t.Fatalf("unexpected finality result: %#v", got)
			}
			if got.LatestToSafe != 2 || got.LatestToFinalized != 4 {
				t.Fatalf("unexpected finality distances: %#v", got)
			}
			if got.EndpointScope != "single_rpc_endpoint_block_tags" {
				t.Fatalf("endpoint scope=%q", got.EndpointScope)
			}
			if got.Observation.Network.ID != tc.network || got.Observation.SubjectKind != "network" {
				t.Fatalf("unexpected observation: %#v", got.Observation)
			}
		})
	}
}

func TestProbeEVMRollupFinalityRejectsNonMonotonicHeads(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "eth_chainId" {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x2105"})
			return
		}
		tag := req.Params[0].(string)
		number := "0x64"
		if tag == "safe" {
			number = "0x65"
		}
		if tag == "finalized" {
			number = "0x60"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"number": number,
				"hash":   "0x" + repeatRollupHex("44"),
			},
		})
	}))
	defer server.Close()

	if _, err := ProbeEVMRollupFinality(context.Background(), server.Client(), server.URL, "base-mainnet", time.Now().UTC()); err == nil {
		t.Fatal("non-monotonic finality heads were accepted")
	}
}

func TestProbeEVMRollupFinalityRejectsNullSafeTag(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "eth_chainId" {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0xa"})
			return
		}
		tag := req.Params[0].(string)
		if tag == "safe" {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"number": "0x64",
				"hash":   "0x" + repeatRollupHex("55"),
			},
		})
	}))
	defer server.Close()

	if _, err := ProbeEVMRollupFinality(context.Background(), server.Client(), server.URL, "optimism-mainnet", time.Now().UTC()); err == nil {
		t.Fatal("null safe tag was accepted")
	}
}

func repeatRollupHex(pair string) string {
	out := ""
	for i := 0; i < 32; i++ {
		out += pair
	}
	return out
}
