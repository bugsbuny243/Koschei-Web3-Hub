package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// UnmarshalJSON rejects malformed identity policy values before the ordinary
// normalization pass. Silently dropping a misspelled expected or blocked
// program would weaken the caller's declared policy and create a fail-open
// decision boundary.
func (input *transactionGuardV2Request) UnmarshalJSON(data []byte) error {
	type requestAlias transactionGuardV2Request
	var decoded requestAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	if wallet := strings.TrimSpace(decoded.Wallet); wallet != "" && !looksLikeGuardPubkey(wallet) {
		return fmt.Errorf("wallet has an invalid Solana address")
	}
	for name, values := range map[string][]string{
		"expected_programs": decoded.ExpectedPrograms,
		"required_programs": decoded.RequiredPrograms,
		"blocked_programs":  decoded.BlockedPrograms,
	} {
		for index, value := range values {
			if !looksLikeGuardPubkey(strings.TrimSpace(value)) {
				return fmt.Errorf("%s[%d] has an invalid Solana program address", name, index)
			}
		}
	}
	for index, account := range decoded.Accounts {
		if mint := strings.TrimSpace(account.Mint); mint != "" && !looksLikeGuardPubkey(mint) {
			return fmt.Errorf("accounts[%d].mint has an invalid Solana address", index)
		}
	}

	*input = transactionGuardV2Request(decoded)
	return nil
}
