package evm

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Flow reads for the EVM Flow Radar: ERC-20 Transfer logs for a block range
// and the current balance / issuer freeze state of one holder. This file owns
// the JSON-RPC transport and ABI decoding so the radar in services only sees
// normalized FlowTransfer values (adapter boundary, AGENTS.md).
//
// Errors carry a category only; never the endpoint URL (it can embed a
// provider key) or a provider message.

const (
	FreezeFrozen      = "frozen"
	FreezeClear       = "not_frozen"
	FreezeUnknown     = "unknown"
	FreezeUnsupported = "issuer_cannot_freeze"

	flowMaxResponseBytes = 64 << 20
)

// TransferTopic is keccak256("Transfer(address,address,uint256)").
var TransferTopic = "0x" + Keccak([]byte("Transfer(address,address,uint256)"))

// ErrLogRangeTooLarge reports that the provider rejected an eth_getLogs block
// range or result size; callers retry with a smaller range.
var ErrLogRangeTooLarge = errors.New("evm_flow_log_range_too_large")

// FlowToken is a stablecoin the radar follows on one network. Issuer and
// FreezeMethod are empty when no issuer can freeze balances of this token.
type FlowToken struct {
	Address      string `json:"address"`
	Symbol       string `json:"symbol"`
	Decimals     int    `json:"decimals"`
	Issuer       string `json:"issuer,omitempty"`
	IssuerOption string `json:"issuer_option,omitempty"`
	FreezeMethod string `json:"freeze_method,omitempty"`
}

// FlowTokens are the stablecoin contracts followed per network: Tether USDT
// and Circle native USDC as listed by their issuers, and on BNB Chain the
// Binance-Peg tokens, which have no issuer freeze function. Token name,
// symbol and decimals were checked against Etherscan V2 token metadata
// (Ethereum USDT/USDC, BNB Chain BSC-USD/USDC, Arbitrum USDC FiatTokenProxy).
var FlowTokens = map[string][]FlowToken{
	"ethereum-mainnet": {
		{Address: "0xdac17f958d2ee523a2206206994597c13d831ec7", Symbol: "USDT", Decimals: 6, Issuer: "Tether", IssuerOption: "issuer_freeze_request:tether_usdt", FreezeMethod: "isBlackListed(address)"},
		{Address: "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", Symbol: "USDC", Decimals: 6, Issuer: "Circle", IssuerOption: "issuer_freeze_request:circle_usdc", FreezeMethod: "isBlacklisted(address)"},
	},
	"base-mainnet": {
		{Address: "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913", Symbol: "USDC", Decimals: 6, Issuer: "Circle", IssuerOption: "issuer_freeze_request:circle_usdc", FreezeMethod: "isBlacklisted(address)"},
	},
	"arbitrum-mainnet": {
		{Address: "0xaf88d065e77c8cc2239327c5edb3a432268e5831", Symbol: "USDC", Decimals: 6, Issuer: "Circle", IssuerOption: "issuer_freeze_request:circle_usdc", FreezeMethod: "isBlacklisted(address)"},
	},
	"optimism-mainnet": {
		{Address: "0x0b2c639c533813f4aa9d7837caf62653d097ff85", Symbol: "USDC", Decimals: 6, Issuer: "Circle", IssuerOption: "issuer_freeze_request:circle_usdc", FreezeMethod: "isBlacklisted(address)"},
	},
	"polygon-mainnet": {
		{Address: "0x3c499c542cef5e3811e1192ce70d8cc03d5c3359", Symbol: "USDC", Decimals: 6, Issuer: "Circle", IssuerOption: "issuer_freeze_request:circle_usdc", FreezeMethod: "isBlacklisted(address)"},
	},
	"bnb-mainnet": {
		{Address: "0x55d398326f99059ff775485246999027b3197955", Symbol: "BSC-USD", Decimals: 18},
		{Address: "0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d", Symbol: "USDC", Decimals: 18},
	},
}

// FlowTransfer is one decoded ERC-20 Transfer log.
type FlowTransfer struct {
	TransactionHash string
	LogIndex        uint64
	BlockNumber     uint64
	BlockHash       string
	Token           string // 0x-lower contract address
	From            string // 0x-lower
	To              string // 0x-lower
	Value           *big.Int
}

// EventID identifies the log uniquely on its chain.
func (t FlowTransfer) EventID() string {
	return t.TransactionHash + ":" + strconv.FormatUint(t.LogIndex, 10)
}

// FlowClient is a minimal JSON-RPC client for one network endpoint.
type FlowClient struct {
	URL  string
	HTTP *http.Client
}

type flowRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func flowRangeRejected(e *flowRPCError) bool {
	if e.Code == -32005 {
		return true
	}
	message := strings.ToLower(e.Message)
	for _, marker := range []string{"block range", "response size", "too many", "range limit", "query returned more than", "exceed"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (c FlowClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return nil, errors.New("evm_flow_request_encode")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("evm_flow_provider_url_invalid")
	}
	request.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("evm_flow_provider_unreachable")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		return nil, errors.New("evm_flow_provider_rate_limited")
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("evm_flow_provider_http_%d", response.StatusCode)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *flowRPCError   `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, flowMaxResponseBytes)).Decode(&envelope); err != nil {
		return nil, errors.New("evm_flow_provider_decode")
	}
	if envelope.Error != nil {
		if method == "eth_getLogs" && flowRangeRejected(envelope.Error) {
			return nil, ErrLogRangeTooLarge
		}
		return nil, fmt.Errorf("evm_flow_provider_rpc_%d", envelope.Error.Code)
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil, errors.New("evm_flow_provider_empty_result")
	}
	return envelope.Result, nil
}

func parseQuantity(raw string) (uint64, error) {
	if !strings.HasPrefix(raw, "0x") || len(raw) < 3 || len(raw) > 18 {
		return 0, errors.New("invalid quantity")
	}
	return strconv.ParseUint(raw[2:], 16, 64)
}

// ChainID returns the RPC endpoint's chain identity; invalid responses fail closed.
func (c FlowClient) ChainID(ctx context.Context) (uint64, error) {
	raw, err := c.call(ctx, "eth_chainId", []any{})
	if err != nil {
		return 0, err
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return 0, errors.New("evm_flow_provider_chain_id_invalid")
	}
	id, err := parseQuantity(value)
	if err != nil || id == 0 {
		return 0, errors.New("evm_flow_provider_chain_id_invalid")
	}
	return id, nil
}

// BlockNumber returns the provider's latest block number.
func (c FlowClient) BlockNumber(ctx context.Context) (uint64, error) {
	raw, err := c.call(ctx, "eth_blockNumber", []any{})
	if err != nil {
		return 0, err
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return 0, errors.New("evm_flow_provider_decode")
	}
	height, err := parseQuantity(text)
	if err != nil {
		return 0, errors.New("evm_flow_provider_decode")
	}
	return height, nil
}

// TransferLogs returns the Transfer logs of the given token contracts in the
// inclusive block range, ordered by block and log index.
func (c FlowClient) TransferLogs(ctx context.Context, from, to uint64, tokens []string) ([]FlowTransfer, error) {
	if to < from || len(tokens) == 0 {
		return nil, nil
	}
	filter := map[string]any{
		"fromBlock": "0x" + strconv.FormatUint(from, 16), "toBlock": "0x" + strconv.FormatUint(to, 16),
		"address": tokens, "topics": []any{TransferTopic},
	}
	raw, err := c.call(ctx, "eth_getLogs", []any{filter})
	if err != nil {
		return nil, err
	}
	allowed := map[string]struct{}{}
	for _, token := range tokens {
		allowed[strings.ToLower(token)] = struct{}{}
	}
	transfers, err := ParseTransferLogs(raw, allowed)
	if err != nil {
		return nil, errors.New("evm_flow_provider_decode")
	}
	for _, transfer := range transfers {
		if transfer.BlockNumber < from || transfer.BlockNumber > to {
			return nil, errors.New("evm_flow_provider_log_outside_range")
		}
	}
	return transfers, nil
}

type flowLog struct {
	Address         string   `json:"address"`
	Topics          []string `json:"topics"`
	Data            string   `json:"data"`
	BlockNumber     string   `json:"blockNumber"`
	BlockHash       string   `json:"blockHash"`
	TransactionHash string   `json:"transactionHash"`
	LogIndex        string   `json:"logIndex"`
	Removed         bool     `json:"removed"`
}

func flowHash(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return false
	}
	_, err := hex.DecodeString(value[2:])
	return err == nil
}

// ParseTransferLogs decodes ERC-20 Transfer logs. Logs from other contracts,
// removed (reorged) logs and ERC-721 shaped Transfer logs (four topics) are
// skipped; a malformed ERC-20 Transfer log fails the whole batch, so a
// partial or corrupt response never advances the radar cursor.
func ParseTransferLogs(raw json.RawMessage, allowed map[string]struct{}) ([]FlowTransfer, error) {
	var logs []flowLog
	if err := json.Unmarshal(raw, &logs); err != nil {
		return nil, err
	}
	out := make([]FlowTransfer, 0, len(logs))
	for _, entry := range logs {
		token := strings.ToLower(entry.Address)
		if _, ok := allowed[token]; !ok || entry.Removed {
			continue
		}
		if len(entry.Topics) != 3 || !strings.EqualFold(entry.Topics[0], TransferTopic) {
			continue
		}
		from, err := addressWord(entry.Topics[1])
		if err != nil {
			return nil, err
		}
		to, err := addressWord(entry.Topics[2])
		if err != nil {
			return nil, err
		}
		data, err := word(entry.Data)
		if err != nil {
			return nil, err
		}
		block, err := parseQuantity(entry.BlockNumber)
		if err != nil {
			return nil, err
		}
		index, err := parseQuantity(entry.LogIndex)
		if err != nil {
			return nil, err
		}
		if !flowHash(entry.TransactionHash) || !flowHash(entry.BlockHash) {
			return nil, errors.New("invalid log hash")
		}
		out = append(out, FlowTransfer{
			TransactionHash: strings.ToLower(entry.TransactionHash), LogIndex: index, BlockNumber: block,
			BlockHash: strings.ToLower(entry.BlockHash), Token: token, From: from, To: to, Value: new(big.Int).SetBytes(data),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].BlockNumber != out[j].BlockNumber {
			return out[i].BlockNumber < out[j].BlockNumber
		}
		return out[i].LogIndex < out[j].LogIndex
	})
	return out, nil
}

func flowAddressCall(method, holder string) (string, error) {
	canonical, err := Address(holder)
	if err != nil {
		return "", err
	}
	return selector(method) + strings.Repeat("0", 24) + canonical[2:], nil
}

func (c FlowClient) ethCallWord(ctx context.Context, token, data string) ([]byte, error) {
	raw, err := c.call(ctx, "eth_call", []any{map[string]string{"to": token, "data": data}, "latest"})
	if err != nil {
		return nil, err
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return nil, errors.New("evm_flow_provider_decode")
	}
	value, err := word(text)
	if err != nil {
		return nil, errors.New("evm_flow_provider_decode")
	}
	return value, nil
}

// TokenBalance returns balanceOf(holder) at the latest block.
func (c FlowClient) TokenBalance(ctx context.Context, token, holder string) (*big.Int, error) {
	data, err := flowAddressCall("balanceOf(address)", holder)
	if err != nil {
		return nil, err
	}
	value, err := c.ethCallWord(ctx, token, data)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(value), nil
}

// ParseFreezeWord decodes a boolean freeze-state return word.
func ParseFreezeWord(value []byte) string {
	if len(value) != 32 {
		return FreezeUnknown
	}
	for _, b := range value[:31] {
		if b != 0 {
			return FreezeUnknown
		}
	}
	switch value[31] {
	case 0:
		return FreezeClear
	case 1:
		return FreezeFrozen
	default:
		return FreezeUnknown
	}
}

// FreezeState reads the issuer blacklist for holder. Tokens without an issuer
// freeze function report FreezeUnsupported; unreadable state is FreezeUnknown,
// never "not frozen".
func (c FlowClient) FreezeState(ctx context.Context, token FlowToken, holder string) string {
	if token.FreezeMethod == "" {
		return FreezeUnsupported
	}
	data, err := flowAddressCall(token.FreezeMethod, holder)
	if err != nil {
		return FreezeUnknown
	}
	value, err := c.ethCallWord(ctx, token.Address, data)
	if err != nil {
		return FreezeUnknown
	}
	return ParseFreezeWord(value)
}

// FormatUnits renders an integer token amount with the given decimals.
func FormatUnits(value *big.Int, decimals int) string {
	if value == nil {
		return ""
	}
	if decimals <= 0 {
		return value.String()
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	whole, fraction := new(big.Int).QuoRem(value, scale, new(big.Int))
	frac := strings.TrimRight(fmt.Sprintf("%0*s", decimals, fraction.String()), "0")
	if frac == "" {
		return whole.String()
	}
	return whole.String() + "." + frac
}

// NormalizeUSD6 scales a stablecoin amount to 6 decimals so thresholds apply
// equally to 6- and 18-decimal tokens. Sub-unit remainders are truncated.
func NormalizeUSD6(value *big.Int, decimals int) *big.Int {
	if value == nil {
		return new(big.Int)
	}
	switch {
	case decimals == 6:
		return new(big.Int).Set(value)
	case decimals > 6:
		return new(big.Int).Quo(value, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals-6)), nil))
	default:
		return new(big.Int).Mul(value, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(6-decimals)), nil))
	}
}
