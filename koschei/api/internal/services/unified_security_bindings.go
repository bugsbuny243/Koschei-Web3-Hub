package services

import "strings"

type UnifiedSecurityBindingIssue struct {
	ObjectID string `json:"object_id"`
	Code     string `json:"code"`
}

// FinalizeUnifiedSecurityInvestigation checks references in a completed adapter
// projection. Consistency is not producer authentication or permission to act.
// Any broken binding makes the additive claims unverified; the authoritative
// base evidence and decision are preserved without mutation or re-grading.
func FinalizeUnifiedSecurityInvestigation(in UnifiedSecurityInvestigation) UnifiedSecurityInvestigation {
	out := in
	out.BindingStatus = "consistent"
	out.BindingIssues = nil
	issue := func(id, code string) {
		out.BindingIssues = append(out.BindingIssues, UnifiedSecurityBindingIssue{ObjectID: id, Code: code})
	}
	if in.ContractVersion != UnifiedSecurityContractVersion || in.Base.ContractVersion != IntelligenceContractVersion {
		issue("", "contract_version_mismatch")
	}
	subjects := unifiedBindingIndex(in.Base.Subjects, func(v IntelligenceSubject) string { return v.ID }, issue)
	evidence := unifiedBindingIndex(in.Base.Evidence, func(v IntelligenceEvidence) string { return v.ID }, issue)
	capabilities := unifiedBindingIndex(in.Capabilities, func(v IntelligenceCapability) string { return v.ID }, issue)
	actions := unifiedBindingIndex(in.Actions, func(v IntelligenceAction) string { return v.ID }, issue)
	boundaries := unifiedBindingIndex(in.BoundaryTransitions, func(v IntelligenceTrustBoundaryTransition) string { return v.ID }, issue)
	unifiedBindingIndex(in.AttackPaths, func(v UnifiedSecurityAttackPath) string { return v.ID }, issue)

	subject := func(id, owner string, optional bool) {
		if id == "" && optional {
			return
		}
		if _, ok := subjects[id]; !ok {
			issue(owner, "subject_reference_missing")
		}
	}
	refs := func(id, status string, references, participants, resources []string) {
		rank := unifiedBindingEvidenceRank(status)
		if rank < 0 {
			issue(id, "evidence_status_invalid")
		}
		if rank > 0 && len(references) == 0 {
			issue(id, "evidence_reference_missing")
		}
		for _, ref := range references {
			e, ok := evidence[ref]
			if !ok {
				issue(id, "evidence_reference_missing")
				continue
			}
			s, ok := subjects[e.SubjectID]
			if !ok {
				issue(id, "evidence_subject_missing")
				continue
			}
			if e.Network != s.Network || e.Chain != s.Chain || e.ChainFamily != s.ChainFamily {
				issue(id, "evidence_network_mismatch")
			}
			resourceMatch := unifiedBindingContains(resources, s.CanonicalRef)
			if unifiedBindingContains(resources, s.Raw) && s.Network != "" && s.Network != "unknown" {
				for _, participant := range participants {
					owner, exists := subjects[participant]
					if exists && owner.Network == s.Network && owner.Chain == s.Chain && owner.ChainFamily == s.ChainFamily {
						resourceMatch = true
					}
				}
			}
			if !unifiedBindingContains(participants, s.ID) && !resourceMatch {
				issue(id, "evidence_subject_mismatch")
			}
			if unifiedBindingEvidenceRank(e.Status) < rank {
				issue(id, "evidence_strength_insufficient")
			}
			if rank > 0 && (strings.TrimSpace(e.Source) == "" || strings.TrimSpace(e.Provenance) == "") {
				issue(id, "evidence_provenance_missing")
			}
		}
	}
	capability := func(id, actor, owner, status string) IntelligenceCapability {
		if id == "" {
			return IntelligenceCapability{}
		}
		c, ok := capabilities[id]
		if !ok {
			issue(owner, "capability_reference_missing")
		} else if c.SubjectID != actor {
			issue(owner, "capability_subject_mismatch")
		} else if unifiedBindingEvidenceRank(c.Status) < unifiedBindingEvidenceRank(status) {
			issue(owner, "capability_strength_insufficient")
		}
		return c
	}
	for _, c := range in.Capabilities {
		subject(c.SubjectID, c.ID, false)
		subject(c.DelegatedBySubjectID, c.ID, true)
		refs(c.ID, c.Status, c.EvidenceRefs, []string{c.SubjectID, c.DelegatedBySubjectID}, []string{c.ResourceRef})
	}
	for _, a := range in.Actions {
		subject(a.SubjectID, a.ID, false)
		subject(a.TargetSubjectID, a.ID, true)
		c := capability(a.CapabilityID, a.SubjectID, a.ID, a.Status)
		if a.CapabilityID != "" && c.Domain != a.Domain {
			issue(a.ID, "capability_domain_mismatch")
		}
		if a.CapabilityID != "" && c.ResourceRef != "" && c.ResourceRef != a.ResourceRef {
			issue(a.ID, "capability_resource_mismatch")
		}
		refs(a.ID, a.Status, a.EvidenceRefs, []string{a.SubjectID, a.TargetSubjectID}, []string{a.ResourceRef})
		if a.TransactionHash != "" {
			matched := false
			for _, ref := range a.EvidenceRefs {
				e := evidence[ref]
				if e.TransactionHash == a.TransactionHash {
					matched = true
				} else if e.TransactionHash != "" {
					issue(a.ID, "action_transaction_mismatch")
				}
			}
			if !matched && unifiedBindingEvidenceRank(a.Status) > 0 {
				issue(a.ID, "action_transaction_evidence_missing")
			}
		}
	}
	for _, b := range in.BoundaryTransitions {
		subject(b.SubjectID, b.ID, false)
		c := capability(b.CapabilityID, b.SubjectID, b.ID, b.Status)
		participants, resources := []string{b.SubjectID}, []string{c.ResourceRef}
		if b.CapabilityID != "" && c.Domain != b.FromDomain {
			issue(b.ID, "boundary_capability_domain_mismatch")
		}
		if b.ActionID != "" {
			a, ok := actions[b.ActionID]
			if !ok {
				issue(b.ID, "action_reference_missing")
			} else {
				if a.SubjectID != b.SubjectID || (b.CapabilityID != "" && a.CapabilityID != b.CapabilityID) {
					issue(b.ID, "boundary_action_mismatch")
				}
				if a.Domain != b.FromDomain && a.Domain != b.ToDomain {
					issue(b.ID, "boundary_action_domain_mismatch")
				}
				if unifiedBindingEvidenceRank(a.Status) < unifiedBindingEvidenceRank(b.Status) {
					issue(b.ID, "action_strength_insufficient")
				}
				participants = append(participants, a.TargetSubjectID)
				resources = append(resources, a.ResourceRef)
			}
		}
		refs(b.ID, b.Status, b.EvidenceRefs, participants, resources)
	}
	for _, p := range in.AttackPaths {
		if p.ContractVersion != UnifiedSecurityContractVersion {
			issue(p.ID, "contract_version_mismatch")
		}
		unifiedBindingIndex(p.Consequences, func(v IntelligenceConsequence) string { return v.ID }, issue)
		subject(p.EntrySubjectID, p.ID, false)
		if len(p.Steps) == 0 || p.Steps[0].SubjectID != p.EntrySubjectID {
			issue(p.ID, "path_entry_mismatch")
		}
		participants, resources := []string{p.EntrySubjectID}, []string{}
		for i, step := range p.Steps {
			subject(step.SubjectID, p.ID, false)
			subject(step.TargetSubjectID, p.ID, true)
			if step.Order != i+1 {
				issue(p.ID, "path_step_order_invalid")
			}
			c := capability(step.CapabilityID, step.SubjectID, p.ID, p.Status)
			stepParticipants := []string{step.SubjectID, step.TargetSubjectID}
			stepResources := []string{c.ResourceRef}
			if step.ActionID != "" {
				a, ok := actions[step.ActionID]
				if !ok {
					issue(p.ID, "action_reference_missing")
				} else {
					if a.SubjectID != step.SubjectID || (step.TargetSubjectID != "" && a.TargetSubjectID != step.TargetSubjectID) || (step.CapabilityID != "" && a.CapabilityID != step.CapabilityID) {
						issue(p.ID, "path_action_mismatch")
					}
					if unifiedBindingEvidenceRank(a.Status) < unifiedBindingEvidenceRank(p.Status) {
						issue(p.ID, "action_strength_insufficient")
					}
					stepParticipants = append(stepParticipants, a.TargetSubjectID)
					stepResources = append(stepResources, a.ResourceRef)
				}
			}
			if step.BoundaryTransitionID != "" {
				b, ok := boundaries[step.BoundaryTransitionID]
				if !ok {
					issue(p.ID, "boundary_reference_missing")
				} else {
					if b.SubjectID != step.SubjectID || b.CapabilityID != step.CapabilityID || b.ActionID != step.ActionID {
						issue(p.ID, "path_boundary_mismatch")
					}
					if unifiedBindingEvidenceRank(b.Status) < unifiedBindingEvidenceRank(p.Status) {
						issue(p.ID, "boundary_strength_insufficient")
					}
				}
			}
			refs(p.ID, p.Status, step.EvidenceRefs, stepParticipants, stepResources)
			participants = append(participants, stepParticipants...)
			resources = append(resources, stepResources...)
		}
		refs(p.ID, p.Status, p.EvidenceRefs, participants, resources)
		for _, c := range p.Consequences {
			refs(c.ID, c.Status, c.EvidenceRefs, participants, resources)
			if unifiedBindingEvidenceRank(c.Status) < unifiedBindingEvidenceRank(p.Status) {
				issue(p.ID, "consequence_strength_insufficient")
			}
		}
	}
	if len(out.BindingIssues) == 0 {
		return out
	}
	out.BindingStatus = "unverified"
	out.Capabilities = append([]IntelligenceCapability(nil), in.Capabilities...)
	out.Actions = append([]IntelligenceAction(nil), in.Actions...)
	out.BoundaryTransitions = append([]IntelligenceTrustBoundaryTransition(nil), in.BoundaryTransitions...)
	out.AttackPaths = append([]UnifiedSecurityAttackPath(nil), in.AttackPaths...)
	for i := range out.Capabilities {
		out.Capabilities[i].Status, out.Capabilities[i].Confidence = IntelligenceEvidenceUnverified, 0
	}
	for i := range out.Actions {
		out.Actions[i].Status, out.Actions[i].Confidence = IntelligenceEvidenceUnverified, 0
	}
	for i := range out.BoundaryTransitions {
		out.BoundaryTransitions[i].Status, out.BoundaryTransitions[i].Confidence = IntelligenceEvidenceUnverified, 0
	}
	for i := range out.AttackPaths {
		p := &out.AttackPaths[i]
		p.Status, p.Confidence = IntelligenceEvidenceUnverified, 0
		p.Consequences = append([]IntelligenceConsequence(nil), p.Consequences...)
		for j := range p.Consequences {
			p.Consequences[j].Status, p.Consequences[j].Confidence = IntelligenceEvidenceUnverified, 0
		}
	}
	return out
}

func unifiedBindingIndex[T any](items []T, id func(T) string, issue func(string, string)) map[string]T {
	out := make(map[string]T, len(items))
	for _, item := range items {
		key := id(item)
		if key == "" || key != strings.TrimSpace(key) {
			issue(key, "object_id_invalid")
		}
		if _, exists := out[key]; exists {
			issue(key, "object_id_ambiguous")
		}
		out[key] = item
	}
	return out
}

func unifiedBindingContains(values []string, target string) bool {
	if target == "" {
		return false
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func unifiedBindingEvidenceRank(status string) int {
	switch status {
	case IntelligenceEvidenceVerified:
		return 3
	case IntelligenceEvidenceObserved:
		return 2
	case IntelligenceEvidenceInferred:
		return 1
	case IntelligenceEvidenceUnverified:
		return 0
	default:
		return -1
	}
}
