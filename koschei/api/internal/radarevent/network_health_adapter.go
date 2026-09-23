package radarevent

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/securityevidence"
)

func BuildNetworkHealthEvent(producer string, observation networktarget.NetworkTelemetryObservation, sourceDigest string) (Event, error) {
	if observation.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion {
		return Event{}, fmt.Errorf("unsupported network telemetry schema %q", observation.SchemaVersion)
	}

	state, err := networkTelemetryEvidenceState(observation.EvidenceStatus)
	if err != nil {
		return Event{}, err
	}

	facts := make([]Fact, 0, 8)
	addFact := func(key, value, unit string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		facts = append(facts, Fact{
			Key:            key,
			Value:          value,
			Unit:           unit,
			EvidenceSHA256: sourceDigest,
		})
	}

	addFact("telemetry_source", observation.Source, "")
	addFact("client_family", observation.ClientFamily, "")
	if observation.StakeSharePct != nil {
		addFact("stake_share_pct", strconv.FormatFloat(*observation.StakeSharePct, 'f', -1, 64), "percent")
	}
	if observation.HashSharePct != nil {
		addFact("hash_share_pct", strconv.FormatFloat(*observation.HashSharePct, 'f', -1, 64), "percent")
	}
	addFact("asn", observation.ASN, "")
	addFact("country_code", observation.CountryCode, "")
	addFact("location_source", observation.LocationSource, "")

	if len(observation.MissingEvidence) > 0 {
		missing := append([]string(nil), observation.MissingEvidence...)
		sort.Strings(missing)
		addFact("missing_evidence", strings.Join(missing, ","), "")
	}

	event := Event{
		SchemaVersion:    SchemaVersionV1,
		Producer:         producer,
		Kind:             KindNetworkHealth,
		NetworkID:        observation.Network.ID,
		SubjectKind:      observation.SubjectKind,
		SubjectID:        observation.SubjectID,
		ObservedAtUnixMS: observation.ObservedAt.UnixMilli(),
		State:            state,
		NativeRefs: []NativeReference{{
			Kind:  observation.SubjectKind,
			Value: observation.SubjectID,
		}},
		SourceDigests: []string{sourceDigest},
		Facts:         facts,
	}
	return event.Seal()
}

func networkTelemetryEvidenceState(value string) (securityevidence.EvidenceState, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "observed":
		return securityevidence.StateObserved, nil
	case "verified":
		return securityevidence.StateVerified, nil
	default:
		return "", fmt.Errorf("unsupported network telemetry evidence state %q", value)
	}
}
