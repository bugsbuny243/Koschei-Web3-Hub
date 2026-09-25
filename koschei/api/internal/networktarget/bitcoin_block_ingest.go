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
	bitcoinBlockIngestResponseLimit = 4 * 1024 * 1024
	MaxBitcoinIngestTransactions    = 20000
)

type BitcoinBlockIngestResult struct {
	NetworkID                    string    `json:"network_id"`
	Height                       uint64    `json:"height"`
	Hash                         string    `json:"hash"`
	PreviousHash                 string    `json:"previous_hash,omitempty"`
	BlockTime                    time.Time `json:"block_time"`
	ObservedAt                   time.Time `json:"observed_at"`
	TransactionIDs               []string  `json:"transaction_ids"`
	BlockchainInfoResponseSHA256 string    `json:"blockchain_info_response_sha256"`
	BlockHashResponseSHA256      string    `json:"block_hash_response_sha256"`
	BlockResponseSHA256          string    `json:"block_response_sha256"`
}

type bitcoinBlockIngestWire struct {
	Hash              string   `json:"hash"`
	Height            int64    `json:"height"`
	PreviousBlockHash string   `json:"previousblockhash"`
	Time              int64    `json:"time"`
	Tx                []string `json:"tx"`
}

func ProbeBitcoinBlockIngest(ctx context.Context, client *http.Client, endpoint string, height uint64, observedAt time.Time) (BitcoinBlockIngestResult, error) {
	if observedAt.IsZero() {
		return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_observed_at_required")
	}
	if _, err := validateBitcoinCoreTelemetryEndpoint(endpoint); err != nil {
		return BitcoinBlockIngestResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}

	var chainInfo struct {
		Chain  string `json:"chain"`
		Blocks int64  `json:"blocks"`
	}
	chainDigest, err := bitcoinCoreRPCParamsWithDigest(ctx, client, endpoint, 901, "getblockchaininfo", []any{}, &chainInfo)
	if err != nil {
		return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_chain_unavailable: %w", err)
	}
	if chainInfo.Chain != "main" || chainInfo.Blocks < 0 || uint64(chainInfo.Blocks) < height {
		return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_chain_mismatch")
	}

	var blockHash string
	blockHashDigest, err := bitcoinCoreRPCParamsWithDigest(ctx, client, endpoint, 902, "getblockhash", []any{height}, &blockHash)
	if err != nil {
		return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_hash_unavailable: %w", err)
	}
	blockHash = strings.ToLower(strings.TrimSpace(blockHash))
	if !validBitcoinIngestHash(blockHash) {
		return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_hash_invalid")
	}

	var block bitcoinBlockIngestWire
	blockDigest, err := bitcoinCoreRPCParamsWithDigest(ctx, client, endpoint, 903, "getblock", []any{blockHash, 1}, &block)
	if err != nil {
		return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_block_unavailable: %w", err)
	}
	if block.Height < 0 || uint64(block.Height) != height || strings.ToLower(strings.TrimSpace(block.Hash)) != blockHash || block.Time <= 0 {
		return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_binding_invalid")
	}
	previousHash := strings.ToLower(strings.TrimSpace(block.PreviousBlockHash))
	if height > 0 && !validBitcoinIngestHash(previousHash) {
		return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_previous_hash_invalid")
	}
	if len(block.Tx) > MaxBitcoinIngestTransactions {
		return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_transaction_limit_exceeded")
	}
	seen := make(map[string]struct{}, len(block.Tx))
	txIDs := make([]string, 0, len(block.Tx))
	for _, txid := range block.Tx {
		txid = strings.ToLower(strings.TrimSpace(txid))
		if !validBitcoinIngestHash(txid) {
			return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_transaction_id_invalid")
		}
		if _, exists := seen[txid]; exists {
			return BitcoinBlockIngestResult{}, fmt.Errorf("bitcoin_block_ingest_duplicate_transaction")
		}
		seen[txid] = struct{}{}
		txIDs = append(txIDs, txid)
	}

	return BitcoinBlockIngestResult{
		NetworkID: "bitcoin-mainnet", Height: height, Hash: blockHash, PreviousHash: previousHash,
		BlockTime: time.Unix(block.Time, 0).UTC(), ObservedAt: observedAt.UTC(), TransactionIDs: txIDs,
		BlockchainInfoResponseSHA256: chainDigest, BlockHashResponseSHA256: blockHashDigest, BlockResponseSHA256: blockDigest,
	}, nil
}

func bitcoinCoreRPCParamsWithDigest(ctx context.Context, client *http.Client, endpoint string, id int, method string, params []any, target any) (string, error) {
	payload, err := json.Marshal(bitcoinCoreRPCRequest{JSONRPC: "1.0", ID: id, Method: method, Params: params})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("rpc_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: bitcoinBlockIngestResponseLimit + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if limited.N <= 0 {
		return "", fmt.Errorf("rpc_response_too_large")
	}
	sum := sha256.Sum256(body)

	var decoded bitcoinCoreRPCResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&decoded); err != nil {
		return "", err
	}
	if decoded.ID != id || decoded.Error != nil || len(decoded.Result) == 0 {
		return "", fmt.Errorf("rpc_response_invalid")
	}
	if err := json.Unmarshal(decoded.Result, target); err != nil {
		return "", fmt.Errorf("rpc_result_invalid")
	}
	return hex.EncodeToString(sum[:]), nil
}

func validBitcoinIngestHash(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
