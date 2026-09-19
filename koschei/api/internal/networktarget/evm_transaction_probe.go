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

	evmTransactionResponseLimit = 8 * 1024 * 1024
)

var ErrEVMTransactionNotFound = errors.New("evm_transaction_not_found")

type EVMTransactionLogSummary struct {
	Address  string   `json:"address"`
	Topics   []string `json:"topics,omitempty"`
	LogIndex string   `json:"log_index,omitempty"`
	Removed  bool     `json:"removed"`
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
	inputHash, inputBytes, err := hashEVMHexData(tx.Input)
	if err != nil {
		return EVMTransactionEvidenceResult{}, fmt.Errorf("evm_transaction_input_invalid")
	}

	result := EVMTransactionEvidenceResult{
		SchemaVersion: SchemaVersion, Network: strings.TrimSpace(networkID),
		ChainID: chainID, ExpectedChainID: expectedChainID, TransactionHash: transactionHash,
		From: strings.ToLower(tx.From), To: to, Value: tx.Value, Nonce: tx.Nonce,
		Gas: tx.Gas, GasPrice: tx.GasPrice, TransactionType: tx.Type,
		InputSHA256: inputHash, InputBytes: inputBytes,
		ExecutionState: EVMTransactionExecutionPending,
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
	receipt.Root = strings.ToLower(strings.TrimSpace(receipt.Root))
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
		result.Logs = append(result.Logs, EVMTransactionLogSummary{
			Address: address, Topics: topics, LogIndex: item.LogIndex, Removed: item.Removed,
		})
	}
	return result, nil
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
