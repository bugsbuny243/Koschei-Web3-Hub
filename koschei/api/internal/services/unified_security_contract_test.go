package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestUnifiedCapabilityCannotBecomeVerifiedWithoutEvidence(t *testing.T) {
	capability := BuildIntelligenceCapability(
		"subject-agent-1",
		UnifiedSecurityDomainAIAgent,
		IntelligenceCapabilitySpend,
		"treasury:payments",
		"treasury-api",
		"subject-human-1",
		IntelligenceCapabilityLifecycleActive,
		IntelligenceEvidenceVerified,
		nil,
		nil,
		0.9,
	)
	if capability.Status != IntelligenceEvidenceUnverified {
		t.Fatalf("capability=%#v", capability)
	}
	if capability.Lifecycle != IntelligenceCapabilityLifecycleActive {
		t.Fatalf("lifecycle=%q", capability.Lifecycle)
	}
}

func TestUnifiedCapabilityUnknownDomainFailsClosedEvenWithEvidence(t *testing.T) {
	capability := BuildIntelligenceCapability(
		"subject-agent-1",
		"future-unregistered-domain",
		IntelligenceCapabilitySpend,
		"treasury:payments",
		"treasury-api",
		"",
		IntelligenceCapabilityLifecycleActive,
		IntelligenceEvidenceVerified,
		nil,
		[]string{"policy:1"},
		1,
	)
	if capability.Domain != UnifiedSecurityDomainUnknown || capability.Status != IntelligenceEvidenceUnverified {
		t.Fatalf("capability=%#v", capability)
	}
}

func TestUnifiedCapabilityPreservesObservedAndRevokedAuthority(t *testing.T) {
	capability := BuildIntelligenceCapability(
		"subject-agent-1",
		UnifiedSecurityDomainIdentity,
		IntelligenceCapabilityCredential,
		"credential:treasury-operator",
		"vc:treasury-operator",
		"issuer-1",
		IntelligenceCapabilityLifecycleRevoked,
		IntelligenceEvidenceObserved,
		[]string{"spend<=1000", "spend<=1000"},
		[]string{"credential-status:abc", "credential-status:abc"},
		0.7,
	)
	if capability.Status != IntelligenceEvidenceObserved {
		t.Fatalf("status=%q", capability.Status)
	}
	if capability.Lifecycle != IntelligenceCapabilityLifecycleRevoked {
		t.Fatalf("lifecycle=%q", capability.Lifecycle)
	}
	if len(capability.EvidenceRefs) != 1 || len(capability.Constraints) != 1 {
		t.Fatalf("capability=%#v", capability)
	}
}

func TestUnifiedUnknownTimesAreOmittedInsteadOfFabricated(t *testing.T) {
	capability := BuildIntelligenceCapability(
		"subject-agent-1",
		UnifiedSecurityDomainAIAgent,
		IntelligenceCapabilityToolUse,
		"tool:treasury-api",
		"treasury-api",
		"",
		IntelligenceCapabilityLifecycleUnknown,
		IntelligenceEvidenceObserved,
		nil,
		[]string{"tool-registry:1"},
		0.5,
	)
	encoded, err := json.Marshal(capability)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"valid_from", "expires_at", "revoked_at", "0001-01-01"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("fabricated time field %q in %s", forbidden, text)
		}
	}
}

func TestTrustBoundaryTransitionRequiresConcreteDomainsMechanismAndEvidence(t *testing.T) {
	transition := BuildTrustBoundaryTransition(
		"subject-agent-1",
		"capability-1",
		"action-1",
		UnifiedSecurityDomainAIAgent,
		"not-a-domain",
		"treasury_api_call",
		IntelligenceEvidenceVerified,
		[]string{"tool-log:1"},
		1,
	)
	if transition.Status != IntelligenceEvidenceUnverified || transition.ToDomain != UnifiedSecurityDomainUnknown {
		t.Fatalf("transition=%#v", transition)
	}

	transition = BuildTrustBoundaryTransition(
		"subject-agent-1",
		"capability-1",
		"action-1",
		UnifiedSecurityDomainAIAgent,
		UnifiedSecurityDomainBlockchain,
		"treasury_api_call",
		IntelligenceEvidenceVerified,
		[]string{"tool-log:1", "tx:abc"},
		1.4,
	)
	if transition.Status != IntelligenceEvidenceVerified || transition.Confidence != 1 {
		t.Fatalf("transition=%#v", transition)
	}
}

func TestUnifiedActionKeepsCaseSensitiveResourceIdentity(t *testing.T) {
	upper := BuildIntelligenceAction(
		"subject-agent-1",
		"capability-1",
		"subject-wallet-1",
		UnifiedSecurityDomainBlockchain,
		"submit_transaction",
		"AbCdEf123",
		"SigA",
		"balance_delta",
		IntelligenceEvidenceObserved,
		[]string{"tx:SigA"},
		0.8,
	)
	lower := BuildIntelligenceAction(
		"subject-agent-1",
		"capability-1",
		"subject-wallet-1",
		UnifiedSecurityDomainBlockchain,
		"submit_transaction",
		"abcdef123",
		"SigA",
		"balance_delta",
		IntelligenceEvidenceObserved,
		[]string{"tx:SigA"},
		0.8,
	)
	if upper.ResourceRef == lower.ResourceRef || upper.ID == lower.ID {
		t.Fatalf("case-sensitive identities conflated: upper=%#v lower=%#v", upper, lower)
	}
}

func TestUnifiedAttackPathFailsClosedWhenAnyStepLacksEvidence(t *testing.T) {
	path := BuildUnifiedSecurityAttackPath(
		"Agent authority reaches treasury",
		"subject-agent-1",
		IntelligenceEvidenceVerified,
		nil,
		[]UnifiedSecurityAttackPathStep{
			{SubjectID: "subject-agent-1", CapabilityID: "cap-1", ActionID: "action-1", Effect: "calls treasury API", EvidenceRefs: []string{"tool-log:1"}},
			{SubjectID: "subject-wallet-1", ActionID: "action-2", Effect: "submits bridge transaction"},
		},
		nil,
		[]string{"case:1"},
		0.9,
	)
	if path.Status != IntelligenceEvidenceUnverified {
		t.Fatalf("path=%#v", path)
	}
	if path.Steps[0].Order != 1 || path.Steps[1].Order != 2 {
		t.Fatalf("steps=%#v", path.Steps)
	}
}

func TestUnifiedAttackPathCanRepresentCrossDomainEvidenceChainWithoutRegradingBase(t *testing.T) {
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	base := BuildIntelligenceInvestigation([]IntelligenceSubject{{ID: "subject-wallet-1", Kind: IntelligenceSubjectAddress}}, now)
	unified := BuildUnifiedSecurityInvestigation(base, now)

	capability := BuildIntelligenceCapability(
		"subject-agent-1",
		UnifiedSecurityDomainAIAgent,
		IntelligenceCapabilitySpend,
		"treasury:payments",
		"treasury-api",
		"subject-human-1",
		IntelligenceCapabilityLifecycleActive,
		IntelligenceEvidenceVerified,
		[]string{"daily_limit=1000"},
		[]string{"policy:agent-spend-v1"},
		1,
	)
	action := BuildIntelligenceAction(
		"subject-agent-1",
		capability.ID,
		"subject-wallet-1",
		UnifiedSecurityDomainAIAgent,
		"request_treasury_transfer",
		"treasury-api",
		"",
		"transfer_requested",
		IntelligenceEvidenceVerified,
		[]string{"tool-log:42"},
		1,
	)
	transition := BuildTrustBoundaryTransition(
		"subject-agent-1",
		capability.ID,
		action.ID,
		UnifiedSecurityDomainAIAgent,
		UnifiedSecurityDomainBlockchain,
		"treasury_api_to_signer",
		IntelligenceEvidenceVerified,
		[]string{"tool-log:42", "signer-log:7"},
		1,
	)
	consequence := BuildIntelligenceConsequence(
		UnifiedSecurityDomainBlockchain,
		"asset_transfer",
		"treasury:asset:USDC",
		"Treasury value can move if the requested transfer is authorized and submitted.",
		"potential_balance_decrease",
		IntelligenceEvidenceInferred,
		[]string{"simulation:99"},
		0.6,
	)
	path := BuildUnifiedSecurityAttackPath(
		"Agent -> treasury -> chain",
		"subject-agent-1",
		IntelligenceEvidenceInferred,
		[]string{"agent retains active spend capability"},
		[]UnifiedSecurityAttackPathStep{
			{SubjectID: "subject-agent-1", CapabilityID: capability.ID, ActionID: action.ID, TargetSubjectID: "subject-wallet-1", BoundaryTransitionID: transition.ID, Effect: "requests treasury transfer", EvidenceRefs: []string{"tool-log:42"}},
			{SubjectID: "subject-wallet-1", Effect: "transaction could move treasury asset", EvidenceRefs: []string{"simulation:99"}},
		},
		[]IntelligenceConsequence{consequence},
		[]string{"policy:agent-spend-v1", "tool-log:42", "simulation:99"},
		0.6,
	)

	unified.Capabilities = []IntelligenceCapability{capability}
	unified.Actions = []IntelligenceAction{action}
	unified.BoundaryTransitions = []IntelligenceTrustBoundaryTransition{transition}
	unified.AttackPaths = []UnifiedSecurityAttackPath{path}

	if unified.ContractVersion != UnifiedSecurityContractVersion {
		t.Fatalf("contract_version=%q", unified.ContractVersion)
	}
	if path.Status != IntelligenceEvidenceInferred {
		t.Fatalf("path=%#v", path)
	}
	if unified.Base.Decision.Status != IntelligenceEvidenceUnverified || unified.Base.Decision.Action != "investigate" {
		t.Fatalf("base decision was regraded: %#v", unified.Base.Decision)
	}
	if transition.FromDomain != UnifiedSecurityDomainAIAgent || transition.ToDomain != UnifiedSecurityDomainBlockchain {
		t.Fatalf("transition=%#v", transition)
	}
}
