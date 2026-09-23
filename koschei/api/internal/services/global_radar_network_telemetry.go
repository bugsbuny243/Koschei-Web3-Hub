package services

import (
	"errors"
	"strings"

	"koschei/api/internal/networktarget"
)

// AdaptNetworkTelemetryToGlobalRadar projects bounded node/validator/miner/network
// telemetry into the Global Radar evidence plane. Missing telemetry dimensions
// remain explicit missing evidence and never become a risk or safety conclusion.
func AdaptNetworkTelemetryToGlobalRadar(observation networktarget.NetworkTelemetryObservation) (GlobalRadarObservation, error) {
	if observation.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion {
		return GlobalRadarObservation{}, errors.New("network telemetry schema mismatch")
	}
	network, ok := networktarget.LookupNetwork(observation.Network.ID)
	if !ok || network.ID != observation.Network.ID ||
		network.Family != observation.Network.Family {
		return GlobalRadarObservation{}, errors.New("registered network telemetry is required")
	}
	if observation.ObservedAt.IsZero() || strings.TrimSpace(observation.Source) == "" ||
		strings.TrimSpace(observation.SubjectID) == "" {
		return GlobalRadarObservation{}, errors.New("concrete network telemetry is required")
	}
	if observation.EvidenceStatus != IntelligenceEvidenceObserved &&
		observation.EvidenceStatus != IntelligenceEvidenceVerified {
		return GlobalRadarObservation{}, errors.New("observed or verified network telemetry is required")
	}

	chainFamily := strings.ToLower(strings.TrimSpace(network.Family))
	chain := universalChainName(network.ID, chainFamily)
	subjectKind := strings.ToLower(strings.TrimSpace(observation.SubjectKind))
	switch subjectKind {
	case "network", "node", "validator", "miner":
	default:
		return GlobalRadarObservation{}, errors.New("unsupported network telemetry subject kind")
	}

	rawSubject := "telemetry:" + subjectKind + ":" + strings.TrimSpace(observation.SubjectID)
	canonical := intelligenceCanonicalRef(chainFamily, chain, network.ID, rawSubject)
	subject := IntelligenceSubject{
		ID:                  intelligenceStableID(canonical),
		Raw:                 rawSubject,
		CanonicalRef:        canonical,
		ChainFamily:         chainFamily,
		Chain:               chain,
		Network:             network.ID,
		Kind:                subjectKind,
		ClassificationBasis: "network_telemetry_subject",
	}

	attributes := map[string]any{
		"telemetry_subject_kind": subjectKind,
		"telemetry_subject_id":   observation.SubjectID,
		"consensus_family":       network.ConsensusFamily,
		"missing_evidence":       append([]string(nil), observation.MissingEvidence...),
		"missing_is_risk":        false,
	}
	if observation.ClientFamily != "" {
		attributes["client_family"] = observation.ClientFamily
	}
	if observation.StakeSharePct != nil {
		attributes["stake_share_pct"] = *observation.StakeSharePct
	}
	if observation.HashSharePct != nil {
		attributes["hash_share_pct"] = *observation.HashSharePct
	}
	if observation.ASN != "" {
		attributes["asn"] = observation.ASN
	}
	if observation.CountryCode != "" {
		attributes["country_code"] = observation.CountryCode
		attributes["location_source"] = observation.LocationSource
	}

	evidenceIdentity := strings.Join([]string{
		"global-radar-network-telemetry",
		network.ID,
		subjectKind,
		observation.SubjectID,
		observation.Source,
		observation.ObservedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}, ":")
	confidence := 0.8
	if observation.EvidenceStatus == IntelligenceEvidenceVerified {
		confidence = 1
	}
	evidence := IntelligenceEvidence{
		ID:          intelligenceStableID(evidenceIdentity),
		SubjectID:   subject.ID,
		ChainFamily: chainFamily,
		Chain:       chain,
		Network:     network.ID,
		Source:      observation.Source,
		Status:      observation.EvidenceStatus,
		ObservedAt:  observation.ObservedAt.UTC(),
		Method:      "network_telemetry:" + subjectKind,
		StateChange: "telemetry_observed",
		Provenance:  networktarget.NetworkTelemetrySchemaVersion,
		Confidence:  confidence,
		Attributes:  attributes,
	}
	return BuildGlobalRadarObservation(GlobalRadarObservationNode, subject, evidence)
}
