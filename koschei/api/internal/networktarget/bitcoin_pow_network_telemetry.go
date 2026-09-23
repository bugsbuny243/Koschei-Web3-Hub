package networktarget

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var bitcoinPoWHex256 = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

type BitcoinPoWNetworkTelemetryResult struct {
	SchemaVersion          string                      `json:"schema_version"`
	Observation            NetworkTelemetryObservation `json:"observation"`
	EstimatedNetworkHashPS float64                     `json:"estimated_network_hash_ps"`
	Difficulty             float64                     `json:"difficulty"`
	BestBlockHash          string                      `json:"best_block_hash"`
	Chainwork              string                      `json:"chainwork"`
	Blocks                 int64                       `json:"blocks"`
	Headers                int64                       `json:"headers"`
	InitialBlockDownload   bool                        `json:"initial_block_download"`
	EstimatorScope         string                      `json:"estimator_scope"`
	AnalysisPerformed      bool                        `json:"analysis_performed"`
	LiveAvailability       string                      `json:"live_availability"`
}

type bitcoinPoWBlockchainInfo struct {
	Chain                string  `json:"chain"`
	Blocks               int64   `json:"blocks"`
	Headers              int64   `json:"headers"`
	BestBlockHash        string  `json:"bestblockhash"`
	Difficulty           float64 `json:"difficulty"`
	Chainwork            string  `json:"chainwork"`
	InitialBlockDownload bool    `json:"initialblockdownload"`
}

// ProbeBitcoinPoWNetworkTelemetry records network-level Proof-of-Work evidence
// computed by one explicitly configured Bitcoin Core mainnet node. The returned
// network hash rate is an estimate, not a count of physical miners and not a
// miner/pool market-share measurement.
func ProbeBitcoinPoWNetworkTelemetry(ctx context.Context, client *http.Client, endpoint string, observedAt time.Time) (BitcoinPoWNetworkTelemetryResult, error) {
	if observedAt.IsZero() {
		return BitcoinPoWNetworkTelemetryResult{}, fmt.Errorf("bitcoin_pow_observed_at_required")
	}
	parsed, err := validateBitcoinCoreTelemetryEndpoint(endpoint)
	if err != nil {
		return BitcoinPoWNetworkTelemetryResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	var blockchain bitcoinPoWBlockchainInfo
	if err := bitcoinCoreRPC(ctx, client, endpoint, 401, "getblockchaininfo", &blockchain); err != nil {
		return BitcoinPoWNetworkTelemetryResult{}, fmt.Errorf("bitcoin_pow_blockchain_info_unavailable: %w", err)
	}
	if err := validateBitcoinPoWBlockchainInfo(blockchain); err != nil {
		return BitcoinPoWNetworkTelemetryResult{}, err
	}

	var hashPS float64
	if err := bitcoinCoreRPC(ctx, client, endpoint, 402, "getnetworkhashps", &hashPS); err != nil {
		return BitcoinPoWNetworkTelemetryResult{}, fmt.Errorf("bitcoin_pow_network_hashps_unavailable: %w", err)
	}
	if math.IsNaN(hashPS) || math.IsInf(hashPS, 0) || hashPS < 0 {
		return BitcoinPoWNetworkTelemetryResult{}, fmt.Errorf("bitcoin_pow_network_hashps_invalid")
	}

	host := strings.ToLower(parsed.Hostname())
	observation, err := NormalizeNetworkTelemetry(NetworkTelemetryInput{
		NetworkID:      "bitcoin-mainnet",
		SubjectKind:    "network",
		SubjectID:      "bitcoin-mainnet",
		Source:         "bitcoin-core-pow-estimate:" + host,
		ObservedAt:     observedAt,
		EvidenceStatus: "observed",
	})
	if err != nil {
		return BitcoinPoWNetworkTelemetryResult{}, err
	}

	return BitcoinPoWNetworkTelemetryResult{
		SchemaVersion:          NetworkTelemetrySchemaVersion,
		Observation:            observation,
		EstimatedNetworkHashPS: hashPS,
		Difficulty:             blockchain.Difficulty,
		BestBlockHash:          strings.ToLower(strings.TrimSpace(blockchain.BestBlockHash)),
		Chainwork:              strings.ToLower(strings.TrimSpace(blockchain.Chainwork)),
		Blocks:                 blockchain.Blocks,
		Headers:                blockchain.Headers,
		InitialBlockDownload:   blockchain.InitialBlockDownload,
		EstimatorScope:         "bitcoin_core_network_estimate_from_single_mainnet_node",
		AnalysisPerformed:      true,
		LiveAvailability:       "checked",
	}, nil
}

func validateBitcoinPoWBlockchainInfo(value bitcoinPoWBlockchainInfo) error {
	if strings.TrimSpace(value.Chain) != "main" {
		return fmt.Errorf("bitcoin_pow_network_mismatch")
	}
	if value.Blocks < 0 || value.Headers < 0 {
		return fmt.Errorf("bitcoin_pow_height_invalid")
	}
	if math.IsNaN(value.Difficulty) || math.IsInf(value.Difficulty, 0) || value.Difficulty < 0 {
		return fmt.Errorf("bitcoin_pow_difficulty_invalid")
	}
	bestBlockHash := strings.TrimSpace(value.BestBlockHash)
	chainwork := strings.TrimSpace(value.Chainwork)
	if !bitcoinPoWHex256.MatchString(bestBlockHash) || !bitcoinPoWHex256.MatchString(chainwork) {
		return fmt.Errorf("bitcoin_pow_chain_anchor_invalid")
	}
	return nil
}