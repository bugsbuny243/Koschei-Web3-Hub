package networktarget

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type EVMRollupBlockRef struct {
	Number uint64 `json:"number"`
	Hash   string `json:"hash"`
}

type EVMRollupFinalityTelemetryResult struct {
	SchemaVersion     string                      `json:"schema_version"`
	Observation       NetworkTelemetryObservation `json:"observation"`
	ChainID           string                      `json:"chain_id"`
	ExpectedChainID   string                      `json:"expected_chain_id"`
	Latest            EVMRollupBlockRef           `json:"latest"`
	Safe              EVMRollupBlockRef           `json:"safe"`
	Finalized         EVMRollupBlockRef           `json:"finalized"`
	LatestToSafe      uint64                      `json:"latest_to_safe_blocks"`
	LatestToFinalized uint64                      `json:"latest_to_finalized_blocks"`
	EndpointScope     string                      `json:"endpoint_scope"`
	AnalysisPerformed bool                        `json:"analysis_performed"`
	LiveAvailability  string                      `json:"live_availability"`
}

type evmRollupBlockWire struct {
	Number string `json:"number"`
	Hash   string `json:"hash"`
}

// ProbeEVMRollupFinality observes the standard EVM latest/safe/finalized block
// tags on one configured RPC endpoint. It does not infer sequencer identity,
// fraud/fault-proof status, or L1 settlement guarantees beyond the block tags
// actually returned by that endpoint.
func ProbeEVMRollupFinality(ctx context.Context, client *http.Client, endpoint, networkID string, observedAt time.Time) (EVMRollupFinalityTelemetryResult, error) {
	if !isSupportedRollupFinalityNetwork(networkID) {
		return EVMRollupFinalityTelemetryResult{}, fmt.Errorf("evm_rollup_finality_unsupported_network")
	}
	if observedAt.IsZero() {
		return EVMRollupFinalityTelemetryResult{}, fmt.Errorf("evm_rollup_finality_observed_at_required")
	}

	parsed, err := validateEVMNodeTelemetryEndpoint(endpoint)
	if err != nil {
		return EVMRollupFinalityTelemetryResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	expectedChainID, ok := ExpectedEVMChainID(networkID)
	if !ok {
		return EVMRollupFinalityTelemetryResult{}, fmt.Errorf("evm_rollup_finality_chain_id_unknown")
	}
	chainID, err := evmRPCString(ctx, client, endpoint, 301, "eth_chainId", nil)
	if err != nil {
		return EVMRollupFinalityTelemetryResult{}, fmt.Errorf("evm_rollup_chain_id_unavailable: %w", err)
	}
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	if chainID != expectedChainID {
		return EVMRollupFinalityTelemetryResult{}, fmt.Errorf("evm_rpc_network_mismatch")
	}

	latest, err := probeEVMRollupBlockTag(ctx, client, endpoint, 302, "latest")
	if err != nil {
		return EVMRollupFinalityTelemetryResult{}, fmt.Errorf("evm_rollup_latest_unavailable: %w", err)
	}
	safe, err := probeEVMRollupBlockTag(ctx, client, endpoint, 303, "safe")
	if err != nil {
		return EVMRollupFinalityTelemetryResult{}, fmt.Errorf("evm_rollup_safe_unavailable: %w", err)
	}
	finalized, err := probeEVMRollupBlockTag(ctx, client, endpoint, 304, "finalized")
	if err != nil {
		return EVMRollupFinalityTelemetryResult{}, fmt.Errorf("evm_rollup_finalized_unavailable: %w", err)
	}

	if finalized.Number > safe.Number || safe.Number > latest.Number {
		return EVMRollupFinalityTelemetryResult{}, fmt.Errorf("evm_rollup_finality_order_invalid")
	}

	host := strings.ToLower(parsed.Hostname())
	observation, err := NormalizeNetworkTelemetry(NetworkTelemetryInput{
		NetworkID:      networkID,
		SubjectKind:    "network",
		SubjectID:      networkID,
		Source:         "evm-json-rpc-finality:" + host,
		ObservedAt:     observedAt,
		EvidenceStatus: "observed",
	})
	if err != nil {
		return EVMRollupFinalityTelemetryResult{}, err
	}

	return EVMRollupFinalityTelemetryResult{
		SchemaVersion:     NetworkTelemetrySchemaVersion,
		Observation:       observation,
		ChainID:           chainID,
		ExpectedChainID:   expectedChainID,
		Latest:            latest,
		Safe:              safe,
		Finalized:         finalized,
		LatestToSafe:      latest.Number - safe.Number,
		LatestToFinalized: latest.Number - finalized.Number,
		EndpointScope:     "single_rpc_endpoint_block_tags",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}, nil
}

func isSupportedRollupFinalityNetwork(networkID string) bool {
	switch strings.TrimSpace(networkID) {
	case "base-mainnet", "optimism-mainnet", "arbitrum-mainnet":
		return true
	default:
		return false
	}
}

func probeEVMRollupBlockTag(ctx context.Context, client *http.Client, endpoint string, id int, tag string) (EVMRollupBlockRef, error) {
	raw, isNull, err := evmRPCRaw(ctx, client, endpoint, id, "eth_getBlockByNumber", []any{tag, false})
	if err != nil {
		return EVMRollupBlockRef{}, err
	}
	if isNull {
		return EVMRollupBlockRef{}, fmt.Errorf("evm_rollup_block_tag_null")
	}

	var block evmRollupBlockWire
	if err := json.Unmarshal(raw, &block); err != nil {
		return EVMRollupBlockRef{}, fmt.Errorf("evm_rollup_block_invalid")
	}
	number := strings.ToLower(strings.TrimSpace(block.Number))
	hash := strings.ToLower(strings.TrimSpace(block.Hash))
	if !validEVMHexQuantity(number) || !validEVMTransactionHash(hash) {
		return EVMRollupBlockRef{}, fmt.Errorf("evm_rollup_block_binding_invalid")
	}
	parsedNumber, err := parseEVMQuantity(number)
	if err != nil {
		return EVMRollupBlockRef{}, fmt.Errorf("evm_rollup_block_number_invalid")
	}
	return EVMRollupBlockRef{Number: parsedNumber, Hash: hash}, nil
}
