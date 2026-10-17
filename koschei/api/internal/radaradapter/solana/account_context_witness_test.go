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

func TestAccountResponseContextSlotExactRawJSON(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		minimum uint64
		want    uint64
		valid   bool
	}{
		{"same finalized slot", `{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":42},"value":{}}}`, 42, 42, true},
		{"newer account slot", `{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":43},"value":{}}}`, 42, 43, true},
		{"older account slot", `{"result":{"context":{"slot":41}}}`, 42, 0, false},
		{"missing result", `{"jsonrpc":"2.0","id":1}`, 42, 0, false},
		{"missing context", `{"result":{"value":{}}}`, 42, 0, false},
		{"missing slot", `{"result":{"context":{}}}`, 42, 0, false},
		{"duplicate result", `{"result":{"context":{"slot":42}},"result":{"context":{"slot":43}}}`, 42, 0, false},
		{"duplicate context", `{"result":{"context":{"slot":42},"context":{"slot":43}}}`, 42, 0, false},
		{"duplicate slot", `{"result":{"context":{"slot":42,"slot":43}}}`, 42, 0, false},
		{"slot as string", `{"result":{"context":{"slot":"42"}}}`, 42, 0, false},
		{"slot as float", `{"result":{"context":{"slot":42.0}}}`, 42, 0, false},
		{"slot as exponent", `{"result":{"context":{"slot":4.2e1}}}`, 42, 0, false},
		{"negative slot", `{"result":{"context":{"slot":-1}}}`, 42, 0, false},
		{"zero slot", `{"result":{"context":{"slot":0}}}`, 42, 0, false},
		{"uint64 overflow", `{"result":{"context":{"slot":18446744073709551616}}}`, 42, 0, false},
		{"null context", `{"result":{"context":null}}`, 42, 0, false},
		{"null result", `{"result":null}`, 42, 0, false},
		{"malformed JSON", `{"result":`, 42, 0, false},
		{"trailing JSON", `{"result":{"context":{"slot":42}}}{}`, 42, 0, false},
		{"unanchored minimum", `{"result":{"context":{"slot":42}}}`, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := accountResponseContextSlot([]byte(tc.body), tc.minimum)
			if tc.valid && (err != nil || got != tc.want) {
				t.Fatalf("valid account slot got=%d want=%d err=%v", got, tc.want, err)
			}
			if !tc.valid && (err == nil || got != 0) {
				t.Fatalf("invalid account slot accepted got=%d err=%v", got, err)
			}
		})
	}
}

func TestSolanaCollectorUsesAccountResponseSlotNotLowerBound(t *testing.T) {
	data := make([]byte, 36)
	binary.LittleEndian.PutUint32(data, ProgramState)
	requests := map[string]int{}
	transport := radar.TransportFunc(func(_ context.Context, q radar.Request) (radar.Response, error) {
		body, ok := q.Body.(map[string]any)
		if !ok {
			return radar.Response{}, errors.New("unexpected transport request")
		}
		method, _ := body["method"].(string)
		requests[method]++
		var result any
		switch method {
		case "getSlot":
			result = uint64(42)
		case "getAccountInfo":
			result = map[string]any{
				"context": map[string]any{"slot": uint64(43)},
				"value": map[string]any{
					"owner": UpgradeableLoaderID,
					"executable": true,
					"data": []string{base64.StdEncoding.EncodeToString(data), "base64"},
				},
			}
		default:
			return radar.Response{}, errors.New("unexpected RPC method")
		}
		raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
		if err != nil {
			return radar.Response{}, err
		}
		return radar.Response{Raw: raw, Host: "fixture.invalid"}, nil
	})
	target := radar.Target{Network: "solana-mainnet", Kind: "program", Raw: "11111111111111111111111111111111"}
	adapter, err := New(target, transport)
	if err != nil {
		t.Fatal(err)
	}
	rows, limits, err := adapter.Collect(context.Background(), target)
	if err != nil || len(rows) != 1 {
		t.Fatalf("collector observation len=%d limitations=%#v err=%v", len(rows), limits, err)
	}
	row := rows[0]
	if row.Height != "43" || row.Attributes["height"] != "43" ||
		row.Attributes["account_context_slot"] != "43" || row.Attributes["requested_min_context_slot"] != "42" ||
		row.Attributes["source_integrity_chain_verified"] != false || row.Status != "observed" {
		t.Fatalf("source account slot not reflected in observation: %#v", row)
	}
	if len(limits) == 0 || limits[0].Code != "solana_non_atomic_reads" {
		t.Fatalf("non-atomic read limitation dropped: %#v", limits)
	}
	if requests["getSlot"] != 1 || requests["getAccountInfo"] != 1 || len(requests) != 2 {
		t.Fatalf("context validation caused extra RPC: %#v", requests)
	}
}
