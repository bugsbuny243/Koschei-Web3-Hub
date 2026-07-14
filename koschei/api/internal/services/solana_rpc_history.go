package services

import (
	"context"
	"fmt"
	"strings"
)

// SolanaSignaturePageOptions maps directly to getSignaturesForAddress cursor
// semantics. It exists separately from the small recent-history helper so broad
// callers cannot accidentally trigger pagination.
type SolanaSignaturePageOptions struct {
	Limit  int
	Before string
	Until  string
}

func SolanaGetSignaturesForAddressPage(ctx context.Context, rpcURL, address string, options SolanaSignaturePageOptions) ([]SolanaSignatureInfo, error) {
	rpcURL = strings.TrimSpace(rpcURL)
	address = strings.TrimSpace(address)
	if rpcURL == "" {
		return nil, fmt.Errorf("solana rpc url is empty")
	}
	if address == "" {
		return nil, fmt.Errorf("solana address is empty")
	}
	limit := options.Limit
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	config := map[string]any{"limit": limit}
	if before := strings.TrimSpace(options.Before); before != "" {
		config["before"] = before
	}
	if until := strings.TrimSpace(options.Until); until != "" {
		config["until"] = until
	}
	return solanaRPCDo[[]SolanaSignatureInfo](ctx, rpcURL, "getSignaturesForAddress", []any{address, config})
}
