package handlers

import "encoding/json"

// MarshalJSON keeps caller-declared token metadata separate from the RPC-
// verified balance delta. Until the raw token-account mint bytes are compared
// with the declaration, the response must not imply that the mint itself was
// verified by Koschei.
func (value transactionGuardAccountDelta) MarshalJSON() ([]byte, error) {
	type accountDeltaOutput struct {
		Address           string `json:"address"`
		DeclaredMint      string `json:"declared_mint,omitempty"`
		MintVerified      bool   `json:"mint_verified"`
		Role              string `json:"role"`
		Decimals          *int   `json:"decimals,omitempty"`
		PreAmountRaw      string `json:"pre_amount_raw"`
		PostAmountRaw     string `json:"post_amount_raw"`
		DeltaRaw          string `json:"delta_raw"`
		SpentRaw          string `json:"spent_raw,omitempty"`
		ReceivedRaw       string `json:"received_raw,omitempty"`
		MaximumSpendRaw   string `json:"maximum_spend_raw,omitempty"`
		MinimumReceiveRaw string `json:"minimum_receive_raw,omitempty"`
		QuotedReceiveRaw  string `json:"quoted_receive_raw,omitempty"`
		SlippageBPS       *int64 `json:"slippage_bps,omitempty"`
		MaxSlippageBPS    int    `json:"max_slippage_bps,omitempty"`
		PolicyStatus      string `json:"policy_status"`
		EvidenceStatus    string `json:"evidence_status"`
	}
	return json.Marshal(accountDeltaOutput{
		Address: value.Address, DeclaredMint: value.Mint, MintVerified: false,
		Role: value.Role, Decimals: value.Decimals,
		PreAmountRaw: value.PreAmountRaw, PostAmountRaw: value.PostAmountRaw, DeltaRaw: value.DeltaRaw,
		SpentRaw: value.SpentRaw, ReceivedRaw: value.ReceivedRaw,
		MaximumSpendRaw: value.MaximumSpendRaw, MinimumReceiveRaw: value.MinimumReceiveRaw,
		QuotedReceiveRaw: value.QuotedReceiveRaw, SlippageBPS: value.SlippageBPS, MaxSlippageBPS: value.MaxSlippageBPS,
		PolicyStatus: value.PolicyStatus, EvidenceStatus: value.EvidenceStatus,
	})
}
