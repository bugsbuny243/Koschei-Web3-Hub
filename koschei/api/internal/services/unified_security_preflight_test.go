package services

import "testing"

func TestProspectiveCapabilityRequiresEvidenceBeforeProspectiveLifecycle(t *testing.T) {
	capability := BuildProspectiveIntelligenceCapability(
		"subject-delegate-1",
		UnifiedSecurityDomainBlockchain,
		IntelligenceCapabilitySpend,
		"single_token_account_allowance",
		"token-account-1",
		"subject-owner-1",
		IntelligenceEvidenceVerified,
		nil,
		nil,
		1,
	)
	if capability.Status != IntelligenceEvidenceUnverified {
		t.Fatalf("status=%q", capability.Status)
	}
	if capability.Lifecycle != IntelligenceCapabilityLifecycleUnknown {
		t.Fatalf("unverified preflight capability must not become prospective: %#v", capability)
	}
}

func TestProspectiveCapabilityStaysDistinctFromCurrentlyActiveAuthority(t *testing.T) {
	capability := BuildProspectiveIntelligenceCapability(
		"subject-delegate-1",
		UnifiedSecurityDomainBlockchain,
		IntelligenceCapabilityTokenAuthority,
		"mint_supply",
		"mint-1",
		"subject-owner-1",
		IntelligenceEvidenceVerified,
		[]string{"mint_wide=true"},
		[]string{"simulation:authority:1"},
		1,
	)
	if capability.Status != IntelligenceEvidenceVerified {
		t.Fatalf("status=%q", capability.Status)
	}
	if capability.Lifecycle != IntelligenceCapabilityLifecycleProspective {
		t.Fatalf("pre-signing authority must be prospective, got %#v", capability)
	}
	if capability.Kind != IntelligenceCapabilityTokenAuthority {
		t.Fatalf("kind=%q", capability.Kind)
	}
}
