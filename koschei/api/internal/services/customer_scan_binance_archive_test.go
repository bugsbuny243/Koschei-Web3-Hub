package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestBinanceArchiveReachesCustomerEvidence(t *testing.T) {
	var fixture struct {
		Network   string `json:"network"`
		ChainID   string `json:"chain_id"`
		Address   string `json:"address"`
		Delegate  string `json:"delegate"`
		Snapshots []struct {
			Phase string `json:"phase"`
			Block string `json:"block_number"`
			Code  string `json:"code"`
		} `json:"snapshots"`
	}
	raw, err := os.ReadFile("../networktarget/testdata/binance_20261029_archive.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	resolution, err := networktarget.Resolve(fixture.Network, fixture.Address)
	if err != nil {
		t.Fatal(err)
	}
	target, err := ClassifyCustomerScanTarget(fixture.Address, fixture.Network)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.Snapshots) != 4 {
		t.Fatal("missing archive samples")
	}
	for _, snapshot := range fixture.Snapshots {
		t.Run(snapshot.Phase, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     int      `json:"id"`
					Method string   `json:"method"`
					Params []string `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				result := fixture.ChainID
				if request.Method == "eth_getCode" {
					if len(request.Params) != 2 || request.Params[0] != fixture.Address || request.Params[1] != snapshot.Block {
						t.Error("wrong archive request")
						w.WriteHeader(400)
						return
					}
					result = snapshot.Code
				} else if request.Method != "eth_chainId" {
					t.Error("unexpected RPC")
					w.WriteHeader(400)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
			}))
			defer server.Close()
			probe, err := networktarget.ProbeEVMAtBlock(context.Background(), server.Client(), server.URL, resolution, snapshot.Block)
			if err != nil {
				t.Fatal(err)
			}
			projection, err := AdaptEVMProbeEvidence(probe, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			result, err := CustomerScanResultFromNetworkProbe(target, projection)
			if err != nil {
				t.Fatal(err)
			}
			want := snapshot.Phase == "delegation" || snapshot.Phase == "first_sweep"
			if slices.Contains(result.Reasons, "EIP7702_DELEGATION_OBSERVED") != want {
				t.Fatalf("lost or invented delegation signal: %v", result.Reasons)
			}
			if !result.Trust.Observed || result.Trust.Authorized || result.Trust.Verified || result.Trust.Finalized {
				t.Fatalf("historical evidence became authorization: %+v", result.Trust)
			}
			if result.NetworkEvidence == nil || result.NetworkEvidence.Attributes["requested_block"] != snapshot.Block || result.NetworkEvidence.Attributes["evidence_scope"] != "read_only_historical_network_probe" {
				t.Fatal("historical provenance lost in customer response")
			}
			if want && result.NetworkEvidence.Attributes["delegation_target"] != fixture.Delegate {
				t.Fatal("lost delegate address")
			}
		})
	}
}
