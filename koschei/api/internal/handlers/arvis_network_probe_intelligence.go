package handlers

import (
	"strings"

	"koschei/api/internal/services"
)

// attachArvisNetworkProbeIntelligence adds already-normalized, read-only network
// probe evidence to an ARVIS intelligence investigation. The projection is
// evidence-only: it never upgrades the investigation decision or creates
// authorization, ownership, trust, or safety claims.
func attachArvisNetworkProbeIntelligence(investigation *services.IntelligenceInvestigation, projection services.NetworkProbeIntelligenceProjection) bool {
	if investigation == nil {
		return false
	}
	subject := projection.Subject
	evidence := projection.Evidence
	if strings.TrimSpace(subject.ID) == "" || strings.TrimSpace(evidence.ID) == "" || evidence.SubjectID != subject.ID {
		return false
	}
	if evidence.Status != services.IntelligenceEvidenceObserved || strings.TrimSpace(evidence.Provenance) != "networktarget_read_only_probe" {
		return false
	}
	if evidence.ChainFamily != subject.ChainFamily || evidence.Chain != subject.Chain || evidence.Network != subject.Network {
		return false
	}
	if subject.ChainFamily != services.IntelligenceChainFamilyEVM && subject.ChainFamily != services.IntelligenceChainFamilyUTXO {
		return false
	}
	for _, existing := range investigation.Subjects {
		if existing.ID == subject.ID {
			appendArvisProbeEvidenceIfMissing(investigation, evidence)
			return true
		}
	}
	investigation.Subjects = append(investigation.Subjects, subject)
	appendArvisProbeEvidenceIfMissing(investigation, evidence)
	return true
}

func appendArvisProbeEvidenceIfMissing(investigation *services.IntelligenceInvestigation, evidence services.IntelligenceEvidence) {
	for _, existing := range investigation.Evidence {
		if existing.ID == evidence.ID {
			return
		}
	}
	investigation.Evidence = append(investigation.Evidence, evidence)
}
