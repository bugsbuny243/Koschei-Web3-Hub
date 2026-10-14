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

func TestReadFinalizedSlotAndAccountContext(t *testing.T) {
	data := make([]byte, 36)
	binary.LittleEndian.PutUint32(data, ProgramState)
	provider := radar.TransportFunc(func(_ context.Context, q radar.Request) (radar.Response, error) {
		body, ok := q.Body.(map[string]any)
		if !ok {
			return radar.Response{}, errors.New("missing request")
		}
		method, _ := body["method"].(string)
		var result any
		switch method {
		case "getSlot":
			result = uint64(42)
		case "getAccountInfo":
			result = map[string]any{"context": map[string]any{"slot": uint64(42)}, "value": map[string]any{"owner": UpgradeableLoaderID, "executable": true, "data": []string{base64.StdEncoding.EncodeToString(data), "base64"}}}
		default:
			return radar.Response{}, errors.New("unexpected RPC method")
		}
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
		return radar.Response{Raw: b, Host: "fixture.invalid"}, nil
	})
	slot, err := ReadFinalizedSlot(context.Background(), provider, "solana-mainnet")
	if err != nil || slot != 42 {
		t.Fatalf("slot: %d %v", slot, err)
	}
	_, account, err := ReadAccountAtSlot(context.Background(), provider, "solana-mainnet", "11111111111111111111111111111111", slot)
	if err != nil || len(account.Data) != 36 {
		t.Fatalf("account: %v", err)
	}
}

func TestSolanaPubkeyValidation(t *testing.T) {
	key, err := DecodeSolanaPubkey("11111111111111111111111111111111")
	if err != nil || key != ([32]byte{}) {
		t.Fatalf("zero key: %v", err)
	}
	for _, bad := range []string{"", "0OIl1111111111111111111111111111", "111111111111111111111111111111111"} {
		if _, err := DecodeSolanaPubkey(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
