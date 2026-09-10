package services

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	HistoricalIntelligenceBindingVersion = "koschei-historical-intelligence-binding-v1"

	historicalClickHouseStreamSource  = "clickhouse_arvis_stream_memory"
	historicalClickHouseVerdictSource = "clickhouse_arvis_verdict_memory"
)

type HistoricalIntelligenceBindingReceipt struct {
	ContractVersion string   `json:"contract_version"`
	Status          string   `json:"status"`
	BoundEvidence   int      `json:"bound_evidence"`
	ExactDuplicates int      `json:"exact_duplicates"`
	SourceEvidence  int      `json:"source_evidence"`
	Limitations     []string `json:"limitations,omitempty"`
}

// BindClickHouseHistoricalEvidence appends only bounded historical evidence to
// an already-built live IntelligenceInvestigation. It never imports historical
// entities, relationships, behavior, hypotheses, attack paths, or decisions.
//
// The operation is atomic: all subject/evidence constraints are validated on a
// copy first. On any conflict the live investigation is left unchanged.
func BindClickHouseHistoricalEvidence(live *IntelligenceInvestigation, historical IntelligenceInvestigation) (HistoricalIntelligenceBindingReceipt, error) {
	receipt := HistoricalIntelligenceBindingReceipt{
		ContractVersion: HistoricalIntelligenceBindingVersion,
		Status:          "rejected",
		SourceEvidence:  len(historical.Evidence),
		Limitations: []string{
			"Historical ClickHouse evidence is context only; fresh evidence and the existing live decision remain authoritative.",
			"Historical evidence alone cannot create entities, relationships, behavior findings, hypotheses, attack paths, or a customer decision.",
		},
	}
	if live == nil {
		return receipt, fmt.Errorf("historical intelligence binding requires a live investigation")
	}
	if err := validateHistoricalBindingEnvelope(*live, historical); err != nil {
		return receipt, err
	}

	candidate := cloneIntelligenceInvestigation(*live)
	liveSubjects := make(map[string]IntelligenceSubject, len(candidate.Subjects))
	for _, subject := range candidate.Subjects {
		id := strings.TrimSpace(subject.ID)
		if id == "" {
			return receipt, fmt.Errorf("live intelligence contains a subject without id")
		}
		if existing, exists := liveSubjects[id]; exists && existing != subject {
			return receipt, fmt.Errorf("live intelligence subject id %s is ambiguous", id)
		}
		liveSubjects[id] = subject
	}

	historicalSubjects := make(map[string]IntelligenceSubject, len(historical.Subjects))
	for _, subject := range historical.Subjects {
		id := strings.TrimSpace(subject.ID)
		if id == "" {
			return receipt, fmt.Errorf("historical intelligence contains a subject without id")
		}
		liveSubject, exists := liveSubjects[id]
		if !exists || liveSubject != subject {
			return receipt, fmt.Errorf("historical intelligence subject %s does not exactly match the live subject boundary", id)
		}
		if existing, exists := historicalSubjects[id]; exists && existing != subject {
			return receipt, fmt.Errorf("historical intelligence subject id %s is ambiguous", id)
		}
		historicalSubjects[id] = subject
	}
	if len(historicalSubjects) == 0 && len(historical.Evidence) > 0 {
		return receipt, fmt.Errorf("historical intelligence evidence has no subject boundary")
	}

	byEvidenceID := make(map[string]IntelligenceEvidence, len(candidate.Evidence)+len(historical.Evidence))
	for _, evidence := range candidate.Evidence {
		id := strings.TrimSpace(evidence.ID)
		if id == "" {
			return receipt, fmt.Errorf("live intelligence contains evidence without id")
		}
		if existing, exists := byEvidenceID[id]; exists && !reflect.DeepEqual(existing, evidence) {
			return receipt, fmt.Errorf("live intelligence evidence id %s is conflicting", id)
		}
		byEvidenceID[id] = evidence
	}

	for _, evidence := range historical.Evidence {
		if err := validateClickHouseHistoricalEvidence(evidence, historicalSubjects); err != nil {
			return receipt, err
		}
		id := strings.TrimSpace(evidence.ID)
		if existing, exists := byEvidenceID[id]; exists {
			if !reflect.DeepEqual(existing, evidence) {
				return receipt, fmt.Errorf("historical intelligence evidence id %s conflicts with live evidence", id)
			}
			receipt.ExactDuplicates++
			continue
		}
		candidate.Evidence = append(candidate.Evidence, cloneIntelligenceEvidence(evidence))
		byEvidenceID[id] = evidence
		receipt.BoundEvidence++
	}

	*live = candidate
	receipt.Status = "bound"
	if receipt.BoundEvidence == 0 {
		receipt.Status = "no_new_evidence"
	}
	return receipt, nil
}

func validateHistoricalBindingEnvelope(live, historical IntelligenceInvestigation) error {
	if strings.TrimSpace(live.ContractVersion) != IntelligenceContractVersion {
		return fmt.Errorf("live intelligence contract version %q is unsupported", live.ContractVersion)
	}
	if strings.TrimSpace(historical.ContractVersion) != IntelligenceContractVersion {
		return fmt.Errorf("historical intelligence contract version %q is unsupported", historical.ContractVersion)
	}
	if len(historical.Entities) != 0 || len(historical.Relationships) != 0 || len(historical.Behaviors) != 0 || len(historical.Hypotheses) != 0 || len(historical.AttackPaths) != 0 {
		return fmt.Errorf("historical intelligence contains higher-order claims that cannot be bound as memory evidence")
	}
	if historical.Decision.Status != IntelligenceEvidenceUnverified || strings.TrimSpace(historical.Decision.Action) != "investigate" {
		return fmt.Errorf("historical intelligence contains a decision authority that cannot be imported")
	}
	return nil
}

func validateClickHouseHistoricalEvidence(evidence IntelligenceEvidence, subjects map[string]IntelligenceSubject) error {
	id := strings.TrimSpace(evidence.ID)
	if id == "" {
		return fmt.Errorf("historical intelligence contains evidence without id")
	}
	subject, exists := subjects[strings.TrimSpace(evidence.SubjectID)]
	if !exists {
		return fmt.Errorf("historical evidence %s references an unbound subject", id)
	}
	if evidence.ChainFamily != subject.ChainFamily || evidence.Chain != subject.Chain || evidence.Network != subject.Network {
		return fmt.Errorf("historical evidence %s escaped the exact subject chain boundary", id)
	}
	if evidence.Status != IntelligenceEvidenceObserved {
		return fmt.Errorf("historical evidence %s must remain observed", id)
	}
	if evidence.Confidence < 0 || evidence.Confidence > 1 {
		return fmt.Errorf("historical evidence %s has invalid confidence", id)
	}

	source := strings.TrimSpace(evidence.Source)
	provenance := strings.TrimSpace(evidence.Provenance)
	switch source {
	case historicalClickHouseStreamSource:
		if provenance != "existing_arvis_clickhouse_stream_shadow" {
			return fmt.Errorf("historical stream evidence %s has invalid provenance", id)
		}
	case historicalClickHouseVerdictSource:
		if provenance != "existing_arvis_clickhouse_verdict_shadow" {
			return fmt.Errorf("historical verdict evidence %s has invalid provenance", id)
		}
		if authority, _ := evidence.Attributes["memory_authority"].(string); authority != "historical_context_only" {
			return fmt.Errorf("historical verdict evidence %s is missing historical-only authority", id)
		}
		if verification, _ := evidence.Attributes["signature_verification"].(string); verification != "not_performed_by_memory_projection" {
			return fmt.Errorf("historical verdict evidence %s has an invalid signature-verification boundary", id)
		}
	default:
		return fmt.Errorf("historical evidence %s has unsupported source %q", id, source)
	}
	return nil
}

func cloneIntelligenceInvestigation(in IntelligenceInvestigation) IntelligenceInvestigation {
	out := in
	out.Subjects = append([]IntelligenceSubject(nil), in.Subjects...)
	out.Entities = append([]IntelligenceEntity(nil), in.Entities...)
	if in.Evidence != nil {
		out.Evidence = make([]IntelligenceEvidence, 0, len(in.Evidence))
		for _, evidence := range in.Evidence {
			out.Evidence = append(out.Evidence, cloneIntelligenceEvidence(evidence))
		}
	} else {
		out.Evidence = nil
	}
	out.Relationships = append([]IntelligenceRelationship(nil), in.Relationships...)
	out.Behaviors = append([]IntelligenceBehaviorFinding(nil), in.Behaviors...)
	out.Hypotheses = append([]IntelligenceThreatHypothesis(nil), in.Hypotheses...)
	out.AttackPaths = append([]IntelligenceAttackPath(nil), in.AttackPaths...)
	out.Decision.Reasons = append([]string(nil), in.Decision.Reasons...)
	out.Decision.EvidenceRefs = append([]string(nil), in.Decision.EvidenceRefs...)
	return out
}

func cloneIntelligenceEvidence(in IntelligenceEvidence) IntelligenceEvidence {
	out := in
	if in.Attributes != nil {
		out.Attributes = make(map[string]any, len(in.Attributes))
		for key, value := range in.Attributes {
			out.Attributes[key] = value
		}
	}
	return out
}
