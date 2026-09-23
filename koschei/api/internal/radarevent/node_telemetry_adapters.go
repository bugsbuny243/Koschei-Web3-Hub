package radarevent

import (
	"errors"
	"strconv"
	"strings"

	"koschei/api/internal/networktarget"
)

func BuildEVMNodeTelemetryEvent(producer string, result networktarget.EVMNodeTelemetryResult, sourceDigest string) (Event, error) {
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion {
		return Event{}, errors.New("unsupported evm node telemetry schema")
	}
	if !result.AnalysisPerformed || result.LiveAvailability != "checked" {
		return Event{}, errors.New("evm node telemetry is not a completed live observation")
	}
	if result.Observation.Network.Family != "evm" {
		return Event{}, errors.New("evm node telemetry network mismatch")
	}

	event, err := BuildNetworkHealthEvent(producer, result.Observation, sourceDigest)
	if err != nil {
		return Event{}, err
	}
	event.Facts = append(event.Facts,
		boundFact("chain_id", result.ChainID, "", sourceDigest),
		boundFact("expected_chain_id", result.ExpectedChainID, "", sourceDigest),
		boundFact("client_version", result.ClientVersion, "", sourceDigest),
		boundFact("peer_count", strconv.FormatUint(result.PeerCount, 10), "peers", sourceDigest),
		boundFact("head_block", strconv.FormatUint(result.HeadBlock, 10), "blocks", sourceDigest),
		boundFact("syncing", strconv.FormatBool(result.Syncing), "", sourceDigest),
		boundFact("endpoint_scope", result.EndpointScope, "", sourceDigest),
		boundFact("live_availability", result.LiveAvailability, "", sourceDigest),
	)
	event.Facts = compactFacts(event.Facts)
	return event.Seal()
}

func BuildEthereumBeaconTelemetryEvent(producer string, result networktarget.EthereumBeaconTelemetryResult, sourceDigest string) (Event, error) {
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion {
		return Event{}, errors.New("unsupported ethereum beacon telemetry schema")
	}
	if !result.AnalysisPerformed || result.LiveAvailability != "checked" {
		return Event{}, errors.New("ethereum beacon telemetry is not a completed live observation")
	}
	if result.Observation.Network.ID != "ethereum-mainnet" {
		return Event{}, errors.New("ethereum beacon telemetry network mismatch")
	}

	event, err := BuildNetworkHealthEvent(producer, result.Observation, sourceDigest)
	if err != nil {
		return Event{}, err
	}
	event.Facts = append(event.Facts,
		boundFact("client_version", result.ClientVersion, "", sourceDigest),
		boundFact("connected_peers", strconv.FormatUint(result.ConnectedPeers, 10), "peers", sourceDigest),
		boundFact("head_slot", strconv.FormatUint(result.HeadSlot, 10), "slots", sourceDigest),
		boundFact("sync_distance", strconv.FormatUint(result.SyncDistance, 10), "slots", sourceDigest),
		boundFact("is_syncing", strconv.FormatBool(result.IsSyncing), "", sourceDigest),
		boundFact("is_optimistic", strconv.FormatBool(result.IsOptimistic), "", sourceDigest),
		boundFact("execution_layer_offline", strconv.FormatBool(result.ExecutionLayerOffline), "", sourceDigest),
		boundFact("previous_justified_epoch", strconv.FormatUint(result.PreviousJustified.Epoch, 10), "epochs", sourceDigest),
		boundFact("previous_justified_root", strings.ToLower(result.PreviousJustified.Root), "", sourceDigest),
		boundFact("current_justified_epoch", strconv.FormatUint(result.CurrentJustified.Epoch, 10), "epochs", sourceDigest),
		boundFact("current_justified_root", strings.ToLower(result.CurrentJustified.Root), "", sourceDigest),
		boundFact("finalized_epoch", strconv.FormatUint(result.Finalized.Epoch, 10), "epochs", sourceDigest),
		boundFact("finalized_root", strings.ToLower(result.Finalized.Root), "", sourceDigest),
		boundFact("checkpoint_state_finalized", strconv.FormatBool(result.CheckpointStateFinalized), "", sourceDigest),
		boundFact("endpoint_scope", result.EndpointScope, "", sourceDigest),
		boundFact("live_availability", result.LiveAvailability, "", sourceDigest),
	)
	event.Facts = compactFacts(event.Facts)
	return event.Seal()
}
