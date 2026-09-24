package radarevent

import (
	"errors"
	"strconv"
	"strings"

	"koschei/api/internal/networktarget"
)

func BuildEVMNodeTelemetryEventFromResult(producer string, result networktarget.EVMNodeTelemetryResult) (Event, error) {
	digests := []string{
		strings.TrimSpace(result.ChainIDResponseSHA256),
		strings.TrimSpace(result.ClientVersionResponseSHA256),
		strings.TrimSpace(result.PeerCountResponseSHA256),
		strings.TrimSpace(result.HeadBlockResponseSHA256),
		strings.TrimSpace(result.SyncingResponseSHA256),
	}
	for _, digest := range digests {
		if digest == "" {
			return Event{}, errors.New("evm node telemetry native response digests are required")
		}
	}
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion ||
		!result.AnalysisPerformed ||
		result.LiveAvailability != "checked" {
		return Event{}, errors.New("evm node telemetry is not a completed live observation")
	}

	state, err := evidenceStateFromString(result.Observation.EvidenceStatus)
	if err != nil {
		return Event{}, err
	}
	facts := compactFacts([]Fact{
		boundFact("chain_id", result.ChainID, "", digests[0]),
		boundFact("expected_chain_id", result.ExpectedChainID, "", digests[0]),
		boundFact("client_version", result.ClientVersion, "", digests[1]),
		boundFact("peer_count", strconv.FormatUint(result.PeerCount, 10), "peers", digests[2]),
		boundFact("head_block", strconv.FormatUint(result.HeadBlock, 10), "blocks", digests[3]),
		boundFact("syncing", strconv.FormatBool(result.Syncing), "", digests[4]),
		boundFact("endpoint_scope", result.EndpointScope, "", digests[0]),
		boundFact("live_availability", result.LiveAvailability, "", digests[4]),
	})

	event := Event{
		SchemaVersion:    SchemaVersionV1,
		Producer:         producer,
		Kind:             KindNetworkHealth,
		NetworkID:        result.Observation.Network.ID,
		SubjectKind:      result.Observation.SubjectKind,
		SubjectID:        result.Observation.SubjectID,
		ObservedAtUnixMS: result.Observation.ObservedAt.UnixMilli(),
		State:            state,
		NativeRefs: []NativeReference{{
			Kind:  result.Observation.SubjectKind,
			Value: result.Observation.SubjectID,
		}},
		SourceDigests: append([]string(nil), digests...),
		Facts:         facts,
	}
	return event.Seal()
}

func BuildBitcoinCoreNodeTelemetryEventFromResult(producer string, result networktarget.BitcoinCoreNodeTelemetryResult) (Event, error) {
	networkDigest := strings.TrimSpace(result.NetworkInfoResponseSHA256)
	blockchainDigest := strings.TrimSpace(result.BlockchainInfoResponseSHA256)
	if networkDigest == "" || blockchainDigest == "" {
		return Event{}, errors.New("bitcoin core telemetry native response digests are required")
	}
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion ||
		!result.AnalysisPerformed ||
		result.LiveAvailability != "checked" {
		return Event{}, errors.New("bitcoin core telemetry is not a completed live observation")
	}

	state, err := evidenceStateFromString(result.Observation.EvidenceStatus)
	if err != nil {
		return Event{}, err
	}
	facts := compactFacts([]Fact{
		boundFact("client_version", result.ClientVersion, "", networkDigest),
		boundFact("protocol_version", strconv.Itoa(result.ProtocolVersion), "", networkDigest),
		boundFact("connections", strconv.Itoa(result.Connections), "connections", networkDigest),
		boundFact("network_active", strconv.FormatBool(result.NetworkActive), "", networkDigest),
		boundFact("blocks", strconv.FormatInt(result.Blocks, 10), "blocks", blockchainDigest),
		boundFact("headers", strconv.FormatInt(result.Headers, 10), "blocks", blockchainDigest),
		boundFact("verification_progress", strconv.FormatFloat(result.VerificationProgress, 'f', -1, 64), "ratio", blockchainDigest),
		boundFact("initial_block_download", strconv.FormatBool(result.InitialBlockDownload), "", blockchainDigest),
		boundFact("endpoint_scope", result.EndpointScope, "", blockchainDigest),
		boundFact("live_availability", result.LiveAvailability, "", blockchainDigest),
	})

	event := Event{
		SchemaVersion:    SchemaVersionV1,
		Producer:         producer,
		Kind:             KindNetworkHealth,
		NetworkID:        result.Observation.Network.ID,
		SubjectKind:      result.Observation.SubjectKind,
		SubjectID:        result.Observation.SubjectID,
		ObservedAtUnixMS: result.Observation.ObservedAt.UnixMilli(),
		State:            state,
		NativeRefs: []NativeReference{{
			Kind:  result.Observation.SubjectKind,
			Value: result.Observation.SubjectID,
		}},
		SourceDigests: []string{networkDigest, blockchainDigest},
		Facts:         facts,
	}
	return event.Seal()
}
