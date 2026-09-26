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
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	EVMTransactionExecutionUnknown  = "unknown"
	EVMTransactionExecutionPending  = "pending"
	EVMTransactionExecutionSuccess  = "success"
	EVMTransactionExecutionReverted = "reverted"

	// JSON-RPC hex payloads are roughly twice the raw calldata/log-data size.
	// Keep a hard bound, but allow consensus-valid high-gas EVM transactions
	// on supported networks without rejecting them solely due to JSON expansion.
	evmTransactionResponseLimit = 128 * 1024 * 1024

	evmTransferEventTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	evmApprovalEventTopic = "0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"
)

var ErrEVMTransactionNotFound = errors.New("evm_transaction_not_found")

type EVMTransactionLogSummary struct {
	Address        string   `json:"address"`
	Topics         []string `json:"topics,omitempty"`
	LogIndex       string   `json:"log_index,omitempty"`
	Removed        bool     `json:"removed"`
	DataSHA256     string   `json:"data_sha256,omitempty"`
	DataBytes      int      `json:"data_bytes"`
	SemanticKind   string   `json:"semantic_kind,omitempty"`
	SemanticLayout string   `json:"semantic_layout,omitempty"`
	FromAddress    string   `json:"from_address,omitempty"`
	ToAddress      string   `json:"to_address,omitempty"`
	OwnerAddress   string   `json:"owner_address,omitempty"`
	SpenderAddress string   `json:"spender_address,omitempty"`
	ValueHex       string   `json:"value_hex,omitempty"`
	TokenIDHex     string   `json:"token_id_hex,omitempty"`
}

type EVMTransactionEvidenceResult struct {
	SchemaVersion     string                     `json:"schema_version"`
	Network           string                     `json:"network"`
	ChainID           string                     `json:"chain_id"`
	ExpectedChainID   string                     `json:"expected_chain_id"`
	TransactionHash   string                     `json:"transaction_hash"`
	From              string                     `json:"from"`
	To                string                     `json:"to,omitempty"`
	Value             string                     `json:"value,omitempty"`
	Nonce             string                     `json:"nonce,omitempty"`
	Gas               string                     `json:"gas,omitempty"`
	GasPrice          string                     `json:"gas_price,omitempty"`
	TransactionType   string                     `json:"transaction_type,omitempty"`
	InputSHA256       string                     `json:"input_sha256,omitempty"`
	InputBytes        int                        `json:"input_bytes"`
	InputSelector     string                     `json:"input_selector,omitempty"`
	InputSelectorHint string                     `json:"input_selector_hint,omitempty"`
	StandardEventCount int                       `json:"standard_event_count"`
	TransferEventCount int                       `json:"transfer_event_count"`
	ApprovalEventCount int                       `json:"approval_event_count"`
	BlockHash         string                     `json:"block_hash,omitempty"`
	BlockNumber       string                     `json:"block_number,omitempty"`
	ExecutionState    string                     `json:"execution_state"`
	ReceiptStatus     string                     `json:"receipt_status,omitempty"`
	ReceiptRoot       string                     `json:"receipt_root,omitempty"`
	GasUsed           string                     `json:"gas_used,omitempty"`
	CumulativeGasUsed string                     `json:"cumulative_gas_used,omitempty"`
	EffectiveGasPrice string                     `json:"effective_gas_price,omitempty"`
	ContractAddress   string                     `json:"contract_address,omitempty"`
	Logs              []EVMTransactionLogSummary `json:"logs,omitempty"`
	AnalysisPerformed bool                       `json:"analysis_performed"`
	EvidenceStatus    string                     `json:"evidence_status"`
	LiveAvailability  string                     `json:"live_availability"`
}

type evmTransactionRPC struct {
	Hash        string  `json:"hash"`
	From        string  `json:"from"`
	To          *string `json:"to"`
	Value       string  `json:"value"`
	Nonce       string  `json:"nonce"`
	Gas         string  `json:"gas"`
	GasPrice    string  `json:"gasPrice"`
	Type        string  `json:"type"`
	Input       string  `json:"input"`
	BlockHash   *string `json:"blockHash"`
	BlockNumber *string `json:"blockNumber"`
}

type evmReceiptRPC struct {
	TransactionHash   string  `json:"transactionHash"`
	BlockHash         string  `json:"blockHash"`
	BlockNumber       string  `json:"blockNumber"`
	Status            string  `json:"status"`
	Root              string  `json:"root"`
	GasUsed           string  `json:"gasUsed"`
	CumulativeGasUsed string  `json:"cumulativeGasUsed"`
	EffectiveGasPrice string  `json:"effectiveGasPrice"`
	ContractAddress   *string `json:"contractAddress"`
	Logs              []struct {
		Address  string   `json:"address"`
		Topics   []string `json:"topics"`
		LogIndex string   `json:"logIndex"`
		Removed  bool     `json:"removed"`
		Data     string   `json:"data"`
	} `json:"logs"`
}

// ProbeEVMTransaction performs a read-only transaction + receipt observation.
// It verifies chain identity first. A receipt proves only observed execution
// state; this collector does not infer safety, ownership, intent, or authorization.
func ProbeEVMTransaction(ctx context.Context, client *http.Client, endpoint, networkID, transactionHash string) (EVMTransactionEvidenceResult, error) {
	expectedChainID, ok := ExpectedEVMChainID(networkID)
	if !ok {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_probe_unsupported_network")
	}
	transactionHash = strings.ToLower(strings.TrimSpace(transactionHash))
	if !validEVMTransactionHash(transactionHash) {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_hash_invalid")
	}
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_rpc_endpoint_invalid")
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	chainID, err := evmRPCString(ctx, client, endpoint, 1, "eth_chainId", nil)
	if err != nil {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_chain_id_unavailable: %w", err)
	}
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	if chainID != expectedChainID {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_rpc_network_mismatch")
	}

	rawTx, isNull, err := evmRPCRaw(ctx, client, endpoint, 2, "eth_getTransactionByHash", []any{transactionHash})
	if err != nil {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_unavailable: %w", err)
	}
	if isNull {
		return EVMTransactionEvidenceResult{}, ErrEVMTransactionNotFound
	}
	var tx evmTransactionRPC
	if err := json.Unmarshal(rawTx, &tx); err != nil {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_invalid")
	}
	if strings.ToLower(strings.TrimSpace(tx.Hash)) != transactionHash || !validEVMAddress(tx.From) {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_binding_invalid")
	}
	to := ""
	if tx.To != nil {
		to = strings.ToLower(strings.TrimSpace(*tx.To))
		if to != "" && !validEVMAddress(to) {
			return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_to_invalid")
		}
	}
	tx.Value = strings.ToLower(strings.TrimSpace(tx.Value))
	tx.Nonce = strings.ToLower(strings.TrimSpace(tx.Nonce))
	tx.Gas = strings.ToLower(strings.TrimSpace(tx.Gas))
	tx.GasPrice = strings.ToLower(strings.TrimSpace(tx.GasPrice))
	tx.Type = strings.ToLower(strings.TrimSpace(tx.Type))
	if !validEVMHexQuantity(tx.Value) || !validEVMHexQuantity(tx.Nonce) ||
		!validEVMHexQuantity(tx.Gas) || !validEVMHexQuantity(tx.GasPrice) ||
		!validEVMHexQuantity(tx.Type) {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_quantity_invalid")
	}
	inputHash, inputBytes, err := hashEVMHexData(tx.Input)
	if err != nil {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_input_invalid")
	}
	inputSelector, inputSelectorHint := evmTransactionInputSelector(tx.Input)

	result := EVMTransactionEvidenceResult{
		SchemaVersion: SchemaVersion, Network: strings.TrimSpace(networkID),
		ChainID: chainID, ExpectedChainID: expectedChainID, TransactionHash: transactionHash,
		From: strings.ToLower(tx.From), To: to, Value: tx.Value, Nonce: tx.Nonce,
		Gas: tx.Gas, GasPrice: tx.GasPrice, TransactionType: tx.Type,
		InputSHA256: inputHash, InputBytes: inputBytes, InputSelector: inputSelector, InputSelectorHint: inputSelectorHint,
		ExecutionState:    EVMTransactionExecutionPending,
		AnalysisPerformed: true, EvidenceStatus: "observed", LiveAvailability: "checked",
	}
	if (tx.BlockHash == nil) != (tx.BlockNumber == nil) {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_block_anchor_incomplete")
	}
	if tx.BlockHash != nil && tx.BlockNumber != nil {
		result.BlockHash = strings.ToLower(strings.TrimSpace(*tx.BlockHash))
		result.BlockNumber = strings.ToLower(strings.TrimSpace(*tx.BlockNumber))
		if !validEVMTransactionHash(result.BlockHash) || !validEVMHexQuantity(result.BlockNumber) {
			return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_block_anchor_invalid")
		}
	}

	rawReceipt, receiptNull, err := evmRPCRaw(ctx, client, endpoint, 3, "eth_getTransactionReceipt", []any{transactionHash})
	if err != nil {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_receipt_unavailable: %w", err)
	}
	if receiptNull {
		if result.BlockHash != "" || result.BlockNumber != "" {
			result.ExecutionState = EVMTransactionExecutionUnknown
		}
		return result, nil
	}

	var receipt evmReceiptRPC
	if err := json.Unmarshal(rawReceipt, &receipt); err != nil {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_receipt_invalid")
	}
	if strings.ToLower(strings.TrimSpace(receipt.TransactionHash)) != transactionHash {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_receipt_binding_invalid")
	}
	receipt.BlockHash = strings.ToLower(strings.TrimSpace(receipt.BlockHash))
	receipt.BlockNumber = strings.ToLower(strings.TrimSpace(receipt.BlockNumber))
	if !validEVMTransactionHash(receipt.BlockHash) || !validEVMHexQuantity(receipt.BlockNumber) {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_receipt_block_anchor_invalid")
	}
	if result.BlockHash != "" && receipt.BlockHash != result.BlockHash {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_block_hash_mismatch")
	}
	if result.BlockNumber != "" && receipt.BlockNumber != result.BlockNumber {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_block_number_mismatch")
	}
	receipt.Status = strings.ToLower(strings.TrimSpace(receipt.Status))
	receipt.Root = strings.ToLower(strings.TrimSpace(receipt.Root))
	receipt.GasUsed = strings.ToLower(strings.TrimSpace(receipt.GasUsed))
	receipt.CumulativeGasUsed = strings.ToLower(strings.TrimSpace(receipt.CumulativeGasUsed))
	receipt.EffectiveGasPrice = strings.ToLower(strings.TrimSpace(receipt.EffectiveGasPrice))
	if !validEVMHexQuantity(receipt.GasUsed) || !validEVMHexQuantity(receipt.CumulativeGasUsed) {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_receipt_quantity_invalid")
	}
	if receipt.EffectiveGasPrice != "" && !validEVMHexQuantity(receipt.EffectiveGasPrice) {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_receipt_effective_gas_price_invalid")
	}
	if receipt.Status != "" && receipt.Status != "0x0" && receipt.Status != "0x1" {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_receipt_status_invalid")
	}
	if receipt.Status == "" && !validEVMTransactionHash(receipt.Root) {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_receipt_status_unknown_without_root")
	}

	result.BlockHash = receipt.BlockHash
	result.BlockNumber = receipt.BlockNumber
	result.ReceiptStatus = receipt.Status
	result.ReceiptRoot = receipt.Root
	result.GasUsed = receipt.GasUsed
	result.CumulativeGasUsed = receipt.CumulativeGasUsed
	result.EffectiveGasPrice = receipt.EffectiveGasPrice
	if receipt.ContractAddress != nil {
		result.ContractAddress = strings.ToLower(strings.TrimSpace(*receipt.ContractAddress))
		if result.ContractAddress != "" && !validEVMAddress(result.ContractAddress) {
			return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_contract_address_invalid")
		}
	}
	switch receipt.Status {
	case "0x1":
		result.ExecutionState = EVMTransactionExecutionSuccess
	case "0x0":
		result.ExecutionState = EVMTransactionExecutionReverted
	default:
		result.ExecutionState = EVMTransactionExecutionUnknown
	}
	result.Logs = make([]EVMTransactionLogSummary, 0, len(receipt.Logs))
	for _, item := range receipt.Logs {
		address := strings.ToLower(strings.TrimSpace(item.Address))
		if !validEVMAddress(address) {
			return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_log_address_invalid")
		}
		topics := make([]string, 0, len(item.Topics))
		for _, topic := range item.Topics {
			topic = strings.ToLower(strings.TrimSpace(topic))
			if !validEVMTransactionHash(topic) {
				return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_log_topic_invalid")
			}
			topics = append(topics, topic)
		}
		logIndex := strings.ToLower(strings.TrimSpace(item.LogIndex))
		if !validEVMHexQuantity(logIndex) {
			return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_log_index_invalid")
		}
		dataHash, dataBytes, dataErr := hashEVMHexData(item.Data)
		if dataErr != nil {
			return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_log_data_invalid")
		}
		summary := EVMTransactionLogSummary{
			Address: address, Topics: topics, LogIndex: logIndex, Removed: item.Removed,
			DataSHA256: dataHash, DataBytes: dataBytes,
		}
		classifyEVMStandardLog(&summary, item.Data)
		switch summary.SemanticKind {
		case "standard_transfer":
			result.StandardEventCount++
			result.TransferEventCount++
		case "standard_approval":
			result.StandardEventCount++
			result.ApprovalEventCount++
		}
		result.Logs = append(result.Logs, summary)
	}
	return result, nil
}

func evmTransactionInputSelector(input string) (string, string) {
	input = strings.ToLower(strings.TrimSpace(input))
	if len(input) < 10 || !strings.HasPrefix(input, "0x") {
		return "", ""
	}
	selector := input[:10]
	if _, err := hex.DecodeString(selector[2:]); err != nil {
		return "", ""
	}
	switch selector {
	case "0xa9059cbb":
		return selector, "transfer(address,uint256)"
	case "0x095ea7b3":
		return selector, "approve(address,uint256)"
	case "0x23b872dd":
		return selector, "transferFrom(address,address,uint256)"
	case "0x42842e0e":
		return selector, "safeTransferFrom(address,address,uint256)"
	case "0xb88d4fde":
		return selector, "safeTransferFrom(address,address,uint256,bytes)"
	default:
		return selector, ""
	}
}

func classifyEVMStandardLog(summary *EVMTransactionLogSummary, data string) {
	if summary == nil || len(summary.Topics) == 0 {
		return
	}
	topic0 := strings.ToLower(strings.TrimSpace(summary.Topics[0]))
	switch topic0 {
	case evmTransferEventTopic:
		if len(summary.Topics) == 3 && summary.DataBytes == 32 {
			from, fromOK := evmABITopicAddress(summary.Topics[1])
			to, toOK := evmABITopicAddress(summary.Topics[2])
			value, valueOK := evmABIUint256Quantity(data)
			if fromOK && toOK && valueOK {
				summary.SemanticKind = "standard_transfer"
				summary.SemanticLayout = "erc20_like"
				summary.FromAddress = from
				summary.ToAddress = to
				summary.ValueHex = value
			}
			return
		}
		if len(summary.Topics) == 4 && summary.DataBytes == 0 {
			from, fromOK := evmABITopicAddress(summary.Topics[1])
			to, toOK := evmABITopicAddress(summary.Topics[2])
			tokenID, tokenOK := evmABITopicUint256Quantity(summary.Topics[3])
			if fromOK && toOK && tokenOK {
				summary.SemanticKind = "standard_transfer"
				summary.SemanticLayout = "erc721_like"
				summary.FromAddress = from
				summary.ToAddress = to
				summary.TokenIDHex = tokenID
			}
		}
	case evmApprovalEventTopic:
		if len(summary.Topics) == 3 && summary.DataBytes == 32 {
			owner, ownerOK := evmABITopicAddress(summary.Topics[1])
			spender, spenderOK := evmABITopicAddress(summary.Topics[2])
			value, valueOK := evmABIUint256Quantity(data)
			if ownerOK && spenderOK && valueOK {
				summary.SemanticKind = "standard_approval"
				summary.SemanticLayout = "erc20_like"
				summary.OwnerAddress = owner
				summary.SpenderAddress = spender
				summary.ValueHex = value
			}
			return
		}
		if len(summary.Topics) == 4 && summary.DataBytes == 0 {
			owner, ownerOK := evmABITopicAddress(summary.Topics[1])
			spender, spenderOK := evmABITopicAddress(summary.Topics[2])
			tokenID, tokenOK := evmABITopicUint256Quantity(summary.Topics[3])
			if ownerOK && spenderOK && tokenOK {
				summary.SemanticKind = "standard_approval"
				summary.SemanticLayout = "erc721_like"
				summary.OwnerAddress = owner
				summary.SpenderAddress = spender
				summary.TokenIDHex = tokenID
			}
		}
	}
}

func evmABITopicAddress(topic string) (string, bool) {
	topic = strings.ToLower(strings.TrimSpace(topic))
	if !validEVMTransactionHash(topic) {
		return "", false
	}
	raw := topic[2:]
	if raw[:24] != strings.Repeat("0", 24) {
		return "", false
	}
	address := "0x" + raw[24:]
	return address, validEVMAddress(address)
}

func evmABITopicUint256Quantity(topic string) (string, bool) {
	topic = strings.ToLower(strings.TrimSpace(topic))
	if !validEVMTransactionHash(topic) {
		return "", false
	}
	return evmABIWordQuantity(topic[2:])
}

func evmABIUint256Quantity(data string) (string, bool) {
	data = strings.ToLower(strings.TrimSpace(data))
	if len(data) != 66 || !strings.HasPrefix(data, "0x") {
		return "", false
	}
	return evmABIWordQuantity(data[2:])
}

func evmABIWordQuantity(raw string) (string, bool) {
	if len(raw) != 64 {
		return "", false
	}
	if _, err := hex.DecodeString(raw); err != nil {
		return "", false
	}
	trimmed := strings.TrimLeft(raw, "0")
	if trimmed == "" {
		trimmed = "0"
	}
	return "0x" + trimmed, true
}

func validEVMTransactionHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return false
	}
	_, err := hex.DecodeString(value[2:])
	return err == nil
}

func validEVMAddress(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 42 || !strings.HasPrefix(value, "0x") {
		return false
	}
	_, err := hex.DecodeString(value[2:])
	return err == nil
}

func validEVMHexQuantity(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) < 3 || !strings.HasPrefix(value, "0x") {
		return false
	}
	raw := value[2:]
	if raw == "" || (len(raw) > 1 && raw[0] == '0') {
		return false
	}
	_, err := hex.DecodeString(func() string {
		if len(raw)%2 != 0 {
			return "0" + raw
		}
		return raw
	}())
	return err == nil
}

func hashEVMHexData(value string) (string, int, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "0x") {
		return "", 0, fmt.Errorf("hex_prefix_required")
	}
	raw := strings.TrimPrefix(value, "0x")
	if len(raw)%2 != 0 {
		return "", 0, fmt.Errorf("hex_length_invalid")
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(decoded)
	return hex.EncodeToString(sum[:]), len(decoded), nil
}

func evmRPCRaw(ctx context.Context, client *http.Client, endpoint string, id int, method string, params []any) (json.RawMessage, bool, error) {
	payload, err := json.Marshal(evmRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, false, fmt.Errorf("rpc_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: evmTransactionResponseLimit + 1}
	var decoded evmRPCResponse
	if err := json.NewDecoder(limited).Decode(&decoded); err != nil {
		return nil, false, err
	}
	if limited.N <= 0 {
		return nil, false, fmt.Errorf("rpc_response_too_large")
	}
	if decoded.JSONRPC != "2.0" || decoded.ID != id || decoded.Error != nil || len(decoded.Result) == 0 {
		return nil, false, fmt.Errorf("rpc_response_invalid")
	}
	trimmed := bytes.TrimSpace(decoded.Result)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, true, nil
	}
	return append(json.RawMessage(nil), decoded.Result...), false, nil
}
