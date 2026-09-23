package services

import (
	"errors"
	"strings"
	"time"
)

const GlobalRadarBridgeLinkSchemaVersion = "koschei.global-radar-bridge-link.v1"

type GlobalRadarBridgeLink struct {
	SchemaVersion          string                   `json:"schema_version"`
	LinkID                 string                   `json:"link_id"`
	BridgeProtocol         string                   `json:"bridge_protocol"`
	BridgeTransferID       string                   `json:"bridge_transfer_id"`
	SourceObservation      GlobalRadarObservation   `json:"source_observation"`
	DestinationObservation GlobalRadarObservation   `json:"destination_observation"`
	LinkEvidence           IntelligenceEvidence     `json:"link_evidence"`
	Relation               GlobalRadarRelationEdge  `json:"relation"`
	VerificationBoundary   string                   `json:"verification_boundary"`
}

// BuildGlobalRadarBridgeLink correlates two already-observed chain transactions.
// It does not infer bridge use from timing, address similarity, token symbols, or
// amounts. Both chain observations must independently carry the same explicit
// bridge protocol and transfer identifier in their evidence attributes.
func BuildGlobalRadarBridgeLink(source, destination GlobalRadarObservation) (GlobalRadarBridgeLink, error) {
	if source.SchemaVersion != GlobalRadarObservationSchemaVersion ||
		destination.SchemaVersion != GlobalRadarObservationSchemaVersion {
		return GlobalRadarBridgeLink{}, errors.New("global radar observations are required")
	}
	if source.ObservationKind != GlobalRadarObservationTransaction ||
		destination.ObservationKind != GlobalRadarObservationTransaction {
		return GlobalRadarBridgeLink{}, errors.New("bridge link requires transaction observations")
	}
	if source.ObservationID == "" || destination.ObservationID == "" ||
		source.ObservationID == destination.ObservationID {
		return GlobalRadarBridgeLink{}, errors.New("distinct bridge observations are required")
	}
	if strings.EqualFold(strings.TrimSpace(source.Subject.Network), strings.TrimSpace(destination.Subject.Network)) {
		return GlobalRadarBridgeLink{}, errors.New("bridge link requires distinct source and destination networks")
	}
	if strings.TrimSpace(source.Evidence.TransactionHash) == "" ||
		strings.TrimSpace(destination.Evidence.TransactionHash) == "" {
		return GlobalRadarBridgeLink{}, errors.New("bridge link requires source and destination transaction hashes")
	}
	if !globalRadarBridgeEvidenceStatusAllowed(source.Evidence.Status) ||
		!globalRadarBridgeEvidenceStatusAllowed(destination.Evidence.Status) {
		return GlobalRadarBridgeLink{}, errors.New("bridge link requires observed or verified endpoint evidence")
	}

	sourceProtocol := globalRadarEvidenceAttributeString(source.Evidence.Attributes, "bridge_protocol")
	destinationProtocol := globalRadarEvidenceAttributeString(destination.Evidence.Attributes, "bridge_protocol")
	sourceTransferID := globalRadarEvidenceAttributeString(source.Evidence.Attributes, "bridge_transfer_id")
	destinationTransferID := globalRadarEvidenceAttributeString(destination.Evidence.Attributes, "bridge_transfer_id")
	if sourceProtocol == "" || sourceTransferID == "" ||
		!strings.EqualFold(sourceProtocol, destinationProtocol) ||
		sourceTransferID != destinationTransferID {
		return GlobalRadarBridgeLink{}, errors.New("matching explicit bridge protocol and transfer identity are required")
	}

	observedAt := destination.ObservedAt
	if source.ObservedAt.After(observedAt) {
		observedAt = source.ObservedAt
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	observedAt = observedAt.UTC()

	linkIdentity := strings.Join([]string{
		"global-radar-bridge-link",
		strings.ToLower(sourceProtocol),
		sourceTransferID,
		source.Subject.Network,
		source.Evidence.TransactionHash,
		destination.Subject.Network,
		destination.Evidence.TransactionHash,
	}, ":")
	linkID := intelligenceStableID(linkIdentity)

	linkEvidence := IntelligenceEvidence{
		ID:              intelligenceStableID("evidence:" + linkIdentity),
		SubjectID:       source.Subject.ID,
		ChainFamily:     source.Subject.ChainFamily,
		Chain:           source.Subject.Chain,
		Network:         source.Subject.Network,
		Source:          "global_radar_bridge_link",
		Status:          IntelligenceEvidenceVerified,
		TransactionHash: source.Evidence.TransactionHash,
		ObservedAt:      observedAt,
		Address:         source.Subject.Raw,
		Method:          "cross_network_bridge_transfer_correlation",
		StateChange:     "bridge_transfer_link_verified",
		Provenance:      GlobalRadarBridgeLinkSchemaVersion,
		Confidence:      1,
		Attributes: map[string]any{
			"bridge_protocol":              sourceProtocol,
			"bridge_transfer_id":           sourceTransferID,
			"source_subject_id":            source.Subject.ID,
			"target_subject_id":            destination.Subject.ID,
			"source_network":               source.Subject.Network,
			"target_network":               destination.Subject.Network,
			"source_transaction_hash":      source.Evidence.TransactionHash,
			"destination_transaction_hash": destination.Evidence.TransactionHash,
			"source_evidence_id":           source.Evidence.ID,
			"destination_evidence_id":      destination.Evidence.ID,
			"source_observation_id":        source.ObservationID,
			"destination_observation_id":   destination.ObservationID,
			"identity_claim":               false,
			"intent_claim":                 false,
		},
	}

	relation, err := BuildGlobalRadarRelationEdge(
		source.Subject,
		destination.Subject,
		"bridge_transfer",
		linkEvidence,
	)
	if err != nil {
		return GlobalRadarBridgeLink{}, err
	}
	if relation.Status != IntelligenceEvidenceVerified {
		return GlobalRadarBridgeLink{}, errors.New("bridge relation did not satisfy explicit link evidence policy")
	}

	return GlobalRadarBridgeLink{
		SchemaVersion:          GlobalRadarBridgeLinkSchemaVersion,
		LinkID:                 linkID,
		BridgeProtocol:         sourceProtocol,
		BridgeTransferID:       sourceTransferID,
		SourceObservation:      source,
		DestinationObservation: destination,
		LinkEvidence:           linkEvidence,
		Relation:               relation,
		VerificationBoundary:   "transfer_link_only_no_actor_identity_or_intent_claim",
	}, nil
}

func globalRadarBridgeEvidenceStatusAllowed(status string) bool {
	return status == IntelligenceEvidenceObserved || status == IntelligenceEvidenceVerified
}
