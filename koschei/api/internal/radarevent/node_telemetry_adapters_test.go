package radarevent

import (
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/securityevidence"
)

func TestBuildEVMNodeTelemetryEventPreservesEndpointScope(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "ethereum-mainnet",
		SubjectKind:    "node",
		SubjectID:      "rpc-endpoint:node.example",
		Source:         "evm-json-rpc:node.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "observed",
		ClientFamily:   "Geth/v1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest := strings.Repeat("a", 64)
	event, err := BuildEVMNodeTelemetryEvent("evm-node-telemetry-adapter", networktarget.EVMNodeTelemetryResult{
		SchemaVersion:     networktarget.NetworkTelemetrySchemaVersion,
		Observation:       observation,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		ClientVersion:     "Geth/v1.0",
		PeerCount:         42,
		HeadBlock:         25000000,
		Syncing:           false,
		EndpointScope:     "single_rpc_endpoint_only",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}, sourceDigest)
	if err != nil {
		t.Fatal(err)
	}
	if event.Kind != KindNetworkHealth ||
		event.NetworkID != "ethereum-mainnet" ||
		event.State != securityevidence.StateObserved {
		t.Fatalf("unexpected event: %#v", event)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}

	facts := map[string]Fact{}
	for _, fact := range event.Facts {
		facts[fact.Key] = fact
	}
	for _, key := range []string{"chain_id", "peer_count", "head_block", "syncing", "endpoint_scope"} {
		if _, ok := facts[key]; !ok {
			t.Fatalf("missing fact %q", key)
		}
		if facts[key].EvidenceSHA256 != sourceDigest {
			t.Fatalf("fact %q lost evidence binding", key)
		}
	}
	if facts["endpoint_scope"].Value != "single_rpc_endpoint_only" {
		t.Fatalf("scope=%q", facts["endpoint_scope"].Value)
	}
}

func TestBuildEthereumBeaconTelemetryEventPreservesFinalityEvidence(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "ethereum-mainnet",
		SubjectKind:    "node",
		SubjectID:      "ethereum-beacon-endpoint:beacon.example",
		Source:         "ethereum-beacon-api:beacon.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "observed",
		ClientFamily:   "Lighthouse/v1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest := strings.Repeat("b", 64)
	rootA := "0x" + strings.Repeat("1", 64)
	rootB := "0x" + strings.Repeat("2", 64)
	rootC := "0x" + strings.Repeat("3", 64)
	event, err := BuildEthereumBeaconTelemetryEvent("ethereum-beacon-adapter", networktarget.EthereumBeaconTelemetryResult{
		SchemaVersion:         networktarget.NetworkTelemetrySchemaVersion,
		Observation:           observation,
		ClientVersion:         "Lighthouse/v1.0",
		ConnectedPeers:        80,
		HeadSlot:              123456,
		SyncDistance:          0,
		IsSyncing:             false,
		IsOptimistic:          false,
		ExecutionLayerOffline: false,
		PreviousJustified: networktarget.EthereumBeaconCheckpoint{
			Epoch: 100,
			Root:  rootA,
		},
		CurrentJustified: networktarget.EthereumBeaconCheckpoint{
			Epoch: 101,
			Root:  rootB,
		},
		Finalized: networktarget.EthereumBeaconCheckpoint{
			Epoch: 100,
			Root:  rootC,
		},
		CheckpointStateFinalized: true,
		EndpointScope:            "single_beacon_endpoint_plus_chain_checkpoints",
		AnalysisPerformed:        true,
		LiveAvailability:         "checked",
	}, sourceDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	facts := map[string]Fact{}
	for _, fact := range event.Facts {
		facts[fact.Key] = fact
	}
	if facts["finalized_epoch"].Value != "100" ||
		facts["checkpoint_state_finalized"].Value != "true" {
		t.Fatalf("finality facts=%#v", facts)
	}
	if event.State != securityevidence.StateObserved {
		t.Fatalf("state=%q", event.State)
	}
}

func TestNodeTelemetryAdapterRejectsIncompleteObservation(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "base-mainnet",
		SubjectKind:    "node",
		SubjectID:      "rpc-endpoint:base.example",
		Source:         "evm-json-rpc:base.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "observed",
		ClientFamily:   "op-geth",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildEVMNodeTelemetryEvent("evm-node-telemetry-adapter", networktarget.EVMNodeTelemetryResult{
		SchemaVersion:    networktarget.NetworkTelemetrySchemaVersion,
		Observation:      observation,
		LiveAvailability: "not_checked",
	}, strings.Repeat("c", 64))
	if err == nil {
		t.Fatal("incomplete node telemetry was accepted")
	}
}
