package solana

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"

	radar "koschei/api/internal/radaradapter"
)

func TestSolanaProgramCollectorBindsAccountJSONWithoutExtraRPC(t *testing.T) {
	accountData := make([]byte, 36)
	binary.LittleEndian.PutUint32(accountData, ProgramState)
	requests := map[string]int{}
	transport := radar.TransportFunc(func(_ context.Context, query radar.Request) (radar.Response, error) {
		body, ok := query.Body.(map[string]any)
		if !ok {
			return radar.Response{}, errors.New("missing RPC request")
		}
		method, _ := body["method"].(string)
		requests[method]++
		var value any
		switch method {
		case "getSlot":
			value = uint64(42)
		case "getAccountInfo":
			value = map[string]any{
				"context": map[string]any{"slot": uint64(42)},
				"value": map[string]any{
					"owner": UpgradeableLoaderID, "executable": true,
					"data": []string{base64.StdEncoding.EncodeToString(accountData), "base64"},
				},
			}
		default:
			return radar.Response{}, errors.New("unexpected RPC method")
		}
		b, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": value})
		if err != nil {
			return radar.Response{}, err
		}
		return radar.Response{Raw: b, Host: "test-rpc.fixture.invalid"}, nil
	})
	target := radar.Target{Network: "solana-mainnet", Kind: "program", Raw: "11111111111111111111111111111111"}
	adapter, err := New(target, transport)
	if err != nil {
		t.Fatal(err)
	}
	rows, limits, err := adapter.Collect(context.Background(), target)
	if err != nil || len(rows) != 1 {
		t.Fatalf("collector evidence len=%d limitations=%#v err=%v", len(rows), limits, err)
	}
	if requests["getSlot"] != 1 || requests["getAccountInfo"] != 1 || len(requests) != 2 {
		t.Fatalf("witness introduced an extra RPC request: %#v", requests)
	}
	got := rows[0]
	if got.Status != "observed" ||
		got.Attributes["source_integrity_scope"] != "collector_raw_json_bytes_match_declared_digest_only" ||
		len(got.Attributes["source_integrity_commitment_sha256"].(string)) != 64 ||
		got.Attributes["source_integrity_chain_verified"] != false {
		t.Fatalf("source integrity was not observational: %#v", got)
	}
	// A read with minContextSlot does NOT prove it was atomic with getSlot.
	if len(limits) == 0 || limits[0].Code != "solana_non_atomic_reads" {
		t.Fatalf("lost non-atomic read limitation: %#v", limits)
	}
}
