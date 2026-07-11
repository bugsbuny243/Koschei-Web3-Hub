package services

import (
	"context"
	"fmt"
	"strings"
)

// SolanaMultipleAccountInfoResult is the jsonParsed response returned by
// getMultipleAccounts. It is used to resolve token-account addresses to their
// controlling owner wallets without spending one RPC request per holder.
type SolanaMultipleAccountInfoResult struct {
	Value []*SolanaAccountInfo `json:"value"`
}

func SolanaGetMultipleAccountsJSONParsed(ctx context.Context, rpcURL string, addresses []string) (SolanaMultipleAccountInfoResult, error) {
	rpcURL = strings.TrimSpace(rpcURL)
	if rpcURL == "" {
		return SolanaMultipleAccountInfoResult{}, fmt.Errorf("solana rpc url is empty")
	}
	clean := make([]string, 0, len(addresses))
	seen := map[string]bool{}
	for _, address := range addresses {
		address = strings.TrimSpace(address)
		if address == "" || seen[address] {
			continue
		}
		seen[address] = true
		clean = append(clean, address)
		if len(clean) == 100 {
			break
		}
	}
	if len(clean) == 0 {
		return SolanaMultipleAccountInfoResult{}, fmt.Errorf("solana account list is empty")
	}
	return solanaRPCDo[SolanaMultipleAccountInfoResult](ctx, rpcURL, "getMultipleAccounts", []any{clean, map[string]any{"encoding": "jsonParsed", "commitment": "confirmed"}})
}
