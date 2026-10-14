package solana

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"

	r "koschei/api/internal/radaradapter"
)

func TestAdapterProgramEvidence(t *testing.T) {
	program := make([]byte, 36)
	binary.LittleEndian.PutUint32(program, ProgramState)
	program[4] = 9
	transport := r.TransportFunc(func(_ context.Context, q r.Request) (r.Response, error) {
		body, ok := q.Body.(map[string]any)
		if !ok {
			return r.Response{}, errors.New("missing request")
		}
		var result any
		switch body["method"] {
		case "getSlot":
			result = uint64(120)
		case "getAccountInfo":
			result = map[string]any{
				"context": map[string]any{"slot": uint64(120)},
				"value": map[string]any{
					"owner": UpgradeableLoaderID, "executable": true, "lamports": uint64(10),
					"data": []string{base64.StdEncoding.EncodeToString(program), "base64"},
				},
			}
		default:
			return r.Response{}, errors.New("unexpected method")
		}
		payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
		return r.Response{Raw: payload, Host: "solana.fixture.invalid"}, nil
	})
	target := r.Target{Network: "solana-mainnet", Kind: "program", Raw: "11111111111111111111111111111111"}
	adapter, err := New(target, transport)
	if err != nil {
		t.Fatal(err)
	}
	rows, limits, err := adapter.Collect(context.Background(), target)
	if err != nil || len(rows) != 1 || len(limits) == 0 {
		t.Fatalf("collection: %d %d %v", len(rows), len(limits), err)
	}
	if err := r.ValidateRow(rows[0], adapter.Subject()); err != nil {
		t.Fatal(err)
	}
	if _, err := New(r.Target{Network: "solana-mainnet", Kind: "program", Raw: "bad"}, transport); err == nil {
		t.Fatal("invalid pubkey accepted")
	}
}
