package services

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
)

const maxTransactionGuardAccounts = 32

type SolanaSimulationAccountsResult struct {
	Context struct {
		Slot int64 `json:"slot"`
	} `json:"context"`
	Value SolanaSimulationAccountsValue `json:"value"`
}

type SolanaSimulationAccountsValue struct {
	Err                  any                  `json:"err"`
	Logs                 []string             `json:"logs"`
	Accounts             []*SolanaAccountInfo `json:"accounts"`
	UnitsConsumed        *int64               `json:"unitsConsumed"`
	ReturnData           any                  `json:"returnData"`
	InnerInstructions    any                  `json:"innerInstructions"`
	ReplacementBlockhash any                  `json:"replacementBlockhash"`
}

func SolanaGetMultipleAccountsBase64(ctx context.Context, rpcURL string, addresses []string) (SolanaMultipleAccountInfoResult, []string, error) {
	rpcURL = resolvedSolanaRPCURL(rpcURL)
	clean := cleanGuardAccountAddresses(addresses)
	if rpcURL == "" {
		return SolanaMultipleAccountInfoResult{}, nil, fmt.Errorf("solana rpc url is empty")
	}
	if len(clean) == 0 {
		return SolanaMultipleAccountInfoResult{}, nil, fmt.Errorf("transaction guard account list is empty")
	}
	result, err := solanaRPCDo[SolanaMultipleAccountInfoResult](ctx, rpcURL, "getMultipleAccounts", []any{
		clean,
		map[string]any{"encoding": "base64", "commitment": "processed"},
	})
	return result, clean, err
}

func SolanaSimulateTransactionWithAccountsBase64(ctx context.Context, rpcURL, transaction, encoding string, addresses []string) (SolanaSimulationAccountsResult, []string, error) {
	rpcURL = resolvedSolanaRPCURL(rpcURL)
	transaction = strings.TrimSpace(transaction)
	encoding = strings.ToLower(strings.TrimSpace(encoding))
	clean := cleanGuardAccountAddresses(addresses)
	if rpcURL == "" {
		return SolanaSimulationAccountsResult{}, nil, fmt.Errorf("solana rpc url is empty")
	}
	if transaction == "" {
		return SolanaSimulationAccountsResult{}, nil, fmt.Errorf("serialized transaction is empty")
	}
	if encoding == "" {
		encoding = "base64"
	}
	if encoding != "base64" {
		return SolanaSimulationAccountsResult{}, nil, fmt.Errorf("unsupported transaction encoding: %s", encoding)
	}
	if len(clean) == 0 {
		return SolanaSimulationAccountsResult{}, nil, fmt.Errorf("transaction guard account list is empty")
	}
	config := map[string]any{
		"encoding":               encoding,
		"commitment":             "processed",
		"sigVerify":              false,
		"replaceRecentBlockhash": true,
		"innerInstructions":      true,
		"accounts": map[string]any{
			"encoding":  "base64",
			"addresses": clean,
		},
	}
	result, err := solanaRPCDo[SolanaSimulationAccountsResult](ctx, rpcURL, "simulateTransaction", []any{transaction, config})
	return result, clean, err
}

func SolanaTokenAccountRawAmount(info *SolanaAccountInfo) (uint64, error) {
	if info == nil {
		return 0, fmt.Errorf("token account is unavailable")
	}
	data, err := solanaAccountDataBytes(info.Data)
	if err != nil {
		return 0, err
	}
	if len(data) < 72 {
		return 0, fmt.Errorf("token account data is too short")
	}
	return binary.LittleEndian.Uint64(data[64:72]), nil
}

func solanaAccountDataBytes(raw any) ([]byte, error) {
	encoded := ""
	switch value := raw.(type) {
	case []any:
		if len(value) > 0 {
			encoded, _ = value[0].(string)
		}
	case []string:
		if len(value) > 0 {
			encoded = value[0]
		}
	case string:
		encoded = value
	}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, fmt.Errorf("base64 account data is unavailable")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode token account data: %w", err)
	}
	return decoded, nil
}

func cleanGuardAccountAddresses(addresses []string) []string {
	out := make([]string, 0, len(addresses))
	seen := map[string]bool{}
	for _, address := range addresses {
		address = strings.TrimSpace(address)
		if address == "" || seen[address] {
			continue
		}
		seen[address] = true
		out = append(out, address)
		if len(out) == maxTransactionGuardAccounts {
			break
		}
	}
	return out
}
