
package services

import (
	"errors"
	"strings"
	"time"
)

type EVMTransactionGraphProjection struct {
	Investigation IntelligenceInvestigation `json:"investigation"`
}

func BuildEVMTransactionGraph(projection NetworkProbeIntelligenceProjection, generatedAt time.Time) (EVMTransactionGraphProjection, error) {
	tx := projection.Subject
	ev := projection.Evidence
	if tx.ChainFamily != IntelligenceChainFamilyEVM || tx.Kind != IntelligenceSubjectTransaction {
		return EVMTransactionGraphProjection{}, errors.New("EVM transaction subject is required")
	}
	if ev.Status != IntelligenceEvidenceObserved || ev.SubjectID != tx.ID || ev.ChainFamily != tx.ChainFamily ||
		ev.Chain != tx.Chain || ev.Network != tx.Network || strings.TrimSpace(ev.ID) == "" {
		return EVMTransactionGraphProjection{}, errors.New("bound observed EVM transaction evidence is required")
	}
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	investigation := BuildIntelligenceInvestigation([]IntelligenceSubject{tx}, generatedAt)
	investigation.Evidence = append(investigation.Evidence, ev)

	attrs := ev.Attributes
	from := stringAttribute(attrs, "from")
	if from == "" {
		from = strings.TrimSpace(ev.Address)
	}
	to := stringAttribute(attrs, "to")
	if to == "" {
		to = strings.TrimSpace(ev.Contract)
	}
	created := stringAttribute(attrs, "contract_address")

	if from != "" {
		if err := appendObservedEVMTransactionAddress(&investigation, tx, ev, from, "submitted_transaction", "transaction_sender"); err != nil {
			return EVMTransactionGraphProjection{}, err
		}
	}
	if to != "" {
		if err := appendObservedEVMTransactionAddress(&investigation, tx, ev, to, "targeted_address", "transaction_target"); err != nil {
			return EVMTransactionGraphProjection{}, err
		}
	}
	if created != "" {
		if err := appendObservedEVMTransactionAddress(&investigation, tx, ev, created, "created_contract", "created_contract"); err != nil {
			return EVMTransactionGraphProjection{}, err
		}
	}

	for _, log := range transactionLogAttributes(attrs["logs"]) {
		if removed, _ := log["removed"].(bool); removed {
			continue
		}
		address := stringAttribute(log, "address")
		if address == "" {
			continue
		}
		if err := appendObservedEVMTransactionAddress(&investigation, tx, ev, address, "emitted_log", "log_emitter"); err != nil {
			return EVMTransactionGraphProjection{}, err
		}
	}

	return EVMTransactionGraphProjection{Investigation: investigation}, nil
}

func appendObservedEVMTransactionAddress(
	investigation *IntelligenceInvestigation,
	tx IntelligenceSubject,
	ev IntelligenceEvidence,
	address, relation, entityKind string,
) error {
	subject, err := ClassifyUniversalInvestigationSubject(address, tx.Network, IntelligenceSubjectAddress)
	if err != nil || subject.ChainFamily != IntelligenceChainFamilyEVM || subject.Network != tx.Network {
		return errors.New("EVM transaction graph address is not canonical for transaction network")
	}
	appendInvestigationSubjectIfMissing(investigation, subject)

	entityID := "entity:" + subject.ID
	if !investigationHasEntity(investigation, entityID) {
		investigation.Entities = append(investigation.Entities, IntelligenceEntity{
			ID: entityID,
			Kind: entityKind,
			Label: subject.Raw,
			Attribution: "onchain_role_only",
			Confidence: 1,
			EvidenceRefs: []string{ev.ID},
		})
	}
	if !investigationHasRelationship(investigation, tx.ID, subject.ID, relation, ev.ID) {
		investigation.Relationships = append(investigation.Relationships, IntelligenceRelationship{
			SourceSubjectID: tx.ID,
			TargetSubjectID: subject.ID,
			Relation: relation,
			Status: IntelligenceEvidenceObserved,
			Confidence: 1,
			EvidenceRefs: []string{ev.ID},
		})
	}
	return nil
}

func appendInvestigationSubjectIfMissing(investigation *IntelligenceInvestigation, subject IntelligenceSubject) {
	if investigation == nil {
		return
	}
	for _, existing := range investigation.Subjects {
		if existing.ID == subject.ID {
			return
		}
	}
	investigation.Subjects = append(investigation.Subjects, subject)
}

func investigationHasEntity(investigation *IntelligenceInvestigation, id string) bool {
	if investigation == nil {
		return false
	}
	for _, entity := range investigation.Entities {
		if entity.ID == id {
			return true
		}
	}
	return false
}

func investigationHasRelationship(investigation *IntelligenceInvestigation, source, target, relation, evidence string) bool {
	if investigation == nil {
		return false
	}
	for _, item := range investigation.Relationships {
		if item.SourceSubjectID == source && item.TargetSubjectID == target && item.Relation == relation {
			for _, ref := range item.EvidenceRefs {
				if ref == evidence {
					return true
				}
			}
		}
	}
	return false
}

func stringAttribute(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	value, _ := attrs[key].(string)
	return strings.ToLower(strings.TrimSpace(value))
}

func transactionLogAttributes(value any) []map[string]any {
	switch logs := value.(type) {
	case []map[string]any:
		return logs
	case []any:
		out := make([]map[string]any, 0, len(logs))
		for _, item := range logs {
			if log, ok := item.(map[string]any); ok {
				out = append(out, log)
			}
		}
		return out
	default:
		return nil
	}
}
