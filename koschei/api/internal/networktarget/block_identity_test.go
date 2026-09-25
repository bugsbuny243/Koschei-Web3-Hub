package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProbeEVMBlockIdentityBindsNetworkHeightAndParent(t *testing.T) {
	blockHash := "0x" + strings.Repeat("a", 64)
	parentHash := "0x" + strings.Repeat("b", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_getBlockByNumber":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"number": "0x2a", "hash": blockHash, "parentHash": parentHash, "timestamp": "0x64", "transactions": []string{},
			}})
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	got, err := ProbeEVMBlockIdentity(context.Background(), server.Client(), server.URL, "ethereum-mainnet", 42, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got.Height != 42 || got.Hash != blockHash || got.ParentHash != parentHash || len(got.SourceSHA256) != 64 {
		t.Fatalf("unexpected identity: %#v", got)
	}
}

func TestProbeBitcoinBlockIdentityBindsMainnetHeader(t *testing.T) {
	blockHash := strings.Repeat("a", 64)
	parentHash := strings.Repeat("b", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "getblockchaininfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"chain": "main", "blocks": 100}, "error": nil, "id": req.ID})
		case "getblockhash":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": blockHash, "error": nil, "id": req.ID})
		case "getblockheader":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"hash": blockHash, "height": 100, "previousblockhash": parentHash,
			}, "error": nil, "id": req.ID})
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	got, err := ProbeBitcoinBlockIdentity(context.Background(), server.Client(), server.URL, 100, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got.NetworkID != "bitcoin-mainnet" || got.Height != 100 || got.Hash != blockHash || got.ParentHash != parentHash || len(got.SourceSHA256) != 64 {
		t.Fatalf("unexpected identity: %#v", got)
	}
}
