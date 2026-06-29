package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Solana getAccountInfo wraps the account in a value field. Keeping this
// decoder separate makes the public status handler easy to test and ensures a
// configured address is an actual mint owned by a supported token program.
func (account *publicTokenMintAccount) UnmarshalJSON(data []byte) error {
	var envelope struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Value) == 0 || bytes.Equal(envelope.Value, []byte("null")) {
		return fmt.Errorf("mint account unavailable")
	}
	type accountAlias publicTokenMintAccount
	var decoded accountAlias
	if err := json.Unmarshal(envelope.Value, &decoded); err != nil {
		return err
	}
	if decoded.Owner != legacyTokenProgramID && decoded.Owner != token2022ProgramID {
		return fmt.Errorf("unsupported token program")
	}
	if decoded.Data.Parsed.Type != "mint" {
		return fmt.Errorf("configured address is not a mint")
	}
	*account = publicTokenMintAccount(decoded)
	return nil
}
