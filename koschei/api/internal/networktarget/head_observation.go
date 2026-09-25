package networktarget

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type EVMHeadObservationResult struct {
	NetworkID               string    `json:"network_id"`
	ChainID                 string    `json:"chain_id"`
	HeadBlock               uint64    `json:"head_block"`
	ObservedAt              time.Time `json:"observed_at"`
	ChainIDResponseSHA256   string    `json:"chain_id_response_sha256"`
	HeadBlockResponseSHA256 string    `json:"head_block_response_sha256"`
}

func ProbeEVMHeadObservation(ctx context.Context, client *http.Client, endpoint, networkID string, observedAt time.Time) (EVMHeadObservationResult, error) {
	network, ok := LookupNetwork(networkID)
	if !ok || network.Family != "evm" {
		return EVMHeadObservationResult{}, fmt.Errorf("evm_head_unsupported_network")
	}
	if observedAt.IsZero() {
		return EVMHeadObservationResult{}, fmt.Errorf("evm_head_observed_at_required")
	}
	if _, err := validateEVMNodeTelemetryEndpoint(endpoint); err != nil {
		return EVMHeadObservationResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	expectedChainID, ok := ExpectedEVMChainID(networkID)
	if !ok {
		return EVMHeadObservationResult{}, fmt.Errorf("evm_head_chain_id_unknown")
	}
	chainID, chainDigest, err := evmRPCStringWithDigest(ctx, client, endpoint, 701, "eth_chainId", nil)
	if err != nil {
		return EVMHeadObservationResult{}, fmt.Errorf("evm_head_chain_id_unavailable: %w", err)
	}
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	if chainID != expectedChainID {
		return EVMHeadObservationResult{}, fmt.Errorf("evm_rpc_network_mismatch")
	}
	headRaw, headDigest, err := evmRPCStringWithDigest(ctx, client, endpoint, 702, "eth_blockNumber", nil)
	if err != nil {
		return EVMHeadObservationResult{}, fmt.Errorf("evm_head_block_unavailable: %w", err)
	}
	head, err := parseEVMQuantity(headRaw)
	if err != nil {
		return EVMHeadObservationResult{}, fmt.Errorf("evm_head_block_invalid")
	}
	return EVMHeadObservationResult{
		NetworkID: networkID, ChainID: chainID, HeadBlock: head, ObservedAt: observedAt.UTC(),
		ChainIDResponseSHA256: chainDigest, HeadBlockResponseSHA256: headDigest,
	}, nil
}

type BitcoinHeadObservationResult struct {
	NetworkID                    string    `json:"network_id"`
	HeadBlock                    uint64    `json:"head_block"`
	ObservedAt                   time.Time `json:"observed_at"`
	BlockchainInfoResponseSHA256 string    `json:"blockchain_info_response_sha256"`
}

func ProbeBitcoinHeadObservation(ctx context.Context, client *http.Client, endpoint string, observedAt time.Time) (BitcoinHeadObservationResult, error) {
	if observedAt.IsZero() {
		return BitcoinHeadObservationResult{}, fmt.Errorf("bitcoin_head_observed_at_required")
	}
	if _, err := validateBitcoinCoreTelemetryEndpoint(endpoint); err != nil {
		return BitcoinHeadObservationResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	var info struct {
		Chain  string `json:"chain"`
		Blocks int64  `json:"blocks"`
	}
	digest, err := bitcoinCoreRPCWithDigest(ctx, client, endpoint, 703, "getblockchaininfo", &info)
	if err != nil {
		return BitcoinHeadObservationResult{}, fmt.Errorf("bitcoin_head_unavailable: %w", err)
	}
	if info.Chain != "main" || info.Blocks < 0 {
		return BitcoinHeadObservationResult{}, fmt.Errorf("bitcoin_head_response_invalid")
	}
	return BitcoinHeadObservationResult{
		NetworkID: "bitcoin-mainnet", HeadBlock: uint64(info.Blocks), ObservedAt: observedAt.UTC(),
		BlockchainInfoResponseSHA256: digest,
	}, nil
}
