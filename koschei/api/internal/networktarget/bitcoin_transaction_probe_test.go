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
				"vin":  []any{map[string]any{"txid": strings.Repeat("c", 64)}},
				"vout": []any{map[string]any{"value": 123}},
				"size": 180, "weight": 720, "fee": 900,
				"status": map[string]any{"confirmed": true, "block_height": 900000, "block_hash": blockHash, "block_time": 1700000000},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	got, err := ProbeBitcoinTransaction(t.Context(), server.Client(), server.URL, txid)
	if err != nil {\n\t\tt.Fatal(err)\n\t}
	if got.TransactionID != txid || !got.Confirmed || got.BlockHeight != 900000 || got.InputCount != 1 || got.OutputCount != 1 || got.FeeSats != 900 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if len(got.TransactionSHA256) != 64 || len(got.GenesisResponseSHA256) != 64 {
		t.Fatalf("missing evidence digests: %+v", got)
	}
}
