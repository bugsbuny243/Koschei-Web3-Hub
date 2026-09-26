package networktarget

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeBitcoinTransactionObserved(t *testing.T) {
	txid := strings.Repeat("a", 64)
	blockHash := strings.Repeat("b", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/block-height/0":
			_, _ = w.Write([]byte(bitcoinMainnetGenesisHash))
		case "/tx/" + txid:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"txid": txid, "version": 2, "locktime": 0,
				"vin": []any{map[string]any{
					"txid": strings.Repeat("c", 64), "vout": 1, "is_coinbase": false, "sequence": 4294967293,
					"prevout": map[string]any{
						"scriptpubkey":         "76a914" + strings.Repeat("1", 40) + "88ac",
						"scriptpubkey_type":    "p2pkh",
						"scriptpubkey_address": "1BoatSLRHtKNngkdXEeobR76b53LETtpyT",
						"value":                1500,
					},
				}},
				"vout": []any{map[string]any{
					"scriptpubkey":         "76a914" + strings.Repeat("2", 40) + "88ac",
					"scriptpubkey_type":    "p2pkh",
					"scriptpubkey_address": "1BoatSLRHtKNngkdXEeobR76b53LETtpyT",
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
	got, err := ProbeBitcoinTransaction(t.Context(), server.Client(), server.URL, txid)
	if err != nil {
		t.Fatal(err)
	}
	if got.TransactionID != txid || !got.Confirmed || got.BlockHeight != 900000 || got.InputCount != 1 || got.OutputCount != 1 || got.FeeSats != 900 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.Coinbase || !got.FeeBalanceChecked || !got.FeeConsistent || got.TotalInputSats != 1500 || got.TotalOutputSats != 600 || got.DerivedFeeSats != 900 {
		t.Fatalf("UTXO value flow evidence missing: %+v", got)
	}
	if len(got.Inputs) != 1 || got.Inputs[0].PreviousTxID != strings.Repeat("c", 64) ||
		got.Inputs[0].PreviousValueSats != 1500 || got.Inputs[0].PreviousScriptType != "p2pkh" ||
		got.Inputs[0].PreviousAddress != "1BoatSLRHtKNngkdXEeobR76b53LETtpyT" || len(got.Inputs[0].PreviousScriptPubKeySHA256) != 64 {
		t.Fatalf("Bitcoin prevout flow evidence missing: %+v", got.Inputs)
	}
	if len(got.Outputs) != 1 || got.Outputs[0].ValueSats != 600 || got.Outputs[0].ScriptType != "p2pkh" ||
		got.Outputs[0].Address != "1BoatSLRHtKNngkdXEeobR76b53LETtpyT" || len(got.Outputs[0].ScriptPubKeySHA256) != 64 {
		t.Fatalf("Bitcoin output flow evidence missing: %+v", got.Outputs)
	}
	if len(got.TransactionSHA256) != 64 || len(got.GenesisResponseSHA256) != 64 {
		t.Fatalf("missing evidence digests: %+v", got)
	}
}
