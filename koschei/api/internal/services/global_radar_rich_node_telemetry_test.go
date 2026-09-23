package services

import (
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func radarTelemetryFixture(t *testing.T, networkID, subjectKind, subjectID, source, clientFamily string) networktarget.NetworkTelemetryObservation {
	t.Helper()
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      networkID,
		SubjectKind:    subjectKind,
		SubjectID:      subjectID,
		Source:         source,
		ObservedAt:     time.Date(2026, 9, 23, 19, 30, 0, 0, time.UTC),
		EvidenceStatus: IntelligenceEvidenceObserved,
		ClientFamily:   clientFamily,
	})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestProjectEVMNodeTelemetryToGlobalRadarPreservesEndpointScope(t *testing.T) {
	result := networktarget.EVMNodeTelemetryResult{
		SchemaVersion:     networktarget.NetworkTelemetrySchemaVersion,
		Observation:       radarTelemetryFixture(t, "ethereum-mainnet", "node", "rpc-endpoint:test", "evm-json-rpc:test", "Geth/v1"),
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		ClientVersion:     "Geth/v1",
		PeerCount:         37,
		HeadBlock:         123456,
		Syncing:           false,
		EndpointScope:     "single_rpc_endpoint_only",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}
	got, err := ProjectEVMNodeTelemetryToGlobalRadar(result)
	if err != nil {
		t.Fatal(err)
	}
	if got.Evidence.Attributes["peer_count"] != uint64(37) ||
		got.Evidence.Attributes["peer_count_is_network_node_count"] != false ||
		got.Evidence.Attributes["network_validator_population_claim"] != false {
		t.Fatalf("EVM endpoint scope was overclaimed: %#v", got.Evidence.Attributes)
	}
}

func TestProjectEthereumBeaconTelemetryToGlobalRadarKeepsFinalityButNoPopulationClaim(t *testing.T) {
	result := networktarget.EthereumBeaconTelemetryResult{
		SchemaVersion:         networktarget.NetworkTelemetrySchemaVersion,
		Observation:           radarTelemetryFixture(t, "ethereum-mainnet", "node", "ethereum-beacon-endpoint:test", "ethereum-beacon-api:test", "Lighthouse/v1"),
		ClientVersion:         "Lighthouse/v1",
		ConnectedPeers:        90,
		HeadSlot:              123,
		SyncDistance:          0,
		Finalized:             networktarget.EthereumBeaconCheckpoint{Epoch: 44, Root: "0x" + string(make([]byte, 64))},
		EndpointScope:         "single_beacon_endpoint_plus_chain_checkpoints",
		AnalysisPerformed:     true,
		LiveAvailability:      "checked",
	}
	result.Finalized.Root = "0x" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	got, err := ProjectEthereumBeaconTelemetryToGlobalRadar(result)
	if err != nil {
		t.Fatal(err)
	}
	if got.Evidence.Attributes["finalized_epoch"] != uint64(44) ||
		got.Evidence.Attributes["connected_peers_is_global_validator_count"] != false ||
		got.Evidence.Attributes["stake_distribution_claim"] != false {
		t.Fatalf("beacon evidence boundary changed: %#v", got.Evidence.Attributes)
	}
}

func TestProjectBitcoinTelemetryKeepsNodeAndPoWScopesSeparate(t *testing.T) {
	node := networktarget.BitcoinCoreNodeTelemetryResult{
		SchemaVersion:        networktarget.NetworkTelemetrySchemaVersion,
		Observation:          radarTelemetryFixture(t, "bitcoin-mainnet", "node", "bitcoin-core-endpoint:test", "bitcoin-core-json-rpc:test", "/Satoshi:30.0/"),
		ClientVersion:        "/Satoshi:30.0/",
		ProtocolVersion:      70016,
		Connections:          125,
		NetworkActive:        true,
		Blocks:               999,
		Headers:              999,
		VerificationProgress: 1,
		EndpointScope:        "single_bitcoin_core_node_only",
		AnalysisPerformed:    true,
		LiveAvailability:     "checked",
	}
	nodeObservation, err := ProjectBitcoinCoreNodeTelemetryToGlobalRadar(node)
	if err != nil {
		t.Fatal(err)
	}
	if nodeObservation.Evidence.Attributes["connections_is_global_node_count"] != false ||
		nodeObservation.Evidence.Attributes["miner_distribution_claim"] != false {
		t.Fatalf("Bitcoin Core node scope was overclaimed: %#v", nodeObservation.Evidence.Attributes)
	}

	powObservation := radarTelemetryFixture(t, "bitcoin-mainnet", "network", "bitcoin-mainnet", "bitcoin-core-pow-estimate:test", "")
	pow := networktarget.BitcoinPoWNetworkTelemetryResult{
		SchemaVersion:          networktarget.NetworkTelemetrySchemaVersion,
		Observation:            powObservation,
		EstimatedNetworkHashPS: 9.5e20,
		Difficulty:             100,
		BestBlockHash:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Chainwork:              "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Blocks:                 999,
		Headers:                999,
		EstimatorScope:         "bitcoin_core_network_estimate_from_single_mainnet_node",
		AnalysisPerformed:      true,
		LiveAvailability:       "checked",
	}
	got, err := ProjectBitcoinPoWNetworkTelemetryToGlobalRadar(pow)
	if err != nil {
		t.Fatal(err)
	}
	if got.Evidence.Attributes["estimated_network_hash_ps"] != 9.5e20 ||
		got.Evidence.Attributes["estimated_hash_rate_is_physical_miner_count"] != false ||
		got.Evidence.Attributes["miner_or_pool_market_share_claim"] != false {
		t.Fatalf("Bitcoin PoW estimate scope was overclaimed: %#v", got.Evidence.Attributes)
	}
}

func TestRichTelemetryProjectionRequiresCompletedLiveResult(t *testing.T) {
	result := networktarget.EVMNodeTelemetryResult{
		SchemaVersion:    networktarget.NetworkTelemetrySchemaVersion,
		Observation:      radarTelemetryFixture(t, "ethereum-mainnet", "node", "rpc-endpoint:test", "evm-json-rpc:test", "Geth/v1"),
		LiveAvailability: "not_checked",
	}
	if _, err := ProjectEVMNodeTelemetryToGlobalRadar(result); err == nil {
		t.Fatal("incomplete telemetry result was accepted")
	}
}
