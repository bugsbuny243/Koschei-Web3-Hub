package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeBitcoinVerifiesMainnetBeforeAddressLookup(t *testing.T) {
	paths := []string{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/block-height/0":
			_, _ = w.Write([]byte(bitcoinMainnetGenesisHash))
		case "/address/1BoatSLRHtKNngkdXEeobR76b53LETtpyT":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"address":       "1BoatSLRHtKNngkdXEeobR76b53LETtpyT",
				"chain_stats":   map[string]any{"funded_txo_count": 2, "funded_txo_sum": 1500, "spent_txo_count": 1, "spent_txo_sum": 500, "tx_count": 2},
				"mempool_stats": map[string]any{"funded_txo_count": 0, "funded_txo_sum": 0, "spent_txo_count": 0, "spent_txo_sum": 0, "tx_count": 0},
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	resolution, err := Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	result, err := ProbeBitcoin(context.Background(), server.Client(), server.URL, resolution)
	if err != nil {
		t.Fatalf("probe bitcoin: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/block-height/0" {
		t.Fatalf("unexpected request order: %v", paths)
	}
	if result.ActivityState != "activity_observed" || result.ConfirmedTXCount != 2 || result.FundedSats != 1500 || result.SpentSats != 500 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.AnalysisPerformed || result.EvidenceStatus != "observed" || result.Resolution.EvidenceStatus != "observed" {
		t.Fatalf("probe did not expose observed evidence: %+v", result)
	}
}

func TestProbeBitcoinStopsOnNetworkMismatch(t *testing.T) {
	paths := []string{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte("000000000933ea01ad0ee984209779baae8c49c3c8741b3f6b11c8e6c7f5b3f0"))
	}))
	defer server.Close()

	resolution, _ := Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	_, err := ProbeBitcoin(context.Background(), server.Client(), server.URL, resolution)
	if err == nil || !strings.Contains(err.Error(), "bitcoin_esplora_network_mismatch") {
		t.Fatalf("expected network mismatch, got %v", err)
	}
	if len(paths) != 1 || paths[0] != "/block-height/0" {
		t.Fatalf("probe continued after network mismatch: %v", paths)
	}
}

func TestProbeBitcoinDoesNotTreatNoActivityAsSafeOrNonexistent(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/block-height/0" {
			_, _ = w.Write([]byte(bitcoinMainnetGenesisHash))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"address":       "1BoatSLRHtKNngkdXEeobR76b53LETtpyT",
			"chain_stats":   map[string]any{"funded_txo_count": 0, "funded_txo_sum": 0, "spent_txo_count": 0, "spent_txo_sum": 0, "tx_count": 0},
			"mempool_stats": map[string]any{"funded_txo_count": 0, "funded_txo_sum": 0, "spent_txo_count": 0, "spent_txo_sum": 0, "tx_count": 0},
		})
	}))
	defer server.Close()

	resolution, _ := Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	result, err := ProbeBitcoin(context.Background(), server.Client(), server.URL, resolution)
	if err != nil {
		t.Fatalf("probe bitcoin: %v", err)
	}
	if result.ActivityState != "no_activity_observed" || result.EvidenceStatus != "observed" {
		t.Fatalf("no-activity evidence overstated: %+v", result)
	}
}

func TestProbeBitcoinRequiresHTTPSConfiguredEndpoint(t *testing.T) {
	resolution, _ := Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	if _, err := ProbeBitcoin(context.Background(), http.DefaultClient, "http://127.0.0.1:3000", resolution); err == nil {
		t.Fatal("non-HTTPS endpoint was accepted")
	}
}
