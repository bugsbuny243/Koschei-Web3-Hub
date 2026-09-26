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

const bitcoinTransactionResponseLimit = 4 * 1024 * 1024

var ErrBitcoinTransactionNotFound = errors.New("bitcoin_transaction_not_found")

type BitcoinTransactionEvidenceResult struct {
	SchemaVersion         string `json:"schema_version"`
	Network               string `json:"network"`
	GenesisHash           string `json:"genesis_hash"`
	ExpectedGenesisHash   string `json:"expected_genesis_hash"`
	GenesisResponseSHA256 string `json:"genesis_response_sha256,omitempty"`
	TransactionID         string `json:"transaction_id"`
	Version               int64  `json:"version"`
	Locktime              int64  `json:"locktime"`
	InputCount            int    `json:"input_count"`
	OutputCount           int    `json:"output_count"`
	Size                  int64  `json:"size"`
	Weight                int64  `json:"weight"`
	FeeSats               int64  `json:"fee_sats"`
	Confirmed             bool   `json:"confirmed"`
	BlockHeight           int64  `json:"block_height,omitempty"`
	BlockHash             string `json:"block_hash,omitempty"`
	BlockTimeUnix         int64  `json:"block_time_unix,omitempty"`
	TransactionSHA256     string `json:"transaction_response_sha256"`
	AnalysisPerformed     bool   `json:"analysis_performed"`
	EvidenceStatus        string `json:"evidence_status"`
	LiveAvailability      string `json:"live_availability"`
}

type bitcoinTransactionResponse struct {
	TxID     string            `json:"txid"`
	Version  int64             `json:"version"`
	Locktime int64             `json:"locktime"`
	Vin      []json.RawMessage `json:"vin"`
	Vout     []json.RawMessage `json:"vout"`
	Size     int64             `json:"size"`
	Weight   int64             `json:"weight"`
	Fee      int64             `json:"fee"`
	Status   struct {
		Confirmed   bool   `json:"confirmed"`
		BlockHeight int64  `json:"block_height"`
		BlockHash   string `json:"block_hash"`
		BlockTime   int64  `json:"block_time"`
	} `json:"status"`
}

func ProbeBitcoinTransaction(ctx context.Context, client *http.Client, endpoint, txid string) (BitcoinTransactionEvidenceResult, error) {
	txid = strings.ToLower(strings.TrimSpace(txid))
	if len(txid) != 64 {
		return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_transaction_id_invalid")
	}
	decoded, err := hex.DecodeString(txid)
	if err != nil || len(decoded) != 32 {
		return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_transaction_id_invalid")
	}
	baseURL, err := validateBitcoinEsploraEndpoint(endpoint)
	if err != nil {
		return BitcoinTransactionEvidenceResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	genesisHash, genesisDigest, err := bitcoinEsploraTextWithDigest(ctx, client, baseURL+"/block-height/0")
	if err != nil {
		return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_genesis_unavailable: %w", err)
	}
	genesisHash = strings.ToLower(strings.TrimSpace(genesisHash))
	if genesisHash != bitcoinMainnetGenesisHash {
		return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_esplora_network_mismatch")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/tx/"+url.PathEscape(txid), nil)
	if err != nil {
		return BitcoinTransactionEvidenceResult{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return BitcoinTransactionEvidenceResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return BitcoinTransactionEvidenceResult{}, ErrBitcoinTransactionNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return BitcoinTransactionEvidenceResult{}, fmt.Errorf("esplora_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: bitcoinTransactionResponseLimit + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return BitcoinTransactionEvidenceResult{}, err
	}
	if limited.N <= 0 {
		return BitcoinTransactionEvidenceResult{}, fmt.Errorf("esplora_response_too_large")
	}
	var tx bitcoinTransactionResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&tx); err != nil {
		return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_transaction_response_invalid")
	}
	if strings.ToLower(strings.TrimSpace(tx.TxID)) != txid ||
		tx.Version < 0 || tx.Locktime < 0 || tx.Size <= 0 || tx.Weight <= 0 || tx.Fee < 0 ||
		len(tx.Vin) == 0 || len(tx.Vout) == 0 {
		return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_transaction_binding_invalid")
	}
	blockHash := strings.ToLower(strings.TrimSpace(tx.Status.BlockHash))
	if tx.Status.Confirmed {
		if tx.Status.BlockHeight <= 0 || tx.Status.BlockTime <= 0 || len(blockHash) != 64 {
			return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_transaction_confirmation_invalid")
		}
		blockBytes, decodeErr := hex.DecodeString(blockHash)
		if decodeErr != nil || len(blockBytes) != 32 {
			return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_transaction_confirmation_invalid")
		}
	} else if tx.Status.BlockHeight != 0 || blockHash != "" || tx.Status.BlockTime != 0 {
		return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_transaction_pending_anchor_invalid")
	}
	sum := sha256.Sum256(body)
	return BitcoinTransactionEvidenceResult{
		SchemaVersion: SchemaVersion,
		Network: "bitcoin-mainnet",
		GenesisHash: genesisHash,
		ExpectedGenesisHash: bitcoinMainnetGenesisHash,
		GenesisResponseSHA256: genesisDigest,
		TransactionID: txid,
		Version: tx.Version,
		Locktime: tx.Locktime,
		InputCount: len(tx.Vin),
		OutputCount: len(tx.Vout),
		Size: tx.Size,
		Weight: tx.Weight,
		FeeSats: tx.Fee,
		Confirmed: tx.Status.Confirmed,
		BlockHeight: tx.Status.BlockHeight,
		BlockHash: blockHash,
		BlockTimeUnix: tx.Status.BlockTime,
		TransactionSHA256: hex.EncodeToString(sum[:]),
		AnalysisPerformed: true,
		EvidenceStatus: "observed",
		LiveAvailability: "checked",
	}, nil
}
