package services

const (
	IntelligenceCapabilityLifecycleProspective = "prospective"
	IntelligenceCapabilityTokenAuthority       = "token_authority"
	IntelligenceCapabilityExecutionHook        = "execution_hook"
)

// BuildProspectiveIntelligenceCapability records a capability that would exist
// after a pre-signing simulation or other forward-looking validation step. It
// must not be confused with an authority that is already active on-chain.
func BuildProspectiveIntelligenceCapability(
	subjectID, domain, kind, scope, resourceRef, delegatedBySubjectID, requestedStatus string,
	constraints, evidenceRefs []string,
	confidence float64,
) IntelligenceCapability {
	capability := BuildIntelligenceCapability(
		subjectID,
		domain,
		kind,
		scope,
		resourceRef,
		delegatedBySubjectID,
		IntelligenceCapabilityLifecycleUnknown,
		requestedStatus,
		constraints,
		evidenceRefs,
		confidence,
	)
	if capability.Status != IntelligenceEvidenceUnverified {
		capability.Lifecycle = IntelligenceCapabilityLifecycleProspective
	}
	return capability
}
