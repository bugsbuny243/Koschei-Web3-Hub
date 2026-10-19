package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func TestCanonicalNativeEVMTransactionRetainsPendingWithoutSafetyGrade(t *testing.T) {
	txid := "0x" + strings.Repeat("a", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		var result any
		switch req.Method {
		case "eth_chainId":
			result = "0x2105"
		case "eth_getTransactionByHash":
			result = map[string]any{
				"hash":        txid,
				"from":        nativeJobEVMFixture,
				"to":          "0x2222222222222222222222222222222222222222",
				"value":       "0x0",
				"nonce":       "0x1",
				"gas":         "0x5208",
				"gasPrice":    "0x1",
				"type":        "0x2",
				"input":       "0x",
				"blockHash":   nil,
				"blockNumber": nil,
			}
		case "eth_getTransactionReceipt":
			result = nil
		default:
			t.Errorf("unexpected method %q", req.Method)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()
	t.Setenv("BASE_RPC_URL", server.URL)
	report, err := canonicalNativeNetworkJobWithClient(t.Context(), server.Client(), txid, "base-mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if report["investigation_kind"] != "native_network_transaction_observation" || report["status"] != "evidence_only_partial" {
		t.Fatalf("transaction overstated completion: %#v", report)
	}
	result := report["customer_scan"].(services.CustomerScanResult)
	if result.TransactionEvidence == nil || result.TransactionEvidence.TransactionHash != txid || result.Trust.Verified {
		t.Fatalf("pending transaction not faithfully preserved: %#v", result)
	}
	if report["final_verdict"].(map[string]any)["signed"] != false {
		t.Fatal("pending transaction fabricated signed grade")
	}
}

func TestCanonicalNativeBitcoinTransactionRetainsUTXOFlow(t *testing.T) {
	txid := strings.Repeat("a", 64)
	blockHash := strings.Repeat("b", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/block-height/0":
			_, _ = w.Write([]byte(nativeJobBitcoinGenesis))
		case "/tx/" + txid:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"txid": txid, "version": 2, "locktime": 0,
				"vin": []any{map[string]any{
					"txid": strings.Repeat("c", 64), "vout": 1, "is_coinbase": false, "sequence": 4294967293,
					"prevout": map[string]any{
						"scriptpubkey":         "76a914" + strings.Repeat("1", 40) + "88ac",
						"scriptpubkey_type":    "p2pkh",
						"scriptpubkey_address": nativeJobBitcoinFixture,
						"value":                1500,
					},
				}},
				"vout": []any{map[string]any{
					"scriptpubkey":         "76a914" + strings.Repeat("2", 40) + "88ac",
					"scriptpubkey_type":    "p2pkh",
					"scriptpubkey_address": nativeJobBitcoinFixture,
					"value":                600,
				}},
				"size": 180, "weight": 720, "fee": 900,
				"status": map[string]any{"confirmed": true, "block_height": 900000, "block_hash": blockHash, "block_time": 1700000000},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("BITCOIN_ESPLORA_URL", server.URL)
	report, err := canonicalNativeNetworkJobWithClient(t.Context(), server.Client(), txid, "bitcoin-mainnet")
	if err != nil {
		t.Fatal(err)
	}
	result := report["customer_scan"].(services.CustomerScanResult)
	if result.TransactionEvidence == nil || result.TransactionEvidence.Network != "bitcoin-mainnet" || result.Trust.Verified {
		t.Fatalf("Bitcoin transaction network/evidence mismatch: %#v", result)
	}
	attr := result.TransactionEvidence.Attributes
	if attr["fee_sats"] == nil && attr["fee_satoshis"] == nil {
		t.Fatalf("BTC fee flow evidence missing: %#v", attr)
	}
	inputs, inputsOK := attr["input_flows"].([]map[string]any)
	outputs, outputsOK := attr["output_flows"].([]map[string]any)
	if !inputsOK || !outputsOK || len(inputs) != 1 || len(outputs) != 1 {
		t.Fatalf("BTC inputs and outputs were truncated in customer evidence: %#v", attr)
	}
	if inputs[0]["previous_value_sats"] != int64(1500) || outputs[0]["value_sats"] != int64(600) ||
		attr["derived_fee_sats"] != int64(900) || attr["fee_consistent"] != true ||
		attr["transaction_response_sha256"] == "" {
		t.Fatalf("BTC value flow integrity or source digest missing: %#v", attr)
	}
	if report["final_verdict"].(map[string]any)["withheld"] != true {
		t.Fatal("Bitcoin UTXO flow observation fabricated safety grade")
	}
}
