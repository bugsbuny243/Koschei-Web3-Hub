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

func TestProbeBitcoinBlockIngestBindsBlockAndTransactions(t *testing.T) {
	blockHash := strings.Repeat("a", 64)
	parentHash := strings.Repeat("b", 64)
	txid := strings.Repeat("c", 64)

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
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"chain": "main", "blocks": 101}, "error": nil, "id": req.ID})
		case "getblockhash":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": blockHash, "error": nil, "id": req.ID})
		case "getblock":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"hash": blockHash, "height": 101, "previousblockhash": parentHash, "time": 1700000000, "tx": []string{txid},
			}, "error": nil, "id": req.ID})
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	got, err := ProbeBitcoinBlockIngest(context.Background(), server.Client(), server.URL, 101, time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.NetworkID != "bitcoin-mainnet" || got.Height != 101 || got.Hash != blockHash || got.PreviousHash != parentHash {
		t.Fatalf("unexpected block identity: %#v", got)
	}
	if len(got.TransactionIDs) != 1 || got.TransactionIDs[0] != txid {
		t.Fatalf("unexpected transactions: %#v", got.TransactionIDs)
	}
	if len(got.BlockchainInfoResponseSHA256) != 64 || len(got.BlockHashResponseSHA256) != 64 || len(got.BlockResponseSHA256) != 64 {
		t.Fatalf("missing native response digests: %#v", got)
	}
}

func TestProbeBitcoinBlockIngestRejectsWrongNetwork(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "getblockchaininfo" {
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"chain": "test", "blocks": 101}, "error": nil, "id": req.ID})
			return
		}
		http.Error(w, "unexpected", http.StatusBadRequest)
	}))
	defer server.Close()

	if _, err := ProbeBitcoinBlockIngest(context.Background(), server.Client(), server.URL, 101, time.Now().UTC()); err == nil {
		t.Fatal("non-mainnet Bitcoin endpoint was accepted")
	}
}
