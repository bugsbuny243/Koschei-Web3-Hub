package radarevent

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

func BuildEVMAddressProbeEvent(producer string, result networktarget.EVMProbeResult, observedAt time.Time, sourceDigest string) (Event, error) {
	if result.SchemaVersion != networktarget.SchemaVersion {
		return Event{}, errors.New("unsupported evm probe schema")
	}
	if !result.AnalysisPerformed || result.LiveAvailability != "checked" {
		return Event{}, errors.New("evm probe is not a completed live observation")
	}
	if observedAt.IsZero() {
		return Event{}, errors.New("evm probe observation time is required")
	}
	state, err := evidenceStateFromString(result.EvidenceStatus)
	if err != nil {
		return Event{}, err
	}
	subjectID := strings.TrimSpace(result.Resolution.CanonicalRef)
	if subjectID == "" {
		return Event{}, errors.New("evm probe canonical subject is required")
	}

	facts := []Fact{
		boundFact("chain_id", result.ChainID, "", sourceDigest),
		boundFact("expected_chain_id", result.ExpectedChainID, "", sourceDigest),
		boundFact("contract_code_state", result.ContractCodeState, "", sourceDigest),
		boundFact("delegation_state", result.DelegationState, "", sourceDigest),
		boundFact("live_availability", result.LiveAvailability, "", sourceDigest),
	}
	if value := strings.TrimSpace(result.ContractCodeHash); value != "" {
		facts = append(facts, boundFact("contract_code_sha256", value, "", sourceDigest))
	}
	if value := strings.TrimSpace(result.DelegationTarget); value != "" {
		facts = append(facts, boundFact("delegation_target", value, "", sourceDigest))
	}

	event := Event{
		SchemaVersion:    SchemaVersionV1,
		Producer:         producer,
		Kind:             KindAccount,
		NetworkID:        result.Resolution.Network.ID,
		SubjectKind:      "address",
		SubjectID:        subjectID,
		ObservedAtUnixMS: observedAt.UnixMilli(),
		State:            state,
		NativeRefs: []NativeReference{
			{Kind: "canonical_ref", Value: subjectID},
			{Kind: "address", Value: result.Resolution.Address},
		},
		SourceDigests: []string{sourceDigest},
		Facts:         compactFacts(facts),
	}
	return event.Seal()
}

func BuildBitcoinAddressProbeEvent(producer string, result networktarget.BitcoinProbeResult, observedAt time.Time, sourceDigest string) (Event, error) {
	if result.SchemaVersion != networktarget.SchemaVersion {
		return Event{}, errors.New("unsupported bitcoin probe schema")
	}
	if !result.AnalysisPerformed || result.LiveAvailability != "checked" {
		return Event{}, errors.New("bitcoin probe is not a completed live observation")
	}
	if observedAt.IsZero() {
		return Event{}, errors.New("bitcoin probe observation time is required")
	}
	state, err := evidenceStateFromString(result.EvidenceStatus)
	if err != nil {
		return Event{}, err
	}
	subjectID := strings.TrimSpace(result.Resolution.CanonicalRef)
	if subjectID == "" {
		return Event{}, errors.New("bitcoin probe canonical subject is required")
	}

	facts := compactFacts([]Fact{
		boundFact("genesis_hash", result.GenesisHash, "", sourceDigest),
		boundFact("expected_genesis_hash", result.ExpectedGenesisHash, "", sourceDigest),
		boundFact("activity_state", result.ActivityState, "", sourceDigest),
		boundFact("confirmed_tx_count", strconv.FormatInt(result.ConfirmedTXCount, 10), "transactions", sourceDigest),
		boundFact("mempool_tx_count", strconv.FormatInt(result.MempoolTXCount, 10), "transactions", sourceDigest),
		boundFact("funded_sats", strconv.FormatInt(result.FundedSats, 10), "sats", sourceDigest),
		boundFact("spent_sats", strconv.FormatInt(result.SpentSats, 10), "sats", sourceDigest),
		boundFact("live_availability", result.LiveAvailability, "", sourceDigest),
	})

	event := Event{
		SchemaVersion:    SchemaVersionV1,
		Producer:         producer,
		Kind:             KindAccount,
		NetworkID:        result.Resolution.Network.ID,
		SubjectKind:      "address",
		SubjectID:        subjectID,
		ObservedAtUnixMS: observedAt.UnixMilli(),
		State:            state,
		NativeRefs: []NativeReference{
			{Kind: "canonical_ref", Value: subjectID},
			{Kind: "address", Value: result.Resolution.Address},
		},
		SourceDigests: []string{sourceDigest},
		Facts:         facts,
	}
	return event.Seal()
}

func BuildSuiIdentityEvent(producer string, result networktarget.SuiMainnetIdentityProbeResult, sourceDigest string) (Event, error) {
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion {
		return Event{}, errors.New("unsupported sui identity probe schema")
	}
	if !result.AnalysisPerformed || result.LiveAvailability != "checked" {
		return Event{}, errors.New("sui identity probe is not a completed live observation")
	}
	if result.Observation.Network.ID != "sui-mainnet" {
		return Event{}, errors.New("sui identity probe network mismatch")
	}

	event, err := BuildNetworkHealthEvent(producer, result.Observation, sourceDigest)
	if err != nil {
		return Event{}, err
	}
	event.Facts = append(event.Facts,
		boundFact("chain_identifier", result.ChainIdentifier, "", sourceDigest),
		boundFact("endpoint_scope", result.EndpointScope, "", sourceDigest),
		boundFact("live_availability", result.LiveAvailability, "", sourceDigest),
	)
	event.Facts = compactFacts(event.Facts)
	return event.Seal()
}

func BuildAptosIdentityEvent(producer string, result networktarget.AptosMainnetIdentityProbeResult, sourceDigest string) (Event, error) {
	if result.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion {
		return Event{}, errors.New("unsupported aptos identity probe schema")
	}
	if !result.AnalysisPerformed || result.LiveAvailability != "checked" {
		return Event{}, errors.New("aptos identity probe is not a completed live observation")
	}
	if result.Observation.Network.ID != "aptos-mainnet" || result.ChainID != 1 {
		return Event{}, errors.New("aptos identity probe network mismatch")
	}

	event, err := BuildNetworkHealthEvent(producer, result.Observation, sourceDigest)
	if err != nil {
		return Event{}, err
	}
	event.Facts = append(event.Facts,
		boundFact("chain_id", strconv.FormatUint(uint64(result.ChainID), 10), "", sourceDigest),
		boundFact("epoch", strconv.FormatUint(result.Epoch, 10), "", sourceDigest),
		boundFact("ledger_version", strconv.FormatUint(result.LedgerVersion, 10), "", sourceDigest),
		boundFact("ledger_timestamp", strconv.FormatUint(result.LedgerTimestamp, 10), "microseconds", sourceDigest),
		boundFact("block_height", strconv.FormatUint(result.BlockHeight, 10), "blocks", sourceDigest),
		boundFact("node_role", result.NodeRole, "", sourceDigest),
		boundFact("git_hash", result.GitHash, "", sourceDigest),
		boundFact("endpoint_scope", result.EndpointScope, "", sourceDigest),
		boundFact("live_availability", result.LiveAvailability, "", sourceDigest),
	)
	event.Facts = compactFacts(event.Facts)
	return event.Seal()
}

func boundFact(key, value, unit, sourceDigest string) Fact {
	return Fact{
		Key:            strings.TrimSpace(key),
		Value:          strings.TrimSpace(value),
		Unit:           strings.TrimSpace(unit),
		EvidenceSHA256: sourceDigest,
	}
}

func compactFacts(values []Fact) []Fact {
	out := make([]Fact, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.Key) == "" || strings.TrimSpace(value.Value) == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
