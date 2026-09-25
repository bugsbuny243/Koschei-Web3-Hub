package radarevent

import (
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestBuildEVMNodeTelemetryEventFromResultBindsNativeDigests(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "ethereum-mainnet",
		SubjectKind:    "node",
		SubjectID:      "rpc-endpoint:node.example",
		Source:         "evm-json-rpc:node.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "observed",
		ClientFamily:   "Geth/v1.16",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := networktarget.EVMNodeTelemetryResult{
		SchemaVersion:               networktarget.NetworkTelemetrySchemaVersion,
		Observation:                 observation,
		ChainID:                     "0x1",
		ExpectedChainID:             "0x1",
		ChainIDResponseSHA256:       strings.Repeat("a", 64),
		ClientVersion:               "Geth/v1.16",
		ClientVersionResponseSHA256: strings.Repeat("b", 64),
		PeerCount:                   12,
		PeerCountResponseSHA256:     strings.Repeat("c", 64),
		HeadBlock:                   123,
		HeadBlockResponseSHA256:     strings.Repeat("d", 64),
		Syncing:                     false,
		SyncingResponseSHA256:       strings.Repeat("e", 64),
		EndpointScope:               "single_rpc_endpoint_only",
		AnalysisPerformed:           true,
		LiveAvailability:            "checked",
	}
	event, err := BuildEVMNodeTelemetryEventFromResult("evm-node-telemetry-adapter", result)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	if len(event.SourceDigests) != 5 {
		t.Fatalf("source digests=%#v", event.SourceDigests)
	}
}

func TestBuildBitcoinCoreNodeTelemetryEventFromResultBindsNativeDigests(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "bitcoin-mainnet",
		SubjectKind:    "node",
		SubjectID:      "bitcoin-core-endpoint:node.example",
		Source:         "bitcoin-core-json-rpc:node.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "observed",
		ClientFamily:   "/Satoshi:28.0.0/",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := networktarget.BitcoinCoreNodeTelemetryResult{
		SchemaVersion:                networktarget.NetworkTelemetrySchemaVersion,
		Observation:                  observation,
		ClientVersion:                "/Satoshi:28.0.0/",
		ProtocolVersion:              70016,
		Connections:                  11,
		NetworkActive:                true,
		NetworkInfoResponseSHA256:    strings.Repeat("a", 64),
		Blocks:                       900000,
		Headers:                      900000,
		VerificationProgress:         1,
		InitialBlockDownload:         false,
		BlockchainInfoResponseSHA256: strings.Repeat("b", 64),
		EndpointScope:                "single_bitcoin_core_node_only",
		AnalysisPerformed:            true,
		LiveAvailability:             "checked",
	}
	event, err := BuildBitcoinCoreNodeTelemetryEventFromResult("bitcoin-core-telemetry-adapter", result)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	if len(event.SourceDigests) != 2 {
		t.Fatalf("source digests=%#v", event.SourceDigests)
	}
}

func TestBuildEthereumBeaconTelemetryEventFromResultBindsNativeDigests(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "ethereum-mainnet",
		SubjectKind:    "node",
		SubjectID:      "ethereum-beacon-endpoint:node.example",
		Source:         "ethereum-beacon-api:node.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "observed",
		ClientFamily:   "Lighthouse/v7.1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	root := "0x" + strings.Repeat("1", 64)
	result := networktarget.EthereumBeaconTelemetryResult{
		SchemaVersion:            networktarget.NetworkTelemetrySchemaVersion,
		Observation:              observation,
		ClientVersion:            "Lighthouse/v7.1.0",
		VersionResponseSHA256:    strings.Repeat("a", 64),
		ConnectedPeers:           64,
		PeerCountResponseSHA256:  strings.Repeat("b", 64),
		HeadSlot:                 123456,
		SyncDistance:             0,
		IsSyncing:                false,
		ExecutionLayerOffline:    false,
		SyncingResponseSHA256:    strings.Repeat("c", 64),
		PreviousJustified:        networktarget.EthereumBeaconCheckpoint{Epoch: 3800, Root: root},
		CurrentJustified:         networktarget.EthereumBeaconCheckpoint{Epoch: 3801, Root: root},
		Finalized:                networktarget.EthereumBeaconCheckpoint{Epoch: 3799, Root: root},
		CheckpointStateFinalized: true,
		FinalityResponseSHA256:   strings.Repeat("d", 64),
		EndpointScope:            "single_beacon_endpoint_plus_chain_checkpoints",
		AnalysisPerformed:        true,
		LiveAvailability:         "checked",
	}
	event, err := BuildEthereumBeaconTelemetryEventFromResult("ethereum-beacon-telemetry-adapter", result)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	if len(event.SourceDigests) != 4 {
		t.Fatalf("source digests=%#v", event.SourceDigests)
	}
	for _, fact := range event.Facts {
		if fact.Key == "is_optimistic" {
			t.Fatal("combined optimistic state was incorrectly bound to a single source digest")
		}
	}
}

func TestBuildBitcoinPoWNetworkTelemetryEventFromResultBindsNativeDigests(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "bitcoin-mainnet",
		SubjectKind:    "network",
		SubjectID:      "bitcoin-mainnet",
		Source:         "bitcoin-core-pow-estimate:node.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "observed",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := networktarget.BitcoinPoWNetworkTelemetryResult{
		SchemaVersion:                networktarget.NetworkTelemetrySchemaVersion,
		Observation:                  observation,
		EstimatedNetworkHashPS:       8.75e20,
		NetworkHashPSResponseSHA256:  strings.Repeat("a", 64),
		Difficulty:                   123456789.5,
		BestBlockHash:                strings.Repeat("1", 64),
		Chainwork:                    strings.Repeat("2", 64),
		Blocks:                       900001,
		Headers:                      900001,
		InitialBlockDownload:         false,
		BlockchainInfoResponseSHA256: strings.Repeat("b", 64),
		EstimatorScope:               "bitcoin_core_network_estimate_from_single_mainnet_node",
		AnalysisPerformed:            true,
		LiveAvailability:             "checked",
	}
	event, err := BuildBitcoinPoWNetworkTelemetryEventFromResult("bitcoin-pow-telemetry-adapter", result)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	if len(event.SourceDigests) != 2 {
		t.Fatalf("source digests=%#v", event.SourceDigests)
	}
}
