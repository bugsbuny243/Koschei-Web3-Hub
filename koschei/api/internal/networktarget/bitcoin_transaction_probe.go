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
	bitcoinTransactionResponseLimit = 4 * 1024 * 1024
	bitcoinMaxSatoshis               = int64(21_000_000 * 100_000_000)
)

var ErrBitcoinTransactionNotFound = errors.New("bitcoin_transaction_not_found")

type BitcoinTransactionEvidenceResult struct {
	SchemaVersion         string                          `json:"schema_version"`
	Network               string                          `json:"network"`
	GenesisHash           string                          `json:"genesis_hash"`
	ExpectedGenesisHash   string                          `json:"expected_genesis_hash"`
	GenesisResponseSHA256 string                          `json:"genesis_response_sha256,omitempty"`
	TransactionID         string                          `json:"transaction_id"`
	Version               int64                           `json:"version"`
	Locktime              int64                           `json:"locktime"`
	InputCount            int                             `json:"input_count"`
	OutputCount           int                             `json:"output_count"`
	Size                  int64                           `json:"size"`
	Weight                int64                           `json:"weight"`
	FeeSats               int64                           `json:"fee_sats"`
	Coinbase              bool                            `json:"coinbase"`
	TotalInputSats        int64                           `json:"total_input_sats"`
	TotalOutputSats       int64                           `json:"total_output_sats"`
	DerivedFeeSats        int64                           `json:"derived_fee_sats"`
	FeeBalanceChecked     bool                            `json:"fee_balance_checked"`
	FeeConsistent         bool                            `json:"fee_consistent"`
	Inputs                []BitcoinTransactionInputFlow   `json:"inputs"`
	Outputs               []BitcoinTransactionOutputFlow  `json:"outputs"`
	Confirmed             bool                            `json:"confirmed"`
	BlockHeight           int64                           `json:"block_height,omitempty"`
	BlockHash             string                          `json:"block_hash,omitempty"`
	BlockTimeUnix         int64                           `json:"block_time_unix,omitempty"`
	TransactionSHA256     string                          `json:"transaction_response_sha256"`
	AnalysisPerformed     bool                            `json:"analysis_performed"`
	EvidenceStatus        string                          `json:"evidence_status"`
	LiveAvailability      string                          `json:"live_availability"`
}

type BitcoinTransactionInputFlow struct {
	Index                       int    `json:"index"`
	PreviousTxID                string `json:"previous_txid,omitempty"`
	PreviousVout                int64  `json:"previous_vout"`
	Coinbase                    bool   `json:"coinbase"`
	Sequence                    uint64 `json:"sequence"`
	PreviousValueSats           int64  `json:"previous_value_sats"`
	PreviousScriptType          string `json:"previous_script_type,omitempty"`
	PreviousAddress             string `json:"previous_address,omitempty"`
	PreviousScriptPubKeySHA256  string `json:"previous_scriptpubkey_sha256,omitempty"`
}

type BitcoinTransactionOutputFlow struct {
	Index              int    `json:"index"`
	ValueSats          int64  `json:"value_sats"`
	ScriptType         string `json:"script_type"`
	Address            string `json:"address,omitempty"`
	ScriptPubKeySHA256 string `json:"scriptpubkey_sha256"`
}

type bitcoinTransactionOutputResponse struct {
	ScriptPubKey        string `json:"scriptpubkey"`
	ScriptPubKeyType    string `json:"scriptpubkey_type"`
	ScriptPubKeyAddress string `json:"scriptpubkey_address"`
	Value               int64  `json:"value"`
}

type bitcoinTransactionInputResponse struct {
	TxID       string                            `json:"txid"`
	Vout       int64                             `json:"vout"`
	IsCoinbase bool                              `json:"is_coinbase"`
	Sequence   uint64                            `json:"sequence"`
	Prevout    *bitcoinTransactionOutputResponse `json:"prevout"`
}

type bitcoinTransactionResponse struct {
	TxID     string                            `json:"txid"`
	Version  int64                             `json:"version"`
	Locktime int64                             `json:"locktime"`
	Vin      []bitcoinTransactionInputResponse `json:"vin"`
	Vout     []bitcoinTransactionOutputResponse `json:"vout"`
	Size     int64                             `json:"size"`
	Weight   int64                             `json:"weight"`
	Fee      int64                             `json:"fee"`
	Status   struct {
		Confirmed   bool   `json:"confirmed"`
		BlockHeight int64  `json:"block_height"`
		BlockHash   string `json:"block_hash"`
		BlockTime   int64  `json:"block_time"`
	} `json:"status"`
}

func summarizeBitcoinTransactionFlow(tx bitcoinTransactionResponse) ([]BitcoinTransactionInputFlow, []BitcoinTransactionOutputFlow, int64, int64, bool, error) {
	inputs := make([]BitcoinTransactionInputFlow, 0, len(tx.Vin))
	outputs := make([]BitcoinTransactionOutputFlow, 0, len(tx.Vout))
	var totalInputSats int64
	var totalOutputSats int64
	coinbase := false

	for index, input := range tx.Vin {
		entry := BitcoinTransactionInputFlow{
			Index:        index,
			PreviousVout: input.Vout,
			Coinbase:     input.IsCoinbase,
			Sequence:     input.Sequence,
		}
		if input.IsCoinbase {
			if coinbase || len(tx.Vin) != 1 {
				return nil, nil, 0, 0, false, fmt.Errorf("bitcoin_transaction_coinbase_layout_invalid")
			}
			coinbase = true
			inputs = append(inputs, entry)
			continue
		}
		previousTxID := strings.ToLower(strings.TrimSpace(input.TxID))
		if !validBitcoinTransactionID(previousTxID) || input.Vout < 0 || input.Prevout == nil {
			return nil, nil, 0, 0, false, fmt.Errorf("bitcoin_transaction_prevout_invalid")
		}
		previousOutput, err := summarizeBitcoinOutput(index, *input.Prevout)
		if err != nil {
			return nil, nil, 0, 0, false, fmt.Errorf("bitcoin_transaction_prevout_invalid: %w", err)
		}
		if totalInputSats > bitcoinMaxSatoshis-previousOutput.ValueSats {
			return nil, nil, 0, 0, false, fmt.Errorf("bitcoin_transaction_input_value_overflow")
		}
		totalInputSats += previousOutput.ValueSats
		entry.PreviousTxID = previousTxID
		entry.PreviousValueSats = previousOutput.ValueSats
		entry.PreviousScriptType = previousOutput.ScriptType
		entry.PreviousAddress = previousOutput.Address
		entry.PreviousScriptPubKeySHA256 = previousOutput.ScriptPubKeySHA256
		inputs = append(inputs, entry)
	}

	for index, output := range tx.Vout {
		entry, err := summarizeBitcoinOutput(index, output)
		if err != nil {
			return nil, nil, 0, 0, false, fmt.Errorf("bitcoin_transaction_output_invalid: %w", err)
		}
		if totalOutputSats > bitcoinMaxSatoshis-entry.ValueSats {
			return nil, nil, 0, 0, false, fmt.Errorf("bitcoin_transaction_output_value_overflow")
		}
		totalOutputSats += entry.ValueSats
		outputs = append(outputs, entry)
	}
	return inputs, outputs, totalInputSats, totalOutputSats, coinbase, nil
}

func summarizeBitcoinOutput(index int, output bitcoinTransactionOutputResponse) (BitcoinTransactionOutputFlow, error) {
	if output.Value < 0 || output.Value > bitcoinMaxSatoshis {
		return BitcoinTransactionOutputFlow{}, fmt.Errorf("bitcoin_output_value_invalid")
	}
	script := strings.ToLower(strings.TrimSpace(output.ScriptPubKey))
	if script != "" {
		decoded, err := hex.DecodeString(script)
		if err != nil {
			return BitcoinTransactionOutputFlow{}, fmt.Errorf("bitcoin_output_script_invalid")
		}
		digest := sha256.Sum256(decoded)
		script = hex.EncodeToString(digest[:])
	} else {
		digest := sha256.Sum256(nil)
		script = hex.EncodeToString(digest[:])
	}
	scriptType := strings.ToLower(strings.TrimSpace(output.ScriptPubKeyType))
	if scriptType == "" {
		scriptType = "unknown"
	}
	address := strings.TrimSpace(output.ScriptPubKeyAddress)
	if address != "" {
		canonical, _, ok := bitcoinMainnetAddress(address)
		if !ok {
			return BitcoinTransactionOutputFlow{}, fmt.Errorf("bitcoin_output_address_invalid")
		}
		address = canonical
	}
	return BitcoinTransactionOutputFlow{
		Index:              index,
		ValueSats:          output.Value,
		ScriptType:         scriptType,
		Address:            address,
		ScriptPubKeySHA256: script,
	}, nil
}

func validBitcoinTransactionID(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
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
	inputs, outputs, totalInputSats, totalOutputSats, coinbase, flowErr := summarizeBitcoinTransactionFlow(tx)
	if flowErr != nil {
		return BitcoinTransactionEvidenceResult{}, flowErr
	}
	feeBalanceChecked := !coinbase
	derivedFeeSats := int64(0)
	feeConsistent := false
	if feeBalanceChecked {
		if totalInputSats < totalOutputSats {
			return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_transaction_value_balance_invalid")
		}
		derivedFeeSats = totalInputSats - totalOutputSats
		feeConsistent = derivedFeeSats == tx.Fee
		if !feeConsistent {
			return BitcoinTransactionEvidenceResult{}, fmt.Errorf("bitcoin_transaction_fee_mismatch")
		}
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
		SchemaVersion:         SchemaVersion,
		Network:               "bitcoin-mainnet",
		GenesisHash:           genesisHash,
		ExpectedGenesisHash:   bitcoinMainnetGenesisHash,
		GenesisResponseSHA256: genesisDigest,
		TransactionID:         txid,
		Version:               tx.Version,
		Locktime:              tx.Locktime,
		InputCount:            len(tx.Vin),
		OutputCount:           len(tx.Vout),
		Size:                  tx.Size,
		Weight:                tx.Weight,
		FeeSats:               tx.Fee,
		Coinbase:              coinbase,
		TotalInputSats:        totalInputSats,
		TotalOutputSats:       totalOutputSats,
		DerivedFeeSats:        derivedFeeSats,
		FeeBalanceChecked:     feeBalanceChecked,
		FeeConsistent:         feeConsistent,
		Inputs:                inputs,
		Outputs:               outputs,
		Confirmed:             tx.Status.Confirmed,
		BlockHeight:           tx.Status.BlockHeight,
		BlockHash:             blockHash,
		BlockTimeUnix:         tx.Status.BlockTime,
		TransactionSHA256:     hex.EncodeToString(sum[:]),
		AnalysisPerformed:     true,
		EvidenceStatus:        "observed",
		LiveAvailability:      "checked",
	}, nil
}
