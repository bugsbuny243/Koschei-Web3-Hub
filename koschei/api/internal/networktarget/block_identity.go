package networktarget

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type BlockIdentity struct {
	NetworkID    string    `json:"network_id"`
	Height       uint64    `json:"height"`
	Hash         string    `json:"hash"`
	ParentHash   string    `json:"parent_hash,omitempty"`
	ObservedAt   time.Time `json:"observed_at"`
	SourceSHA256 string    `json:"source_sha256"`
}

func ProbeEVMBlockIdentity(ctx context.Context, client *http.Client, endpoint, networkID string, height uint64, observedAt time.Time) (BlockIdentity, error) {
	network, ok := LookupNetwork(networkID)
	if !ok || network.Family != "evm" {
		return BlockIdentity{}, fmt.Errorf("evm_block_identity_unsupported_network")
	}
	if observedAt.IsZero() {
		return BlockIdentity{}, fmt.Errorf("evm_block_identity_observed_at_required")
	}
	if _, err := validateEVMNodeTelemetryEndpoint(endpoint); err != nil {
		return BlockIdentity{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	expectedChainID, ok := ExpectedEVMChainID(networkID)
	if !ok {
		return BlockIdentity{}, fmt.Errorf("evm_block_identity_chain_id_unknown")
	}
	chainID, _, err := evmRPCStringWithDigest(ctx, client, endpoint, 811, "eth_chainId", nil)
	if err != nil {
		return BlockIdentity{}, fmt.Errorf("evm_block_identity_chain_id_unavailable: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(chainID)) != expectedChainID {
		return BlockIdentity{}, fmt.Errorf("evm_rpc_network_mismatch")
	}

	tag := fmt.Sprintf("0x%x", height)
	raw, digest, err := evmIngestRPCRawWithDigest(ctx, client, endpoint, 812, "eth_getBlockByNumber", []any{tag, false})
	if err != nil {
		return BlockIdentity{}, fmt.Errorf("evm_block_identity_unavailable: %w", err)
	}
	var block evmBlockIngestWire
	if err := json.Unmarshal(raw, &block); err != nil {
		return BlockIdentity{}, fmt.Errorf("evm_block_identity_invalid")
	}
	number, err := parseEVMQuantity(strings.TrimSpace(block.Number))
	if err != nil || number != height {
		return BlockIdentity{}, fmt.Errorf("evm_block_identity_height_mismatch")
	}
	hash := strings.ToLower(strings.TrimSpace(block.Hash))
	parentHash := strings.ToLower(strings.TrimSpace(block.ParentHash))
	if !validEVMTransactionHash(hash) || !validEVMTransactionHash(parentHash) {
		return BlockIdentity{}, fmt.Errorf("evm_block_identity_hash_invalid")
	}
	return BlockIdentity{
		NetworkID:    networkID,
		Height:       height,
		Hash:         hash,
		ParentHash:   parentHash,
		ObservedAt:   observedAt.UTC(),
		SourceSHA256: digest,
	}, nil
}

type bitcoinBlockHeaderIdentityWire struct {
	Hash              string `json:"hash"`
	Height            int64  `json:"height"`
	PreviousBlockHash string `json:"previousblockhash"`
}

func ProbeBitcoinBlockIdentity(ctx context.Context, client *http.Client, endpoint string, height uint64, observedAt time.Time) (BlockIdentity, error) {
	if observedAt.IsZero() {
		return BlockIdentity{}, fmt.Errorf("bitcoin_block_identity_observed_at_required")
	}
	if _, err := validateBitcoinCoreTelemetryEndpoint(endpoint); err != nil {
		return BlockIdentity{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	var chainInfo struct {
		Chain  string `json:"chain"`
		Blocks int64  `json:"blocks"`
	}
	if _, err := bitcoinCoreRPCParamsWithDigest(ctx, client, endpoint, 911, "getblockchaininfo", []any{}, &chainInfo); err != nil {
		return BlockIdentity{}, fmt.Errorf("bitcoin_block_identity_chain_unavailable: %w", err)
	}
	if chainInfo.Chain != "main" || chainInfo.Blocks < 0 || uint64(chainInfo.Blocks) < height {
		return BlockIdentity{}, fmt.Errorf("bitcoin_block_identity_chain_mismatch")
	}
	var blockHash string
	if _, err := bitcoinCoreRPCParamsWithDigest(ctx, client, endpoint, 912, "getblockhash", []any{height}, &blockHash); err != nil {
		return BlockIdentity{}, fmt.Errorf("bitcoin_block_identity_hash_unavailable: %w", err)
	}
	blockHash = strings.ToLower(strings.TrimSpace(blockHash))
	if !validBitcoinIngestHash(blockHash) {
		return BlockIdentity{}, fmt.Errorf("bitcoin_block_identity_hash_invalid")
	}
	var header bitcoinBlockHeaderIdentityWire
	digest, err := bitcoinCoreRPCParamsWithDigest(ctx, client, endpoint, 913, "getblockheader", []any{blockHash, true}, &header)
	if err != nil {
		return BlockIdentity{}, fmt.Errorf("bitcoin_block_identity_header_unavailable: %w", err)
	}
	if header.Height < 0 || uint64(header.Height) != height || strings.ToLower(strings.TrimSpace(header.Hash)) != blockHash {
		return BlockIdentity{}, fmt.Errorf("bitcoin_block_identity_binding_invalid")
	}
	parentHash := strings.ToLower(strings.TrimSpace(header.PreviousBlockHash))
	if height > 0 && !validBitcoinIngestHash(parentHash) {
		return BlockIdentity{}, fmt.Errorf("bitcoin_block_identity_parent_invalid")
	}
	return BlockIdentity{
		NetworkID:    "bitcoin-mainnet",
		Height:       height,
		Hash:         blockHash,
		ParentHash:   parentHash,
		ObservedAt:   observedAt.UTC(),
		SourceSHA256: digest,
	}, nil
}
