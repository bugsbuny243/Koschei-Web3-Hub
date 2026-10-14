package solana

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

// UpgradeableLoaderID is the on-chain owner of upgradeable BPF program accounts.
const UpgradeableLoaderID = "BPFLoaderUpgradeab1e11111111111111111111111"

// Account is a validated getAccountInfo value; it is not proof of a live provider.
type Account struct {
	Lamports   uint64
	Owner      string
	Executable bool
	Data       []byte
}

const maxRPCAccountBytes = 16 << 20

// DecodeAccountInfo accepts only the explicit base64 RPC account encoding.
// Owner, executable and byte representation are preserved for caller checks.
func DecodeAccountInfo(raw json.RawMessage) (Account, error) {
	var envelope struct {
		Value *struct {
			Lamports   uint64          `json:"lamports"`
			Owner      string          `json:"owner"`
			Executable bool            `json:"executable"`
			Data       json.RawMessage `json:"data"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Value == nil {
		return Account{}, errors.New("missing Solana account value")
	}
	var pair []string
	if err := json.Unmarshal(envelope.Value.Data, &pair); err != nil || len(pair) != 2 || pair[1] != "base64" {
		return Account{}, errors.New("Solana account requires base64 data")
	}
	if len(pair[0]) > base64.StdEncoding.EncodedLen(maxRPCAccountBytes) {
		return Account{}, errors.New("Solana account payload exceeds limit")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(pair[0])
	if err != nil || len(data) == 0 || envelope.Value.Owner == "" {
		return Account{}, errors.New("invalid Solana account payload")
	}
	return Account{Lamports: envelope.Value.Lamports, Owner: envelope.Value.Owner, Executable: envelope.Value.Executable, Data: data}, nil
}

// DecodeUpgradeableProgramAccount fails closed on incorrect ownership or flags.
func DecodeUpgradeableProgramAccount(account Account) ([32]byte, error) {
	if account.Owner != UpgradeableLoaderID || !account.Executable {
		return [32]byte{}, errors.New("account is not an executable upgradeable program")
	}
	return DecodeProgram(account.Data)
}

// DecodeUpgradeableProgramDataAccount requires the same loader owner and a
// non-executable data account. Authority None is a valid revoked state.
func DecodeUpgradeableProgramDataAccount(account Account) (ProgramData, error) {
	if account.Owner != UpgradeableLoaderID || account.Executable {
		return ProgramData{}, errors.New("account is not upgradeable program data")
	}
	return DecodeProgramData(account.Data)
}
