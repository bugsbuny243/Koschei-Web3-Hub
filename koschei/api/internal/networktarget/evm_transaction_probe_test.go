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

func evmTransactionTestServer(t *testing.T, chainID string, tx any, receipt any) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		var result any
		switch req.Method {
		case "eth_chainId":
			result = chainID
		case "eth_getTransactionByHash":
			result = tx
		case "eth_getTransactionReceipt":
			result = receipt
		default:
			t.Fatalf("unexpected RPC method %q", req.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		})
	}))
}

func evmTransactionFixture(hash string, mined bool) map[string]any {
	out := map[string]any{
		"hash":     hash,
		"from":     "0x1111111111111111111111111111111111111111",
		"to":       "0x2222222222222222222222222222222222222222",
		"value":    "0x0",
		"nonce":    "0x1",
		"gas":      "0x5208",
		"gasPrice": "0x1",
		"type":     "0x2",
		"input":    "0x1234",
	}
	if mined {
		out["blockHash"] = "0x" + strings.Repeat("c", 64)
		out["blockNumber"] = "0x10"
	} else {
		out["blockHash"] = nil
		out["blockNumber"] = nil
	}
	return out
}

func evmReceiptFixture(hash string, status string) map[string]any {
	return map[string]any{
		"transactionHash":   hash,
		"blockHash":         "0x" + strings.Repeat("c", 64),
		"blockNumber":       "0x10",
		"status":            status,
		"gasUsed":           "0x5000",
		"cumulativeGasUsed": "0x5000",
		"effectiveGasPrice": "0x4",
		"contractAddress":   nil,
		"logs": []any{
			map[string]any{
				"address":  "0x3333333333333333333333333333333333333333",
				"topics":   []string{"0x" + strings.Repeat("b", 64)},
				"logIndex": "0x0",
				"removed":  false,
				"data":     "0x" + strings.Repeat("00", 32),
			},
		},
	}
}

func TestProbeEVMTransactionBindsReceiptAndExecutionState(t *testing.T) {
	txHash := "0x" + strings.Repeat("a", 64)
	server := evmTransactionTestServer(t, "0x1", evmTransactionFixture(txHash, true), evmReceiptFixture(txHash, "0x1"))
	defer server.Close()

	result, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "ethereum-mainnet", txHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionState != EVMTransactionExecutionSuccess || result.ReceiptStatus != "0x1" {
		t.Fatalf("unexpected execution result: %+v", result)
	}
	if result.BlockNumber != "0x10" || result.InputBytes != 2 || result.InputSHA256 == "" {
		t.Fatalf("incomplete transaction evidence: %+v", result)
	}
	if len(result.Logs) != 1 || result.Logs[0].DataBytes != 32 || result.Logs[0].DataSHA256 == "" {
		t.Fatalf("structured log evidence missing: %+v", result.Logs)
	}
}

func TestProbeEVMTransactionKeepsUnminedMissingReceiptPending(t *testing.T) {
	txHash := "0x" + strings.Repeat("d", 64)
	server := evmTransactionTestServer(t, "0x2105", evmTransactionFixture(txHash, false), nil)
	defer server.Close()

	result, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "base-mainnet", txHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionState != EVMTransactionExecutionPending || result.BlockHash != "" || result.ReceiptStatus != "" {
		t.Fatalf("unmined transaction was overstated: %+v", result)
	}
}

func TestProbeEVMTransactionMissingReceiptAfterMinedTransactionIsUnknown(t *testing.T) {
	txHash := "0x" + strings.Repeat("e", 64)
	server := evmTransactionTestServer(t, "0x1", evmTransactionFixture(txHash, true), nil)
	defer server.Close()

	result, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "ethereum-mainnet", txHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionState != EVMTransactionExecutionUnknown || result.BlockHash == "" || result.BlockNumber != "0x10" {
		t.Fatalf("mined transaction with missing receipt was overstated: %+v", result)
	}
}

func TestProbeEVMTransactionPreservesPreByzantiumRootAsUnknown(t *testing.T) {
	txHash := "0x" + strings.Repeat("f", 64)
	root := "0x" + strings.Repeat("d", 64)
	receipt := evmReceiptFixture(txHash, "")
	receipt["root"] = root
	server := evmTransactionTestServer(t, "0x1", evmTransactionFixture(txHash, true), receipt)
	defer server.Close()

	result, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "ethereum-mainnet", txHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionState != EVMTransactionExecutionUnknown || result.ReceiptRoot != root || result.ReceiptStatus != "" {
		t.Fatalf("pre-Byzantium receipt evidence was lost or overstated: %+v", result)
	}
}

func TestProbeEVMTransactionNotFoundUsesSentinel(t *testing.T) {
	txHash := "0x" + strings.Repeat("9", 64)
	server := evmTransactionTestServer(t, "0x1", nil, nil)
	defer server.Close()

	_, err := ProbeEVMTransaction(context.Background(), server.Client(), server.URL, "ethereum-mainnet", txHash)
	if !errors.Is(err, ErrEVMTransactionNotFound) {
		t.Fatalf("err=%v want ErrEVMTransactionNotFound", err)
	}
}

func TestProbeEVMTransactionRejectsBadHashBeforeRPC(t *testing.T) {
	if _, err := ProbeEVMTransaction(context.Background(), http.DefaultClient, "https://example.com", "ethereum-mainnet", "0x1234"); err == nil {
		t.Fatal("invalid transaction hash was accepted")
	}
}

func TestEVMTransactionResponseLimitAllowsLargeConsensusPayloads(t *testing.T) {
	if evmTransactionResponseLimit < 128*1024*1024 {
		t.Fatalf("transaction response limit=%d want at least 128 MiB", evmTransactionResponseLimit)
	}
}
