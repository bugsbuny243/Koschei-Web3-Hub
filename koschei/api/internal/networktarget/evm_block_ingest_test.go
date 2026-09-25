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

func TestProbeEVMBlockIngestBindsBlockTransactionsAndLogs(t *testing.T) {
	blockHash := "0x" + strings.Repeat("a", 64)
	parentHash := "0x" + strings.Repeat("b", 64)
	txHash := "0x" + strings.Repeat("c", 64)
	topic := "0x" + strings.Repeat("d", 64)
	address := "0x" + strings.Repeat("e", 40)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params []json.RawMessage `json:"params"`
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
				"number": "0x2a", "hash": blockHash, "parentHash": parentHash, "timestamp": "0x64",
				"transactions": []string{txHash},
			}})
		case "eth_getLogs":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": []map[string]any{{
				"address": address, "topics": []string{topic}, "data": "0x1234",
				"transactionHash": txHash, "blockHash": blockHash, "blockNumber": "0x2a", "logIndex": "0x0", "removed": false,
			}}})
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	got, err := ProbeEVMBlockIngest(context.Background(), server.Client(), server.URL, "ethereum-mainnet", 42, time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.Height != 42 || got.Hash != blockHash || got.ParentHash != parentHash || got.ChainID != "0x1" {
		t.Fatalf("unexpected block identity: %#v", got)
	}
	if len(got.TransactionHashes) != 1 || got.TransactionHashes[0] != txHash {
		t.Fatalf("transactions=%#v", got.TransactionHashes)
	}
	if len(got.Logs) != 1 || got.Logs[0].TxHash != txHash || got.Logs[0].Address != address || got.Logs[0].LogIndex != 0 {
		t.Fatalf("logs=%#v", got.Logs)
	}
	if len(got.BlockResponseSHA256) != 64 || len(got.LogsResponseSHA256) != 64 || len(got.Logs[0].DataSHA256) != 64 {
		t.Fatalf("missing evidence digests: %#v", got)
	}
}

func TestProbeEVMBlockIngestRejectsLogFromDifferentBlock(t *testing.T) {
	blockHash := "0x" + strings.Repeat("a", 64)
	parentHash := "0x" + strings.Repeat("b", 64)
	txHash := "0x" + strings.Repeat("c", 64)
	otherHash := "0x" + strings.Repeat("f", 64)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_getBlockByNumber":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"number": "0x2a", "hash": blockHash, "parentHash": parentHash, "timestamp": "0x64", "transactions": []string{txHash},
			}})
		case "eth_getLogs":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": []map[string]any{{
				"address": "0x" + strings.Repeat("e", 40), "topics": []string{}, "data": "0x",
				"transactionHash": txHash, "blockHash": otherHash, "blockNumber": "0x2a", "logIndex": "0x0", "removed": false,
			}}})
		}
	}))
	defer server.Close()

	if _, err := ProbeEVMBlockIngest(context.Background(), server.Client(), server.URL, "ethereum-mainnet", 42, time.Now().UTC()); err == nil {
		t.Fatal("cross-block log was accepted")
	}
}
