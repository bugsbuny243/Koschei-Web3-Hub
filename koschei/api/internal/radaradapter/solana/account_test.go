package solana

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestAccountInfoStrictEnvelope(t *testing.T) {
	program := make([]byte, 36)
	binary.LittleEndian.PutUint32(program, ProgramState)
	program[4] = 17
	payload, _ := json.Marshal(map[string]any{"value": map[string]any{
		"lamports": 42, "owner": UpgradeableLoaderID, "executable": true,
		"data": []string{base64.StdEncoding.EncodeToString(program), "base64"},
	}})
	account, err := DecodeAccountInfo(payload)
	if err != nil || account.Lamports != 42 || len(account.Data) != 36 {
		t.Fatalf("valid account rejected: %+v %v", account, err)
	}
	key, err := DecodeUpgradeableProgramAccount(account)
	if err != nil || key[0] != 17 {
		t.Fatalf("valid program rejected: %v", err)
	}
	account.Owner = "foreign-owner"
	if _, err := DecodeUpgradeableProgramAccount(account); err == nil {
		t.Fatal("foreign program owner accepted")
	}
	account.Owner = UpgradeableLoaderID
	account.Executable = false
	if _, err := DecodeUpgradeableProgramAccount(account); err == nil {
		t.Fatal("non-executable program accepted")
	}
	oversized, _ := json.Marshal(map[string]any{"value": map[string]any{"owner": UpgradeableLoaderID, "data": []string{strings.Repeat("A", base64.StdEncoding.EncodedLen(maxRPCAccountBytes)+4), "base64"}}})
	if _, err := DecodeAccountInfo(oversized); err == nil {
		t.Fatal("oversized RPC account accepted")
	}
	for _, raw := range []string{
		`{"value":null}`,
		`{"value":{"owner":"owner","data":["AA==","base58"]}}`,
		`{"value":{"owner":"owner","data":["!!","base64"]}}`,
		`{"value":{"owner":"owner","data":["AA=="]}}`,
		`{"value":{"owner":"","data":["AA==","base64"]}}`,
	} {
		if _, err := DecodeAccountInfo(json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid account accepted: %s", raw)
		}
	}
}

func TestUpgradeableProgramDataOwnerAndAuthority(t *testing.T) {
	data := make([]byte, 47)
	binary.LittleEndian.PutUint32(data, ProgramDataState)
	binary.LittleEndian.PutUint64(data[4:], 123)
	data[45] = 9
	account := Account{Owner: UpgradeableLoaderID, Data: data}
	state, err := DecodeUpgradeableProgramDataAccount(account)
	if err != nil || state.Authority != nil || state.Slot != 123 {
		t.Fatalf("revoked authority rejected: %+v %v", state, err)
	}
	account.Executable = true
	if _, err := DecodeUpgradeableProgramDataAccount(account); err == nil {
		t.Fatal("executable program data accepted")
	}
	account.Executable = false
	account.Owner = "foreign-owner"
	if _, err := DecodeUpgradeableProgramDataAccount(account); err == nil {
		t.Fatal("foreign program data accepted")
	}
}
