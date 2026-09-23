package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeEthereumBeaconTelemetryCollectsConsensusEvidence(t *testing.T) {
	root1 := "0x" + repeatBeaconHex("11")
	root2 := "0x" + repeatBeaconHex("22")
	root3 := "0x" + repeatBeaconHex("33")

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/eth/v1/node/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": "Lighthouse/v7.1.0"}})
		case "/eth/v1/node/peer_count":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"disconnected":  "3",
				"connecting":    "1",
				"connected":     "64",
				"disconnecting": "0",
			}})
		case "/eth/v1/node/syncing":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"head_slot":     "123456",
				"sync_distance": "0",
				"is_syncing":    false,
				"is_optimistic": false,
				"el_offline":    false,
			}})
		case "/eth/v1/beacon/states/head/finality_checkpoints":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"execution_optimistic": false,
				"finalized":            false,
				"data": map[string]any{
					"previous_justified": map[string]any{"epoch": "3800", "root": root1},
					"current_justified":  map[string]any{"epoch": "3801", "root": root2},
					"finalized":          map[string]any{"epoch": "3799", "root": root3},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	got, err := ProbeEthereumBeaconTelemetry(context.Background(), server.Client(), server.URL, time.Date(2026, 9, 23, 4, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientVersion != "Lighthouse/v7.1.0" || got.ConnectedPeers != 64 || got.HeadSlot != 123456 || got.SyncDistance != 0 || got.IsSyncing {
		t.Fatalf("unexpected beacon telemetry: %#v", got)
	}
	if got.Finalized.Epoch != 3799 || got.CurrentJustified.Epoch != 3801 || got.Finalized.Root != root3 {
		t.Fatalf("unexpected checkpoint evidence: %#v", got)
	}
	if got.EndpointScope != "single_beacon_endpoint_plus_chain_checkpoints" {
		t.Fatalf("endpoint scope=%q", got.EndpointScope)
	}
	if got.Observation.Network.ID != "ethereum-mainnet" || got.Observation.SubjectKind != "node" {
		t.Fatalf("unexpected observation: %#v", got.Observation)
	}
	if got.Observation.StakeSharePct != nil {
		t.Fatalf("beacon endpoint invented validator stake share: %#v", got.Observation)
	}
}

func TestProbeEthereumBeaconTelemetryFailsClosedOnInvalidCheckpoint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/eth/v1/node/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": "Teku/v1"}})
		case "/eth/v1/node/peer_count":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"disconnected": "0", "connecting": "0", "connected": "1", "disconnecting": "0"}})
		case "/eth/v1/node/syncing":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"head_slot": "1", "sync_distance": "0", "is_syncing": false, "is_optimistic": false, "el_offline": false}})
		case "/eth/v1/beacon/states/head/finality_checkpoints":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"execution_optimistic": false,
				"finalized":            true,
				"data": map[string]any{
					"previous_justified": map[string]any{"epoch": "1", "root": "bad-root"},
					"current_justified":  map[string]any{"epoch": "1", "root": "bad-root"},
					"finalized":          map[string]any{"epoch": "1", "root": "bad-root"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if _, err := ProbeEthereumBeaconTelemetry(context.Background(), server.Client(), server.URL, time.Now().UTC()); err == nil {
		t.Fatal("invalid checkpoint root was accepted")
	}
}

func TestProbeEthereumBeaconTelemetryRejectsNonHTTPS(t *testing.T) {
	if _, err := ProbeEthereumBeaconTelemetry(context.Background(), http.DefaultClient, "http://beacon.example", time.Now().UTC()); err == nil {
		t.Fatal("non-HTTPS beacon endpoint was accepted")
	}
}

func repeatBeaconHex(pair string) string {
	out := ""
	for i := 0; i < 32; i++ {
		out += pair
	}
	return out
}
