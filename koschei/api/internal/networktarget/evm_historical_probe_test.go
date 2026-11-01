package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestBinanceArchiveDelegationDetectionWindow(t *testing.T) {
	var fixture struct {
		Network   string `json:"network"`
		ChainID   string `json:"chain_id"`
		Address   string `json:"address"`
		Delegate  string `json:"delegate"`
		Snapshots []struct {
			Phase     string `json:"phase"`
			Block     string `json:"block_number"`
			Hash      string `json:"block_hash"`
			Parent    string `json:"parent_hash"`
			Timestamp string `json:"timestamp"`
			Code      string `json:"code"`
		} `json:"snapshots"`
		DelegationReceipt struct {
			Hash   string `json:"blockHash"`
			Block  string `json:"blockNumber"`
			To     string `json:"to"`
			Status string `json:"status"`
		} `json:"delegation_receipt"`
		SweepTransaction struct {
			Hash  string `json:"blockHash"`
			Block string `json:"blockNumber"`
			To    string `json:"to"`
		} `json:"sweep_transaction"`
		SweepStatus string `json:"sweep_receipt_status"`
	}
	raw, err := os.ReadFile("testdata/binance_20261029_archive.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Snapshots) != 4 {
		t.Fatal("incomplete archive capture")
	}
	before, delegated, sweep := fixture.Snapshots[0], fixture.Snapshots[1], fixture.Snapshots[2]
	if delegated.Parent != before.Hash || fixture.DelegationReceipt.Hash != delegated.Hash || fixture.DelegationReceipt.Block != delegated.Block || fixture.DelegationReceipt.Status != "0x1" || fixture.DelegationReceipt.To != fixture.Address || fixture.SweepTransaction.Hash != sweep.Hash || fixture.SweepTransaction.Block != sweep.Block || fixture.SweepTransaction.To != fixture.Address || fixture.SweepStatus != "0x1" {
		t.Fatal("archive metadata is inconsistent")
	}
	resolution, err := Resolve(fixture.Network, fixture.Address)
	if err != nil {
		t.Fatal(err)
	}
	var firstObserved int64
	for _, snapshot := range fixture.Snapshots {
		t.Run(snapshot.Phase, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request evmRPCRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				result := fixture.ChainID
				if request.Method == "eth_getCode" {
					if len(request.Params) != 2 || request.Params[0] != fixture.Address || request.Params[1] != snapshot.Block {
						t.Error("historical query changed address or fell back to latest")
						w.WriteHeader(400)
						return
					}
					result = snapshot.Code
				} else if request.Method != "eth_chainId" {
					t.Error("unexpected RPC method")
					w.WriteHeader(400)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
			}))
			defer server.Close()
			result, err := ProbeEVMAtBlock(context.Background(), server.Client(), server.URL, resolution, snapshot.Block)
			if err != nil {
				t.Fatal(err)
			}
			want := EVMDelegationStateNotObserved
			if snapshot.Phase == "delegation" || snapshot.Phase == "first_sweep" {
				want = EVMDelegationStateObserved
			}
			if result.DelegationState != want || result.RequestedBlock != snapshot.Block {
				t.Fatalf("unexpected probe result: %+v", result)
			}
			if want == EVMDelegationStateObserved {
				if result.DelegationTarget != fixture.Delegate {
					t.Fatal("wrong delegation target")
				}
				if firstObserved == 0 {
					firstObserved, err = strconv.ParseInt(snapshot.Timestamp[2:], 16, 64)
					if err != nil {
						t.Fatal(err)
					}
				}
			} else if result.DelegationTarget != "" {
				t.Fatal("invented delegation target")
			}
			t.Logf("phase=%s block=%s finding=%s", snapshot.Phase, snapshot.Block, result.DelegationState)
		})
	}
	sweepAt, err := strconv.ParseInt(sweep.Timestamp[2:], 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	if firstObserved == 0 || sweepAt-firstObserved != 204 {
		t.Fatalf("unexpected block-time opportunity: %d seconds", sweepAt-firstObserved)
	}
	t.Logf("first_observable_state=%s sweep_block_time=%s opportunity_seconds=%d; historical state detection only; actual_alert_latency=unmeasured; prevented_loss=unproven", time.Unix(firstObserved, 0).UTC().Format(time.RFC3339), time.Unix(sweepAt, 0).UTC().Format(time.RFC3339), sweepAt-firstObserved)
}

func TestHistoricalProbeRejectsMovingOrMalformedBlock(t *testing.T) {
	resolution, _ := Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	for _, block := range []string{"", "latest", "pending", "safe", "finalized", "123", "0x", "0x00", "0x01", "0xFF", "0x-1", "0x+1", "0x10000000000000000"} {
		if _, err := ProbeEVMAtBlock(context.Background(), nil, "", resolution, block); err == nil || err.Error() != "evm_historical_block_invalid" {
			t.Fatalf("accepted malformed block %q: %v", block, err)
		}
	}
}

func TestHistoricalProbeNeverFallsBackOnArchiveFailure(t *testing.T) {
	resolution, _ := Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	codeReads := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x1"}
		if request.Method == "eth_getCode" {
			codeReads++
			if len(request.Params) != 2 || request.Params[1] != "0x1915151" {
				t.Error("archive error triggered latest fallback")
			}
			delete(response, "result")
			response["error"] = map[string]any{"code": -32000, "message": "historical state unavailable"}
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	result, err := ProbeEVMAtBlock(context.Background(), server.Client(), server.URL, resolution, "0x1915151")
	if err == nil || result.AnalysisPerformed || result.DelegationState != "" || codeReads != 1 {
		t.Fatalf("archive failure turned into evidence: %+v, %v, reads=%d", result, err, codeReads)
	}
}
