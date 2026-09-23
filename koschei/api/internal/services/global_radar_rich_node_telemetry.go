package services

import (
	"fmt"

	"koschei/api/internal/networktarget"
)

func ProjectEVMNodeTelemetryToGlobalRadar(result networktarget.EVMNodeTelemetryResult) (GlobalRadarObservation, error) {
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion ||
		!result.AnalysisPerformed ||
		result.LiveAvailability != "checked" ||
		result.Observation.SubjectKind != "node" {
		return GlobalRadarObservation{}, fmt.Errorf("completed EVM node telemetry result is required")
	}
	observation, err := AdaptNetworkTelemetryToGlobalRadar(result.Observation)
	if err != nil {
		return GlobalRadarObservation{}, err
	}
	observation.Evidence.Attributes["chain_id"] = result.ChainID
	observation.Evidence.Attributes["expected_chain_id"] = result.ExpectedChainID
	observation.Evidence.Attributes["client_version"] = result.ClientVersion
	observation.Evidence.Attributes["peer_count"] = result.PeerCount
	observation.Evidence.Attributes["head_block"] = result.HeadBlock
	observation.Evidence.Attributes["syncing"] = result.Syncing
	observation.Evidence.Attributes["endpoint_scope"] = result.EndpointScope
	observation.Evidence.Attributes["peer_count_is_network_node_count"] = false
	observation.Evidence.Attributes["network_validator_population_claim"] = false
	return observation, nil
}

func ProjectEthereumBeaconTelemetryToGlobalRadar(result networktarget.EthereumBeaconTelemetryResult) (GlobalRadarObservation, error) {
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion ||
		!result.AnalysisPerformed ||
		result.LiveAvailability != "checked" ||
		result.Observation.Network.ID != "ethereum-mainnet" ||
		result.Observation.SubjectKind != "node" {
		return GlobalRadarObservation{}, fmt.Errorf("completed Ethereum beacon telemetry result is required")
	}
	observation, err := AdaptNetworkTelemetryToGlobalRadar(result.Observation)
	if err != nil {
		return GlobalRadarObservation{}, err
	}
	observation.Evidence.Attributes["client_version"] = result.ClientVersion
	observation.Evidence.Attributes["connected_peers"] = result.ConnectedPeers
	observation.Evidence.Attributes["head_slot"] = result.HeadSlot
	observation.Evidence.Attributes["sync_distance"] = result.SyncDistance
	observation.Evidence.Attributes["is_syncing"] = result.IsSyncing
	observation.Evidence.Attributes["is_optimistic"] = result.IsOptimistic
	observation.Evidence.Attributes["execution_layer_offline"] = result.ExecutionLayerOffline
	observation.Evidence.Attributes["previous_justified_epoch"] = result.PreviousJustified.Epoch
	observation.Evidence.Attributes["previous_justified_root"] = result.PreviousJustified.Root
	observation.Evidence.Attributes["current_justified_epoch"] = result.CurrentJustified.Epoch
	observation.Evidence.Attributes["current_justified_root"] = result.CurrentJustified.Root
	observation.Evidence.Attributes["finalized_epoch"] = result.Finalized.Epoch
	observation.Evidence.Attributes["finalized_root"] = result.Finalized.Root
	observation.Evidence.Attributes["checkpoint_state_finalized"] = result.CheckpointStateFinalized
	observation.Evidence.Attributes["endpoint_scope"] = result.EndpointScope
	observation.Evidence.Attributes["connected_peers_is_global_validator_count"] = false
	observation.Evidence.Attributes["stake_distribution_claim"] = false
	return observation, nil
}

func ProjectBitcoinCoreNodeTelemetryToGlobalRadar(result networktarget.BitcoinCoreNodeTelemetryResult) (GlobalRadarObservation, error) {
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion ||
		!result.AnalysisPerformed ||
		result.LiveAvailability != "checked" ||
		result.Observation.Network.ID != "bitcoin-mainnet" ||
		result.Observation.SubjectKind != "node" {
		return GlobalRadarObservation{}, fmt.Errorf("completed Bitcoin Core node telemetry result is required")
	}
	observation, err := AdaptNetworkTelemetryToGlobalRadar(result.Observation)
	if err != nil {
		return GlobalRadarObservation{}, err
	}
	observation.Evidence.Attributes["client_version"] = result.ClientVersion
	observation.Evidence.Attributes["protocol_version"] = result.ProtocolVersion
	observation.Evidence.Attributes["connections"] = result.Connections
	observation.Evidence.Attributes["network_active"] = result.NetworkActive
	observation.Evidence.Attributes["blocks"] = result.Blocks
	observation.Evidence.Attributes["headers"] = result.Headers
	observation.Evidence.Attributes["verification_progress"] = result.VerificationProgress
	observation.Evidence.Attributes["initial_block_download"] = result.InitialBlockDownload
	observation.Evidence.Attributes["endpoint_scope"] = result.EndpointScope
	observation.Evidence.Attributes["connections_is_global_node_count"] = false
	observation.Evidence.Attributes["miner_distribution_claim"] = false
	return observation, nil
}

func ProjectBitcoinPoWNetworkTelemetryToGlobalRadar(result networktarget.BitcoinPoWNetworkTelemetryResult) (GlobalRadarObservation, error) {
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion ||
		!result.AnalysisPerformed ||
		result.LiveAvailability != "checked" ||
		result.Observation.Network.ID != "bitcoin-mainnet" ||
		result.Observation.SubjectKind != "network" {
		return GlobalRadarObservation{}, fmt.Errorf("completed Bitcoin PoW network telemetry result is required")
	}
	observation, err := AdaptNetworkTelemetryToGlobalRadar(result.Observation)
	if err != nil {
		return GlobalRadarObservation{}, err
	}
	observation.Evidence.Attributes["estimated_network_hash_ps"] = result.EstimatedNetworkHashPS
	observation.Evidence.Attributes["difficulty"] = result.Difficulty
	observation.Evidence.Attributes["best_block_hash"] = result.BestBlockHash
	observation.Evidence.Attributes["chainwork"] = result.Chainwork
	observation.Evidence.Attributes["blocks"] = result.Blocks
	observation.Evidence.Attributes["headers"] = result.Headers
	observation.Evidence.Attributes["initial_block_download"] = result.InitialBlockDownload
	observation.Evidence.Attributes["estimator_scope"] = result.EstimatorScope
	observation.Evidence.Attributes["estimated_hash_rate_is_physical_miner_count"] = false
	observation.Evidence.Attributes["miner_or_pool_market_share_claim"] = false
	return observation, nil
}
