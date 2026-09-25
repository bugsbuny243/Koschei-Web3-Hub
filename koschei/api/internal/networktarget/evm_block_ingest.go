package networktarget

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	evmIngestResponseLimit   = 8 * 1024 * 1024
	MaxEVMIngestTransactions = 5000
	MaxEVMIngestLogs         = 10000
	MaxEVMIngestTopicsPerLog = 16
)

type EVMLogObservation struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	DataSHA256  string   `json:"data_sha256"`
	TxHash      string   `json:"transaction_hash"`
	BlockHash   string   `json:"block_hash"`
	BlockNumber uint64   `json:"block_number"`
	LogIndex    uint64   `json:"log_index"`
	Removed     bool     `json:"removed"`
}

type EVMBlockIngestResult struct {
	NetworkID             string              `json:"network_id"`
	ChainID               string              `json:"chain_id"`
	Height                uint64              `json:"height"`
	Hash                  string              `json:"hash"`
	ParentHash            string              `json:"parent_hash"`
	BlockTimestamp        time.Time           `json:"block_timestamp"`
	ObservedAt            time.Time           `json:"observed_at"`
	TransactionHashes     []string            `json:"transaction_hashes"`
	Logs                  []EVMLogObservation `json:"logs"`
	ChainIDResponseSHA256 string              `json:"chain_id_response_sha256"`
	BlockResponseSHA256   string              `json:"block_response_sha256"`
	LogsResponseSHA256    string              `json:"logs_response_sha256"`
}

type evmBlockIngestWire struct {
	Number       string   `json:"number"`
	Hash         string   `json:"hash"`
	ParentHash   string   `json:"parentHash"`
	Timestamp    string   `json:"timestamp"`
	Transactions []string `json:"transactions"`
}

type evmLogIngestWire struct {
	Address         string   `json:"address"`
	Topics          []string `json:"topics"`
	Data            string   `json:"data"`
	TransactionHash string   `json:"transactionHash"`
	BlockHash       string   `json:"blockHash"`
	BlockNumber     string   `json:"blockNumber"`
	LogIndex        string   `json:"logIndex"`
	Removed         bool     `json:"removed"`
}

func ProbeEVMBlockIngest(ctx context.Context, client *http.Client, endpoint, networkID string, height uint64, observedAt time.Time) (EVMBlockIngestResult, error) {
	network, ok := LookupNetwork(networkID)
	if !ok || network.Family != "evm" {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_unsupported_network")
	}
	if observedAt.IsZero() {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_observed_at_required")
	}
	if _, err := validateEVMNodeTelemetryEndpoint(endpoint); err != nil {
		return EVMBlockIngestResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	expectedChainID, ok := ExpectedEVMChainID(networkID)
	if !ok {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_chain_id_unknown")
	}
	chainID, chainDigest, err := evmRPCStringWithDigest(ctx, client, endpoint, 801, "eth_chainId", nil)
	if err != nil {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_chain_id_unavailable: %w", err)
	}
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	if chainID != expectedChainID {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_rpc_network_mismatch")
	}

	tag := fmt.Sprintf("0x%x", height)
	blockRaw, blockDigest, err := evmIngestRPCRawWithDigest(ctx, client, endpoint, 802, "eth_getBlockByNumber", []any{tag, false})
	if err != nil {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_block_unavailable: %w", err)
	}
	var block evmBlockIngestWire
	if err := json.Unmarshal(blockRaw, &block); err != nil {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_block_invalid")
	}
	number, err := parseEVMQuantity(strings.TrimSpace(block.Number))
	if err != nil || number != height {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_height_mismatch")
	}
	hash := strings.ToLower(strings.TrimSpace(block.Hash))
	parentHash := strings.ToLower(strings.TrimSpace(block.ParentHash))
	if !validEVMTransactionHash(hash) || !validEVMTransactionHash(parentHash) {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_hash_invalid")
	}
	timestampSeconds, err := parseEVMQuantity(strings.TrimSpace(block.Timestamp))
	if err != nil {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_timestamp_invalid")
	}
	if len(block.Transactions) > MaxEVMIngestTransactions {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_transaction_limit_exceeded")
	}
	txSeen := make(map[string]struct{}, len(block.Transactions))
	txHashes := make([]string, 0, len(block.Transactions))
	for _, txHash := range block.Transactions {
		txHash = strings.ToLower(strings.TrimSpace(txHash))
		if !validEVMTransactionHash(txHash) {
			return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_transaction_hash_invalid")
		}
		if _, exists := txSeen[txHash]; exists {
			return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_duplicate_transaction")
		}
		txSeen[txHash] = struct{}{}
		txHashes = append(txHashes, txHash)
	}

	logsRaw, logsDigest, err := evmIngestRPCRawWithDigest(ctx, client, endpoint, 803, "eth_getLogs", []any{map[string]any{"blockHash": hash}})
	if err != nil {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_logs_unavailable: %w", err)
	}
	var logRows []evmLogIngestWire
	if err := json.Unmarshal(logsRaw, &logRows); err != nil {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_logs_invalid")
	}
	if len(logRows) > MaxEVMIngestLogs {
		return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_log_limit_exceeded")
	}
	logSeen := make(map[string]struct{}, len(logRows))
	logs := make([]EVMLogObservation, 0, len(logRows))
	for _, row := range logRows {
		txHash := strings.ToLower(strings.TrimSpace(row.TransactionHash))
		logBlockHash := strings.ToLower(strings.TrimSpace(row.BlockHash))
		if !validEVMTransactionHash(txHash) || logBlockHash != hash {
			return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_log_binding_invalid")
		}
		blockNumber, err := parseEVMQuantity(strings.TrimSpace(row.BlockNumber))
		if err != nil || blockNumber != height {
			return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_log_height_invalid")
		}
		logIndex, err := parseEVMQuantity(strings.TrimSpace(row.LogIndex))
		if err != nil {
			return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_log_index_invalid")
		}
		address := strings.ToLower(strings.TrimSpace(row.Address))
		if !validEVMIngestAddress(address) {
			return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_log_address_invalid")
		}
		if len(row.Topics) > MaxEVMIngestTopicsPerLog {
			return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_log_topic_limit_exceeded")
		}
		topics := make([]string, 0, len(row.Topics))
		for _, topic := range row.Topics {
			topic = strings.ToLower(strings.TrimSpace(topic))
			if !validEVMTransactionHash(topic) {
				return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_log_topic_invalid")
			}
			topics = append(topics, topic)
		}
		dataSHA256, err := evmHexPayloadSHA256(row.Data)
		if err != nil {
			return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_log_data_invalid")
		}
		key := txHash + "/" + fmt.Sprintf("%d", logIndex)
		if _, exists := logSeen[key]; exists {
			return EVMBlockIngestResult{}, fmt.Errorf("evm_block_ingest_duplicate_log")
		}
		logSeen[key] = struct{}{}
		logs = append(logs, EVMLogObservation{
			Address: address, Topics: topics, DataSHA256: dataSHA256, TxHash: txHash,
			BlockHash: hash, BlockNumber: height, LogIndex: logIndex, Removed: row.Removed,
		})
	}

	return EVMBlockIngestResult{
		NetworkID: networkID, ChainID: chainID, Height: height, Hash: hash, ParentHash: parentHash,
		BlockTimestamp: time.Unix(int64(timestampSeconds), 0).UTC(), ObservedAt: observedAt.UTC(),
		TransactionHashes: txHashes, Logs: logs,
		ChainIDResponseSHA256: chainDigest, BlockResponseSHA256: blockDigest, LogsResponseSHA256: logsDigest,
	}, nil
}

func evmIngestRPCRawWithDigest(ctx context.Context, client *http.Client, endpoint string, id int, method string, params []any) (json.RawMessage, string, error) {
	payload, err := json.Marshal(evmRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("rpc_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: evmIngestResponseLimit + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if limited.N <= 0 {
		return nil, "", fmt.Errorf("rpc_response_too_large")
	}
	sum := sha256.Sum256(body)
	var decoded evmRPCResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&decoded); err != nil {
		return nil, "", err
	}
	if decoded.JSONRPC != "2.0" || decoded.ID != id || decoded.Error != nil || len(decoded.Result) == 0 || bytes.Equal(bytes.TrimSpace(decoded.Result), []byte("null")) {
		return nil, "", fmt.Errorf("rpc_response_invalid")
	}
	return decoded.Result, hex.EncodeToString(sum[:]), nil
}

func validEVMIngestAddress(value string) bool {
	if len(value) != 42 || !strings.HasPrefix(value, "0x") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	return err == nil
}

func evmHexPayloadSHA256(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(value, "0x") {
		return "", fmt.Errorf("hex prefix required")
	}
	encoded := strings.TrimPrefix(value, "0x")
	if len(encoded)%2 != 0 {
		return "", fmt.Errorf("hex length invalid")
	}
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
