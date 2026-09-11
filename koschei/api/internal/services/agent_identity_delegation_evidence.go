package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/securityevidence"
)

const (
	AgentIdentityEvidenceFindingKindV1   = "agent_identity_binding"
	AgentDelegationEvidenceFindingKindV1 = "agent_delegation_binding"
)

type AgentIdentityDelegationEvidenceBindingV1 struct {
	ExpectedProducer         string
	TrustedPublicKey         string
	ExpectedSubject          securityevidence.Subject
	IdentityArtifactSHA256   string
	DelegationArtifactSHA256 string
	IntentSHA256             string
}

type AgentIdentityDelegationEvidenceProjectionV1 struct {
	ActorSubjectID string
	IntentRef      string
	EventSHA256    string
	Identity       AgentExecutionStageEvidence
	Delegation     AgentExecutionStageEvidence
}

// AdaptSignedAgentIdentityDelegationEvidenceV1 authenticates identity and
// delegation evidence without creating authorization authority. The producer
// key is supplied out of band by the consuming control; caller-selected trust
// material cannot upgrade a stage. Exact subject, identity artifact,
// delegation artifact and intent digests are all bound before evidence can be
// projected into the agent execution trace.
func AdaptSignedAgentIdentityDelegationEvidenceV1(event securityevidence.Event, binding AgentIdentityDelegationEvidenceBindingV1) (AgentIdentityDelegationEvidenceProjectionV1, error) {
	expectedProducer := strings.TrimSpace(binding.ExpectedProducer)
	trustedPublicKey := strings.TrimSpace(binding.TrustedPublicKey)
	if expectedProducer == "" || trustedPublicKey == "" {
		return AgentIdentityDelegationEvidenceProjectionV1{}, errors.New("trusted producer identity and public key are required")
	}

	expectedSubject := binding.ExpectedSubject
	expectedSubject.Chain = strings.ToLower(strings.TrimSpace(expectedSubject.Chain))
	expectedSubject.Type = strings.ToLower(strings.TrimSpace(expectedSubject.Type))
	expectedSubject.ID = strings.TrimSpace(expectedSubject.ID)
	if expectedSubject.Chain == "" || expectedSubject.Type != IntelligenceSubjectAgent || expectedSubject.ID == "" {
		return AgentIdentityDelegationEvidenceProjectionV1{}, errors.New("expected agent subject binding is required")
	}

	identityDigest := normalizeAgentSHA256(binding.IdentityArtifactSHA256)
	delegationDigest := normalizeAgentSHA256(binding.DelegationArtifactSHA256)
	intentDigest := normalizeAgentSHA256(binding.IntentSHA256)
	if identityDigest == "" || delegationDigest == "" || intentDigest == "" {
		return AgentIdentityDelegationEvidenceProjectionV1{}, errors.New("identity, delegation and intent bindings require sha256 digests")
	}

	eventSHA256 := strings.ToLower(strings.TrimSpace(event.EventSHA256))
	if err := event.VerifyEd25519(expectedProducer, trustedPublicKey); err != nil {
		return AgentIdentityDelegationEvidenceProjectionV1{}, fmt.Errorf("authenticate agent identity/delegation evidence event: %w", err)
	}
	canonical, err := event.Canonical()
	if err != nil {
		return AgentIdentityDelegationEvidenceProjectionV1{}, fmt.Errorf("canonicalize agent identity/delegation evidence event: %w", err)
	}
	canonical.EventSHA256 = eventSHA256
	if !unifiedSignedEvidenceSubjectMatches(canonical.Subject, expectedSubject) {
		return AgentIdentityDelegationEvidenceProjectionV1{}, errors.New("agent evidence subject does not match trusted adapter binding")
	}
	if !agentEventHasSourceDigests(canonical.SourceDigests, identityDigest, delegationDigest, intentDigest) {
		return AgentIdentityDelegationEvidenceProjectionV1{}, errors.New("agent evidence event does not bind identity, delegation and intent source digests")
	}

	identityFinding, err := agentBindingFinding(canonical.Findings, AgentIdentityEvidenceFindingKindV1, identityDigest)
	if err != nil {
		return AgentIdentityDelegationEvidenceProjectionV1{}, fmt.Errorf("identity evidence: %w", err)
	}
	delegationFinding, err := agentBindingFinding(canonical.Findings, AgentDelegationEvidenceFindingKindV1, delegationDigest)
	if err != nil {
		return AgentIdentityDelegationEvidenceProjectionV1{}, fmt.Errorf("delegation evidence: %w", err)
	}

	observedAt := agentEvidenceObservedAt(canonical.Window)
	identityStatus := unifiedSignedFindingStatus(identityFinding.State)
	delegationStatus := unifiedSignedFindingStatus(delegationFinding.State)
	eventRef := "security-evidence:" + eventSHA256

	return AgentIdentityDelegationEvidenceProjectionV1{
		ActorSubjectID: expectedSubject.ID,
		IntentRef:      intentDigest,
		EventSHA256:    eventSHA256,
		Identity: AgentExecutionStageEvidence{
			Stage:                AgentExecutionStageIdentity,
			SubjectID:            expectedSubject.ID,
			ArtifactRef:          "agent-identity-binding",
			ArtifactDigestSHA256: identityDigest,
			Outcome:              "bound",
			Status:               identityStatus,
			ObservedAt:           observedAt,
			EvidenceRefs:         []string{eventRef, "agent-identity:" + identityDigest},
			Confidence:           unifiedSignedFindingConfidence(identityStatus),
		},
		Delegation: AgentExecutionStageEvidence{
			Stage:                AgentExecutionStageDelegation,
			SubjectID:            expectedSubject.ID,
			ArtifactRef:          "agent-delegation-binding",
			ArtifactDigestSHA256: delegationDigest,
			Outcome:              "delegated",
			Status:               delegationStatus,
			ObservedAt:           observedAt,
			EvidenceRefs:         []string{eventRef, "agent-delegation:" + delegationDigest, "intent:" + intentDigest},
			Confidence:           unifiedSignedFindingConfidence(delegationStatus),
		},
	}, nil
}

// BindAgentIdentityDelegationEvidenceV1 fills only the identity/delegation
// stages of an existing evidence trace. It cannot manufacture authorization,
// enforcement, execution or effect evidence, and it rejects intent mismatch.
func BindAgentIdentityDelegationEvidenceV1(trace AgentExecutionEvidenceTrace, projection AgentIdentityDelegationEvidenceProjectionV1) (AgentExecutionEvidenceTrace, error) {
	if trace.ContractVersion != "" && trace.ContractVersion != AgentExecutionEvidenceContractVersion {
		return AgentExecutionEvidenceTrace{}, errors.New("unsupported agent execution evidence contract version")
	}
	actorSubjectID := strings.TrimSpace(projection.ActorSubjectID)
	intentRef := normalizeAgentSHA256(projection.IntentRef)
	if actorSubjectID == "" || intentRef == "" {
		return AgentExecutionEvidenceTrace{}, errors.New("authenticated actor subject and intent binding are required")
	}
	if existingIntent := normalizeAgentSHA256(trace.IntentRef); existingIntent != "" && existingIntent != intentRef {
		return AgentExecutionEvidenceTrace{}, errors.New("agent identity/delegation intent does not match execution trace intent")
	}

	stages := append([]AgentExecutionStageEvidence(nil), trace.Stages...)
	stages = append(stages, projection.Identity, projection.Delegation)
	generatedAt := trace.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	return BuildAgentExecutionEvidenceTrace(
		actorSubjectID,
		intentRef,
		trace.IndependentObservationState,
		trace.ReceiptRef,
		trace.ReceiptDigestSHA256,
		stages,
		generatedAt,
	), nil
}

func agentBindingFinding(findings []securityevidence.Finding, kind, expectedDigest string) (securityevidence.Finding, error) {
	var matched *securityevidence.Finding
	for index := range findings {
		finding := findings[index]
		if strings.ToLower(strings.TrimSpace(finding.Kind)) != kind {
			continue
		}
		if matched != nil {
			return securityevidence.Finding{}, fmt.Errorf("multiple %q findings are not allowed", kind)
		}
		copyFinding := finding
		matched = &copyFinding
	}
	if matched == nil {
		return securityevidence.Finding{}, fmt.Errorf("required %q finding is missing", kind)
	}
	if normalizeAgentSHA256(matched.EvidenceSHA256) != expectedDigest {
		return securityevidence.Finding{}, fmt.Errorf("%q finding evidence digest does not match trusted binding", kind)
	}
	if matched.State != securityevidence.StateVerified && matched.State != securityevidence.StateObserved {
		return securityevidence.Finding{}, fmt.Errorf("%q finding must be OBSERVED or VERIFIED", kind)
	}
	return *matched, nil
}

func agentEventHasSourceDigests(sourceDigests []string, required ...string) bool {
	available := make(map[string]struct{}, len(sourceDigests))
	for _, digest := range sourceDigests {
		if normalized := normalizeAgentSHA256(digest); normalized != "" {
			available[normalized] = struct{}{}
		}
	}
	for _, digest := range required {
		if _, ok := available[normalizeAgentSHA256(digest)]; !ok {
			return false
		}
	}
	return true
}

func agentEvidenceObservedAt(window securityevidence.ObservationWindow) *time.Time {
	if window.ToUnixMS <= 0 {
		return nil
	}
	observed := time.UnixMilli(window.ToUnixMS).UTC()
	return &observed
}
