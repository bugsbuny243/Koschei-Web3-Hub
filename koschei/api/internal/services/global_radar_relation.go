package services

import (
	"errors"
	"strings"
)

const GlobalRadarRelationSchemaVersion = "koschei.global-radar-relation.v1"

type GlobalRadarRelationEdge struct {
	SchemaVersion       string              `json:"schema_version"`
	EdgeID              string              `json:"edge_id"`
	Source              IntelligenceSubject `json:"source"`
	Target              IntelligenceSubject `json:"target"`
	Relation            string              `json:"relation"`
	Status              string              `json:"status"`
	CrossNetwork        bool                `json:"cross_network"`
	EvidenceRefs        []string            `json:"evidence_refs,omitempty"`
	VerificationPolicy  string              `json:"verification_policy"`
}

// BuildGlobalRadarRelationEdge creates a durable graph edge without inferring
// identity or intent. A relation becomes VERIFIED only when one concrete
// evidence record explicitly binds both endpoint subject IDs.
//
// Cross-network relations require that the same binding evidence also names the
// source and target networks. Two independent observations on different chains
// are not enough to prove that the subjects are related.
func BuildGlobalRadarRelationEdge(source, target IntelligenceSubject, relation string, linkEvidence IntelligenceEvidence) (GlobalRadarRelationEdge, error) {
	relation = strings.ToLower(strings.TrimSpace(relation))
	if strings.TrimSpace(source.ID) == "" || strings.TrimSpace(target.ID) == "" ||
		strings.TrimSpace(source.Network) == "" || strings.TrimSpace(target.Network) == "" ||
		strings.TrimSpace(source.CanonicalRef) == "" || strings.TrimSpace(target.CanonicalRef) == "" {
		return GlobalRadarRelationEdge{}, errors.New("canonical relation endpoints are required")
	}
	if source.ID == target.ID {
		return GlobalRadarRelationEdge{}, errors.New("global radar relation endpoints must be distinct")
	}
	if relation == "" {
		return GlobalRadarRelationEdge{}, errors.New("global radar relation type is required")
	}

	crossNetwork := !strings.EqualFold(strings.TrimSpace(source.Network), strings.TrimSpace(target.Network))
	edge := GlobalRadarRelationEdge{
		SchemaVersion:      GlobalRadarRelationSchemaVersion,
		EdgeID:             intelligenceStableID(strings.Join([]string{"global-radar-edge", source.ID, relation, target.ID}, ":")),
		Source:             source,
		Target:             target,
		Relation:           relation,
		Status:             IntelligenceEvidenceUnverified,
		CrossNetwork:       crossNetwork,
		VerificationPolicy: "explicit_link_evidence_required",
	}

	if strings.TrimSpace(linkEvidence.ID) == "" ||
		(linkEvidence.Status != IntelligenceEvidenceObserved && linkEvidence.Status != IntelligenceEvidenceVerified) {
		return edge, nil
	}

	sourceID := globalRadarEvidenceAttributeString(linkEvidence.Attributes, "source_subject_id")
	targetID := globalRadarEvidenceAttributeString(linkEvidence.Attributes, "target_subject_id")
	if sourceID != source.ID || targetID != target.ID {
		return edge, nil
	}

	if crossNetwork {
		sourceNetwork := globalRadarEvidenceAttributeString(linkEvidence.Attributes, "source_network")
		targetNetwork := globalRadarEvidenceAttributeString(linkEvidence.Attributes, "target_network")
		if !strings.EqualFold(sourceNetwork, source.Network) || !strings.EqualFold(targetNetwork, target.Network) {
			return edge, nil
		}
	}

	edge.Status = IntelligenceEvidenceVerified
	edge.EvidenceRefs = []string{linkEvidence.ID}
	return edge, nil
}

func globalRadarEvidenceAttributeString(attributes map[string]any, key string) string {
	if len(attributes) == 0 {
		return ""
	}
	value, ok := attributes[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
