package networktarget

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeEVMTransactionBindsReceiptAndExecutionState(t *testing.T) {
	txHash := "0x" + strings.Repeat("a", 64)
	from := "0x1111111111111111111111111111111111111111"
	to := "0x2222222222222222222222222222222222222222"
	logAddress := "0x3333333333333333333333333333333333333333"
	topic := "0x" + strings.Repeat("b", 64)
	methods := []string{}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, req.Method)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_getTransactionByHash":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"hash": txHash, "from": from, "to": to, "value": "0x1", "nonce": "0x2",
				"gas": "0x5208", "gasPrice": "0x3", "type": "0x2", "input": "0x1234",
				"blockHash": "0x" + strings.Repeat("c", 64), "blockNumber": "0x10",
			}})
		case "eth_getTransactionReceipt":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"transactionHash": txHash, "blockHash": "0x" + strings.Repeat("c", 64), "blockNumber": "0x10",
				"status": "0x1", "gasUsed": "0x5000", "cumulativeGasUsed": "0x5000", "effectiveGasPrice": "0x4",
				"contractAddress": nil, "logs": []any{map[string]any{
					"address": logAddress, "topics": []string{topic}, "logIndex": "0x0", "removed": false,
				}},
			}})
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
	}))
	defer server.Close()

	result, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "ethereum-mainnet", txHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionState != EVMTransactionExecutionSuccess || result.ReceiptStatus != "0x1" {
		t.Fatalf("unexpected execution result: %+v", result)
	}
	if result.BlockNumber != "0x10" || len(result.Logs) != 1 || result.InputBytes != 2 || result.InputSHA256 == "" {
		t.Fatalf("incomplete transaction evidence: %+v", result)
	}
	if strings.Join(methods, ",") != "eth_chainId,eth_getTransactionByHash,eth_getTransactionReceipt" {
		t.Fatalf("unexpected RPC order: %v", methods)
	}
}

func TestProbeEVMTransactionKeepsMissingReceiptPending(t *testing.T) {
	txHash := "0x" + strings.Repeat("d", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x2105"})
		case "eth_getTransactionByHash":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"hash": txHash, "from": "0x1111111111111111111111111111111111111111", "to": nil,
				"value": "0x0", "nonce": "0x1", "gas": "0x5208", "gasPrice": "0x1", "type": "0x2",
				"input": "0x", "blockHash": nil, "blockNumber": nil,
			}})
		case "eth_getTransactionReceipt":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": nil})
		}
	}))
	defer server.Close()

	result, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "base-mainnet", txHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionState != EVMTransactionExecutionPending || result.ReceiptStatus != "" || result.BlockHash != "" {
		t.Fatalf("pending transaction was overstated: %+v", result)
	}
}

func TestProbeEVMTransactionRejectsBadHashBeforeRPC(t *testing.T) {
	if _, err := ProbeEVMTransaction(context.Background(), http.DefaultClient, "https://example.com", "ethereum-mainnet", "0x1234"); err == nil {
		t.Fatal("invalid transaction hash was accepted")
	}
}

func TestProbeEVMTransactionMissingReceiptAfterMinedTransactionIsUnknown(t *testing.T) {
	txHash := "0x" + strings.Repeat("e", 64)
	blockHash := "0x" + strings.Repeat("c", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_getTransactionByHash":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"hash": txHash, "from": "0x1111111111111111111111111111111111111111",
				"to":    "0x2222222222222222222222222222222222222222",
				"value": "0x0", "nonce": "0x1", "gas": "0x5208", "gasPrice": "0x1", "type": "0x2",
				"input": "0x", "blockHash": blockHash, "blockNumber": "0x10",
			}})
		case "eth_getTransactionReceipt":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": nil})
		}
	}))
	defer server.Close()

	result, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "ethereum-mainnet", txHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionState != EVMTransactionExecutionUnknown {
		t.Fatalf("execution=%q want unknown result=%+v", result.ExecutionState, result)
	}
	if result.BlockHash != blockHash || result.BlockNumber != "0x10" {
		t.Fatalf("mined anchors lost: %+v", result)
	}
}

func TestProbeEVMTransactionPreservesPreByzantiumRootAsUnknown(t *testing.T) {
	txHash := "0x" + strings.Repeat("f", 64)
	blockHash := "0x" + strings.Repeat("c", 64)
	root := "0x" + strings.Repeat("d", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_getTransactionByHash":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"hash": txHash, "from": "0x1111111111111111111111111111111111111111",
				"to":    "0x2222222222222222222222222222222222222222",
				"value": "0x0", "nonce": "0x1", "gas": "0x5208", "gasPrice": "0x1", "type": "0x0",
				"input": "0x", "blockHash": blockHash, "blockNumber": "0x10",
			}})
		case "eth_getTransactionReceipt":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"transactionHash": txHash, "blockHash": blockHash, "blockNumber": "0x10",
				"root": root, "gasUsed": "0x5000", "cumulativeGasUsed": "0x5000",
				"logs": []any{},
			}})
		}
	}))
	defer server.Close()

	result, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "ethereum-mainnet", txHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionState != EVMTransactionExecutionUnknown || result.ReceiptRoot != root || result.ReceiptStatus != "" {
		t.Fatalf("pre-Byzantium receipt overstated or lost: %+v", result)
	}
}

func TestProbeEVMTransactionNotFoundUsesSentinel(t *testing.T) {
	txHash := "0x" + strings.Repeat("9", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_getTransactionByHash":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": nil})
		}
	}))
	defer server.Close()

	_, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "ethereum-mainnet", txHash)
	if !errors.Is(err, ErrEVMTransactionNotFound) {
		t.Fatalf("err=%v want ErrEVMTransactionNotFound", err)
	}
}
