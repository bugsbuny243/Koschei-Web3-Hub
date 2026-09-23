package services

import (
	"errors"
	"strings"
	"time"
)

const GlobalRadarSnapshotSchemaVersion = "koschei.global-radar-snapshot.v1"

type GlobalRadarSnapshotCoverage struct {
	NetworkCount              int  `json:"network_count"`
	SubjectCount              int  `json:"subject_count"`
	ObservationCount          int  `json:"observation_count"`
	ObservedEvidenceCount     int  `json:"observed_evidence_count"`
	VerifiedEvidenceCount     int  `json:"verified_evidence_count"`
	RelationCount             int  `json:"relation_count"`
	VerifiedRelationCount     int  `json:"verified_relation_count"`
	CrossNetworkRelationCount int  `json:"cross_network_relation_count"`
	BridgeLinkCount           int  `json:"bridge_link_count"`
	MissingEvidenceItemCount  int  `json:"missing_evidence_item_count"`
	RiskScoreProduced         bool `json:"risk_score_produced"`
}

type GlobalRadarSnapshot struct {
	SchemaVersion string                      `json:"schema_version"`
	GeneratedAt   time.Time                   `json:"generated_at"`
	Observations  []GlobalRadarObservation    `json:"observations"`
	Relations     []GlobalRadarRelationEdge   `json:"relations,omitempty"`
	BridgeLinks   []GlobalRadarBridgeLink     `json:"bridge_links,omitempty"`
	Coverage      GlobalRadarSnapshotCoverage `json:"coverage"`
}

// BuildGlobalRadarSnapshot assembles a self-consistent machine-readable radar
// snapshot. Coverage is descriptive only. It never becomes a risk score.
func BuildGlobalRadarSnapshot(
	observations []GlobalRadarObservation,
	relations []GlobalRadarRelationEdge,
	bridgeLinks []GlobalRadarBridgeLink,
	generatedAt time.Time,
) (GlobalRadarSnapshot, error) {
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	networkIDs := map[string]bool{}
	subjectIDs := map[string]bool{}
	evidenceIDs := map[string]bool{}
	observationIDs := map[string]bool{}
	coverage := GlobalRadarSnapshotCoverage{RiskScoreProduced: false}

	for _, observation := range observations {
		if observation.SchemaVersion != GlobalRadarObservationSchemaVersion ||
			strings.TrimSpace(observation.ObservationID) == "" ||
			observation.DecisionState != "evidence_only_no_verdict_created" {
			return GlobalRadarSnapshot{}, errors.New("invalid global radar observation")
		}
		if observationIDs[observation.ObservationID] {
			return GlobalRadarSnapshot{}, errors.New("duplicate global radar observation")
		}
		observationIDs[observation.ObservationID] = true
		if observation.Subject.ID == "" || observation.Evidence.ID == "" ||
			observation.Evidence.SubjectID != observation.Subject.ID {
			return GlobalRadarSnapshot{}, errors.New("global radar observation identity mismatch")
		}
		networkIDs[observation.Subject.Network] = true
		subjectIDs[observation.Subject.ID] = true
		evidenceIDs[observation.Evidence.ID] = true
		switch observation.Evidence.Status {
		case IntelligenceEvidenceObserved:
			coverage.ObservedEvidenceCount++
		case IntelligenceEvidenceVerified:
			coverage.VerifiedEvidenceCount++
		default:
			return GlobalRadarSnapshot{}, errors.New("snapshot contains unsupported evidence status")
		}
		coverage.MissingEvidenceItemCount += globalRadarMissingEvidenceCount(observation.Evidence.Attributes)
	}

	bridgeIDs := map[string]bool{}
	for _, link := range bridgeLinks {
		if link.SchemaVersion != GlobalRadarBridgeLinkSchemaVersion ||
			strings.TrimSpace(link.LinkID) == "" ||
			link.Relation.Status != IntelligenceEvidenceVerified ||
			!link.Relation.CrossNetwork {
			return GlobalRadarSnapshot{}, errors.New("invalid global radar bridge link")
		}
		if bridgeIDs[link.LinkID] {
			return GlobalRadarSnapshot{}, errors.New("duplicate global radar bridge link")
		}
		bridgeIDs[link.LinkID] = true
		if !subjectIDs[link.SourceObservation.Subject.ID] ||
			!subjectIDs[link.DestinationObservation.Subject.ID] {
			return GlobalRadarSnapshot{}, errors.New("bridge link endpoints are missing from snapshot observations")
		}
		evidenceIDs[link.LinkEvidence.ID] = true
	}

	relationIDs := map[string]bool{}
	for _, relation := range relations {
		if relation.SchemaVersion != GlobalRadarRelationSchemaVersion ||
			strings.TrimSpace(relation.EdgeID) == "" ||
			relationIDs[relation.EdgeID] {
			return GlobalRadarSnapshot{}, errors.New("invalid or duplicate global radar relation")
		}
		relationIDs[relation.EdgeID] = true
		if !subjectIDs[relation.Source.ID] || !subjectIDs[relation.Target.ID] {
			return GlobalRadarSnapshot{}, errors.New("relation endpoints are missing from snapshot observations")
		}
		if relation.CrossNetwork != !strings.EqualFold(relation.Source.Network, relation.Target.Network) {
			return GlobalRadarSnapshot{}, errors.New("relation cross-network flag mismatch")
		}
		if relation.Status == IntelligenceEvidenceVerified {
			refs := nonEmptyIntelligenceRefs(relation.EvidenceRefs)
			if len(refs) == 0 {
				return GlobalRadarSnapshot{}, errors.New("verified relation is missing evidence")
			}
			for _, ref := range refs {
				if !evidenceIDs[ref] {
					return GlobalRadarSnapshot{}, errors.New("verified relation references evidence outside snapshot")
				}
			}
			coverage.VerifiedRelationCount++
		} else if relation.Status != IntelligenceEvidenceUnverified {
			return GlobalRadarSnapshot{}, errors.New("unsupported relation status")
		}
		if relation.CrossNetwork {
			coverage.CrossNetworkRelationCount++
		}
	}

	coverage.NetworkCount = len(networkIDs)
	coverage.SubjectCount = len(subjectIDs)
	coverage.ObservationCount = len(observations)
	coverage.RelationCount = len(relations)
	coverage.BridgeLinkCount = len(bridgeLinks)

	return GlobalRadarSnapshot{
		SchemaVersion: GlobalRadarSnapshotSchemaVersion,
		GeneratedAt:   generatedAt.UTC(),
		Observations:  append([]GlobalRadarObservation(nil), observations...),
		Relations:     append([]GlobalRadarRelationEdge(nil), relations...),
		BridgeLinks:   append([]GlobalRadarBridgeLink(nil), bridgeLinks...),
		Coverage:      coverage,
	}, nil
}

func globalRadarMissingEvidenceCount(attributes map[string]any) int {
	if len(attributes) == 0 {
		return 0
	}
	raw, ok := attributes["missing_evidence"]
	if !ok {
		return 0
	}
	switch values := raw.(type) {
	case []string:
		return len(values)
	case []any:
		count := 0
		for _, value := range values {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				count++
			}
		}
		return count
	default:
		return 0
	}
}
