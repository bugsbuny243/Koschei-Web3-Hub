package networktarget

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const erc20AllowanceSelector = "dd62ed3e"

type EVMAllowanceProbeResult struct {
	SchemaVersion    string `json:"schema_version"`
	Network          string `json:"network"`
	ChainID          string `json:"chain_id"`
	ExpectedChainID  string `json:"expected_chain_id"`
	Token            string `json:"token"`
	Owner            string `json:"owner"`
	Spender          string `json:"spender"`
	Amount           string `json:"amount"`
	Unlimited        bool   `json:"unlimited"`
	EvidenceStatus   string `json:"evidence_status"`
	LiveAvailability string `json:"live_availability"`
}

// ProbeEVMAllowance performs a bounded read-only ERC-20 allowance(owner,spender)
// eth_call after verifying that the configured endpoint belongs to the requested
// EVM network. The returned value is current observed state only: it does not
// prove how the allowance was created, who controls the spender, or that spending
// the allowance would be safe.
func ProbeEVMAllowance(ctx context.Context, client *http.Client, endpoint, network, token, owner, spender string) (EVMAllowanceProbeResult, error) {
	expectedChainID, ok := ExpectedEVMChainID(strings.TrimSpace(network))
	if !ok {
		return EVMAllowanceProbeResult{}, fmt.Errorf("evm_allowance_unsupported_network")
	}
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return EVMAllowanceProbeResult{}, fmt.Errorf("evm_rpc_endpoint_invalid")
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	token, err = normalizeEVMCallAddress(token)
	if err != nil {
		return EVMAllowanceProbeResult{}, fmt.Errorf("evm_allowance_token_invalid")
	}
	owner, err = normalizeEVMCallAddress(owner)
	if err != nil {
		return EVMAllowanceProbeResult{}, fmt.Errorf("evm_allowance_owner_invalid")
	}
	spender, err = normalizeEVMCallAddress(spender)
	if err != nil {
		return EVMAllowanceProbeResult{}, fmt.Errorf("evm_allowance_spender_invalid")
	}

	chainID, err := evmRPCString(ctx, client, endpoint, 1, "eth_chainId", nil)
	if err != nil {
		return EVMAllowanceProbeResult{}, fmt.Errorf("evm_chain_id_unavailable: %w", err)
	}
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	if chainID != expectedChainID {
		return EVMAllowanceProbeResult{}, fmt.Errorf("evm_rpc_network_mismatch")
	}

	callData := "0x" + erc20AllowanceSelector + abiAddressWord(owner) + abiAddressWord(spender)
	value, err := evmRPCString(ctx, client, endpoint, 2, "eth_call", []any{
		map[string]any{"to": token, "data": callData},
		"latest",
	})
	if err != nil {
		return EVMAllowanceProbeResult{}, fmt.Errorf("evm_allowance_unavailable: %w", err)
	}
	amount, err := parseEVMUint256(value)
	if err != nil {
		return EVMAllowanceProbeResult{}, fmt.Errorf("evm_allowance_result_invalid")
	}

	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	return EVMAllowanceProbeResult{
		SchemaVersion:    SchemaVersion,
		Network:          strings.TrimSpace(network),
		ChainID:          chainID,
		ExpectedChainID:  expectedChainID,
		Token:            token,
		Owner:            owner,
		Spender:          spender,
		Amount:           amount.String(),
		Unlimited:        amount.Cmp(maxUint256) == 0,
		EvidenceStatus:   "observed",
		LiveAvailability: "checked",
	}, nil
}

func normalizeEVMCallAddress(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 42 || !strings.HasPrefix(value, "0x") {
		return "", fmt.Errorf("invalid_evm_address")
	}
	for _, ch := range value[2:] {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return "", fmt.Errorf("invalid_evm_address")
		}
	}
	return value, nil
}

func abiAddressWord(address string) string {
	return strings.Repeat("0", 24) + strings.TrimPrefix(address, "0x")
}

func parseEVMUint256(value string) (*big.Int, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(value, "0x") {
		return nil, fmt.Errorf("invalid_uint256")
	}
	hexValue := strings.TrimPrefix(value, "0x")
	if hexValue == "" || len(hexValue) > 64 {
		return nil, fmt.Errorf("invalid_uint256")
	}
	for _, ch := range hexValue {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return nil, fmt.Errorf("invalid_uint256")
		}
	}
	amount := new(big.Int)
	if _, ok := amount.SetString(hexValue, 16); !ok {
		return nil, fmt.Errorf("invalid_uint256")
	}
	return amount, nil
}
