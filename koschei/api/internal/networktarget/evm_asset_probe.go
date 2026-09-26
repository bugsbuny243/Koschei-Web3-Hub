package networktarget

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	evmERC20TotalSupplySelector	= "18160ddd"
	evmERC20BalanceOfSelector	= "70a08231"
	evmERC20DecimalsSelector	= "313ce567"
	evmERC20SymbolSelector	= "95d89b41"
	evmERC20NameSelector		= "06fdde03"
)

var ErrEVMERC20SurfaceNotObserved = errors.New("evm_erc20_surface_not_observed")

type EVMAssetProbeResult struct {
	SchemaVersion			string	`json:"schema_version"`
	Network				string	`json:"network"`
	ChainID				string	`json:"chain_id"`
	ExpectedChainID			string	`json:"expected_chain_id"`
	Contract			string	`json:"contract"`
	InterfaceState			string	`json:"interface_state"`
	TotalSupply			string	`json:"total_supply"`
	ContractBalance			string	`json:"contract_balance"`
	Decimals			uint64	`json:"decimals,omitempty"`
	DecimalsObserved		bool	`json:"decimals_observed"`
	Symbol				string	`json:"symbol,omitempty"`
	SymbolEncoding			string	`json:"symbol_encoding,omitempty"`
	SymbolObserved			bool	`json:"symbol_observed"`
	Name				string	`json:"name,omitempty"`
	NameEncoding			string	`json:"name_encoding,omitempty"`
	NameObserved			bool	`json:"name_observed"`
	TotalSupplyResponseSHA256	string	`json:"total_supply_response_sha256"`
	ContractBalanceResponseSHA256	string	`json:"contract_balance_response_sha256"`
	DecimalsResponseSHA256		string	`json:"decimals_response_sha256,omitempty"`
	SymbolResponseSHA256		string	`json:"symbol_response_sha256,omitempty"`
	NameResponseSHA256		string	`json:"name_response_sha256,omitempty"`
	EvidenceStatus			string	`json:"evidence_status"`
	LiveAvailability		string	`json:"live_availability"`
}

// ProbeEVMERC20Asset observes an ERC-20-like method surface only after a base EVM
// probe has verified chain identity and contract bytecode. totalSupply() and
// balanceOf(address) must both return canonical uint256 values before the
// surface is reported. Selector compatibility is evidence, not proof that the
// contract is a standards-compliant or safe ERC-20 token.
func ProbeEVMERC20Asset(ctx context.Context, client *http.Client, endpoint string, base EVMProbeResult) (EVMAssetProbeResult, error) {
	resolution := base.Resolution
	expectedChainID, ok := ExpectedEVMChainID(resolution.Network.ID)
	if !ok || resolution.Network.Family != "evm" || !resolution.SyntaxValid ||
		!base.AnalysisPerformed || base.EvidenceStatus != "observed" || base.LiveAvailability != "checked" ||
		strings.ToLower(strings.TrimSpace(base.ChainID)) != expectedChainID ||
		strings.ToLower(strings.TrimSpace(base.ExpectedChainID)) != expectedChainID ||
		base.ContractCodeState != "contract_code_observed" {
		return EVMAssetProbeResult{}, fmt.Errorf("evm_asset_base_probe_invalid")
	}
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return EVMAssetProbeResult{}, fmt.Errorf("evm_rpc_endpoint_invalid")
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	contract, err := normalizeEVMCallAddress(resolution.Address)
	if err != nil {
		return EVMAssetProbeResult{}, fmt.Errorf("evm_asset_contract_invalid")
	}

	totalRaw, totalDigest, observed, err := evmAssetCall(ctx, client, endpoint, 41, contract, "0x"+evmERC20TotalSupplySelector)
	if err != nil {
		return EVMAssetProbeResult{}, fmt.Errorf("evm_asset_total_supply_unavailable: %w", err)
	}
	if !observed {
		return EVMAssetProbeResult{}, ErrEVMERC20SurfaceNotObserved
	}
	totalSupply, err := parseEVMUint256(totalRaw)
	if err != nil {
		return EVMAssetProbeResult{}, fmt.Errorf("evm_asset_total_supply_invalid")
	}

	balanceData := "0x" + evmERC20BalanceOfSelector + abiAddressWord(contract)
	balanceRaw, balanceDigest, observed, err := evmAssetCall(ctx, client, endpoint, 42, contract, balanceData)
	if err != nil {
		return EVMAssetProbeResult{}, fmt.Errorf("evm_asset_balance_unavailable: %w", err)
	}
	if !observed {
		return EVMAssetProbeResult{}, ErrEVMERC20SurfaceNotObserved
	}
	contractBalance, err := parseEVMUint256(balanceRaw)
	if err != nil {
		return EVMAssetProbeResult{}, fmt.Errorf("evm_asset_balance_invalid")
	}

	result := EVMAssetProbeResult{
		SchemaVersion:			SchemaVersion,
		Network:			resolution.Network.ID,
		ChainID:			expectedChainID,
		ExpectedChainID:		expectedChainID,
		Contract:			contract,
		InterfaceState:		"erc20_like_surface_observed",
		TotalSupply:			totalSupply.String(),
		ContractBalance:		contractBalance.String(),
		TotalSupplyResponseSHA256:		totalDigest,
		ContractBalanceResponseSHA256:	balanceDigest,
		EvidenceStatus:			"observed",
		LiveAvailability:		"checked",
	}

	if value, digest, ok, callErr := evmAssetCall(ctx, client, endpoint, 43, contract, "0x"+evmERC20DecimalsSelector); callErr == nil {
		result.DecimalsResponseSHA256 = digest
		if ok {
			decimals, parseErr := parseEVMUint256(value)
			if parseErr == nil && decimals.IsUint64() && decimals.Uint64() <= 255 {
				result.Decimals = decimals.Uint64()
				result.DecimalsObserved = true
			}
		}
	}
	if value, digest, ok, callErr := evmAssetCall(ctx, client, endpoint, 44, contract, "0x"+evmERC20SymbolSelector); callErr == nil {
		result.SymbolResponseSHA256 = digest
		if ok {
			if symbol, encoding, decodeErr := decodeEVMMetadataString(value); decodeErr == nil {
				result.Symbol = symbol
				result.SymbolEncoding = encoding
				result.SymbolObserved = true
			}
		}
	}
	if value, digest, ok, callErr := evmAssetCall(ctx, client, endpoint, 45, contract, "0x"+evmERC20NameSelector); callErr == nil {
		result.NameResponseSHA256 = digest
		if ok {
			if name, encoding, decodeErr := decodeEVMMetadataString(value); decodeErr == nil {
				result.Name = name
				result.NameEncoding = encoding
				result.NameObserved = true
			}
		}
	}
	return result, nil
}

func evmAssetCall(ctx context.Context, client *http.Client, endpoint string, id int, contract, data string) (string, string, bool, error) {
	payload, err := json.Marshal(evmRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "eth_call",
		Params:  []any{map[string]any{"to": contract, "data": data}, "latest"},
	})
	if err != nil {
		return "", "", false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", "", false, fmt.Errorf("rpc_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: evmProbeResponseLimit + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", "", false, err
	}
	if limited.N <= 0 {
		return "", "", false, fmt.Errorf("rpc_response_too_large")
	}
	digest := sha256.Sum256(body)
	responseSHA256 := hex.EncodeToString(digest[:])
	var decoded evmRPCResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&decoded); err != nil {
		return "", responseSHA256, false, err
	}
	if decoded.JSONRPC != "2.0" || decoded.ID != id {
		return "", responseSHA256, false, fmt.Errorf("rpc_response_invalid")
	}
	if decoded.Error != nil || len(decoded.Result) == 0 || bytes.Equal(bytes.TrimSpace(decoded.Result), []byte("null")) {
		return "", responseSHA256, false, nil
	}
	var value string
	if err := json.Unmarshal(decoded.Result, &value); err != nil || strings.TrimSpace(value) == "" {
		return "", responseSHA256, false, fmt.Errorf("rpc_result_invalid")
	}
	return value, responseSHA256, true, nil
}

func decodeEVMMetadataString(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "0x") {
		return "", "", fmt.Errorf("metadata_hex_required")
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil || len(raw) == 0 || len(raw) > 4096 {
		return "", "", fmt.Errorf("metadata_hex_invalid")
	}
	if len(raw) == 32 {
		text := strings.TrimSpace(string(bytes.TrimRight(raw, "\x00")))
		if text == "" || !utf8.ValidString(text) {
			return "", "", fmt.Errorf("metadata_bytes32_invalid")
		}
		return text, "bytes32_legacy", nil
	}
	if len(raw) < 64 {
		return "", "", fmt.Errorf("metadata_abi_invalid")
	}
	offsetInt := new(big.Int).SetBytes(raw[:32])
	if !offsetInt.IsInt64() {
		return "", "", fmt.Errorf("metadata_offset_invalid")
	}
	offset := int(offsetInt.Int64())
	if offset < 0 || offset%32 != 0 || offset+32 > len(raw) {
		return "", "", fmt.Errorf("metadata_offset_invalid")
	}
	lengthInt := new(big.Int).SetBytes(raw[offset : offset+32])
	if !lengthInt.IsInt64() {
		return "", "", fmt.Errorf("metadata_length_invalid")
	}
	length := int(lengthInt.Int64())
	if length <= 0 || length > 256 || offset+32+length > len(raw) {
		return "", "", fmt.Errorf("metadata_length_invalid")
	}
	data := raw[offset+32 : offset+32+length]
	if !utf8.Valid(data) {
		return "", "", fmt.Errorf("metadata_utf8_invalid")
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", "", fmt.Errorf("metadata_empty")
	}
	return text, "abi_string", nil
}
