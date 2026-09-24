package networktarget

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeBitcoinPoWNetworkTelemetryCollectsBoundedNetworkEvidence(t *testing.T) {
	best := repeatBitcoinPoWHex("11")
	work := repeatBitcoinPoWHex("22")

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req bitcoinCoreRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		var result any
		switch req.Method {
		case "getblockchaininfo":
			result = map[string]any{
				"chain":                "main",
				"blocks":               900001,
				"headers":              900001,
				"bestblockhash":        best,
				"difficulty":           123456789.5,
				"chainwork":            work,
				"initialblockdownload": false,
			}
		case "getnetworkhashps":
			result = 8.75e20
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil, "id": req.ID})
	}))
	defer server.Close()

	got, err := ProbeBitcoinPoWNetworkTelemetry(context.Background(), server.Client(), server.URL, time.Date(2026, 9, 23, 5, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.EstimatedNetworkHashPS != 8.75e20 || got.Difficulty != 123456789.5 {
		t.Fatalf("unexpected PoW estimates: %#v", got)
	}
	if got.BestBlockHash != best || got.Chainwork != work || got.Blocks != 900001 || got.Headers != 900001 {
		t.Fatalf("unexpected chain anchors: %#v", got)
	}
	if got.EstimatorScope != "bitcoin_core_network_estimate_from_single_mainnet_node" {
		t.Fatalf("scope=%q", got.EstimatorScope)
	}
	if got.Observation.Network.ID != "bitcoin-mainnet" || got.Observation.SubjectKind != "network" {
		t.Fatalf("unexpected observation: %#v", got.Observation)
	}
	if got.Observation.HashSharePct != nil || got.Observation.StakeSharePct != nil {
		t.Fatalf("network estimate invented miner share: %#v", got.Observation)
	}
}

func TestProbeBitcoinPoWNetworkTelemetryRejectsNonMainnet(t *testing.T) {
	server := bitcoinPoWTestServer(t, "test", 1, 1)
	defer server.Close()

	if _, err := ProbeBitcoinPoWNetworkTelemetry(context.Background(), server.Client(), server.URL, time.Now().UTC()); err == nil {
		t.Fatal("non-mainnet Bitcoin Core endpoint was accepted")
	}
}

func TestProbeBitcoinPoWNetworkTelemetryRejectsInvalidNumericEvidence(t *testing.T) {
	server := bitcoinPoWTestServer(t, "main", -1, math.Inf(1))
	defer server.Close()

	if _, err := ProbeBitcoinPoWNetworkTelemetry(context.Background(), server.Client(), server.URL, time.Now().UTC()); err == nil {
		t.Fatal("invalid PoW numeric evidence was accepted")
	}
}

func bitcoinPoWTestServer(t *testing.T, chain string, difficulty, hashPS float64) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req bitcoinCoreRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		var result any
		switch req.Method {
		case "getblockchaininfo":
			result = map[string]any{
				"chain":                chain,
				"blocks":               1,
				"headers":              1,
				"bestblockhash":        repeatBitcoinPoWHex("aa"),
				"difficulty":           difficulty,
				"chainwork":            repeatBitcoinPoWHex("bb"),
				"initialblockdownload": false,
			}
		case "getnetworkhashps":
			result = hashPS
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil, "id": req.ID})
	}))
}

func repeatBitcoinPoWHex(pair string) string {
	out := ""
	for i := 0; i < 32; i++ {
		out += pair
	}
	return out
}

func TestProbeBitcoinPoWNetworkTelemetryCapturesNativeResponseDigests(t *testing.T) {
	best := repeatBitcoinPoWHex("33")
	work := repeatBitcoinPoWHex("44")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req bitcoinCoreRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		switch req.Method {
		case "getblockchaininfo":
			_, _ = w.Write([]byte(`{"result":{"chain":"main","blocks":900001,"headers":900001,"bestblockhash":"` + best + `","difficulty":123456789.5,"chainwork":"` + work + `","initialblockdownload":false},"error":null,"id":401}`))
		case "getnetworkhashps":
			_, _ = w.Write([]byte(`{"result":8.75e20,"error":null,"id":402}`))
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
	}))
	defer server.Close()

	got, err := ProbeBitcoinPoWNetworkTelemetry(context.Background(), server.Client(), server.URL, time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.BlockchainInfoResponseSHA256 == "" || got.NetworkHashPSResponseSHA256 == "" {
		t.Fatalf("missing native response digests: %#v", got)
	}
}
