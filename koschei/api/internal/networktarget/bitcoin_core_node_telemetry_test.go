package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeBitcoinCoreNodeTelemetryIsEndpointScoped(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req bitcoinCoreRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		var result any
		switch req.Method {
		case "getnetworkinfo":
			result = map[string]any{
				"version": 280000,
				"subversion": "/Satoshi:28.0.0/",
				"protocolversion": 70016,
				"connections": 11,
				"networkactive": true,
			}
		case "getblockchaininfo":
			result = map[string]any{
				"chain": "main",
				"blocks": 900000,
				"headers": 900000,
				"verificationprogress": 1.0,
				"initialblockdownload": false,
			}
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil, "id": req.ID})
	}))
	defer server.Close()

	got, err := ProbeBitcoinCoreNodeTelemetry(context.Background(), server.Client(), server.URL, time.Date(2026, 9, 23, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.Connections != 11 || got.Blocks != 900000 || got.Headers != 900000 || !got.NetworkActive {
		t.Fatalf("unexpected Bitcoin telemetry: %#v", got)
	}
	if got.EndpointScope != "single_bitcoin_core_node_only" {
		t.Fatalf("endpoint scope=%q", got.EndpointScope)
	}
	if got.Observation.Network.ID != "bitcoin-mainnet" || got.Observation.SubjectKind != "node" {
		t.Fatalf("unexpected observation: %#v", got.Observation)
	}
	if got.Observation.HashSharePct != nil {
		t.Fatalf("node probe invented miner hash share: %#v", got.Observation)
	}
}

func TestProbeBitcoinCoreNodeTelemetryRejectsNonMainnet(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req bitcoinCoreRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := any(map[string]any{
			"version": 280000,
			"subversion": "/Satoshi:28.0.0/",
			"protocolversion": 70016,
			"connections": 1,
			"networkactive": true,
		})
		if req.Method == "getblockchaininfo" {
			result = map[string]any{
				"chain": "test",
				"blocks": 1,
				"headers": 1,
				"verificationprogress": 1.0,
				"initialblockdownload": false,
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil, "id": req.ID})
	}))
	defer server.Close()

	if _, err := ProbeBitcoinCoreNodeTelemetry(context.Background(), server.Client(), server.URL, time.Now().UTC()); err == nil {
		t.Fatal("non-mainnet Bitcoin node was accepted")
	}
}