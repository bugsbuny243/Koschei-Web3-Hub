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
