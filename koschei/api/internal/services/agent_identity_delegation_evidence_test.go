package services

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/securityevidence"
)

func TestAdaptSignedAgentIdentityDelegationEvidenceV1(t *testing.T) {
	event, binding := signedAgentIdentityDelegationFixture(t)
	projection, err := AdaptSignedAgentIdentityDelegationEvidenceV1(event, binding)
	if err != nil {
		t.Fatal(err)
	}
	if projection.ActorSubjectID != binding.ExpectedSubject.ID || projection.IntentRef != binding.IntentSHA256 {
		t.Fatalf("projection binding mismatch: %#v", projection)
	}
	if projection.Identity.Status != IntelligenceEvidenceVerified || projection.Delegation.Status != IntelligenceEvidenceVerified {
		t.Fatalf("verified signed bindings were not preserved: identity=%#v delegation=%#v", projection.Identity, projection.Delegation)
	}
	if projection.Identity.ArtifactDigestSHA256 != binding.IdentityArtifactSHA256 || projection.Delegation.ArtifactDigestSHA256 != binding.DelegationArtifactSHA256 {
		t.Fatalf("artifact digests drifted: %#v", projection)
	}
}

func TestAdaptSignedAgentIdentityDelegationEvidenceV1RejectsSubjectSubstitution(t *testing.T) {
	event, binding := signedAgentIdentityDelegationFixture(t)
	binding.ExpectedSubject.ID = "agent-b"
	if _, err := AdaptSignedAgentIdentityDelegationEvidenceV1(event, binding); err == nil {
		t.Fatal("subject-substituted agent evidence was accepted")
	}
}

func TestAdaptSignedAgentIdentityDelegationEvidenceV1RejectsDelegationDigestMismatch(t *testing.T) {
	event, binding := signedAgentIdentityDelegationFixture(t)
	binding.DelegationArtifactSHA256 = strings.Repeat("d", 64)
	if _, err := AdaptSignedAgentIdentityDelegationEvidenceV1(event, binding); err == nil {
		t.Fatal("delegation digest substitution was accepted")
	}
}

func TestAdaptSignedAgentIdentityDelegationEvidenceV1RejectsUnsignedEvent(t *testing.T) {
	event, binding := signedAgentIdentityDelegationFixture(t)
	event.Authentication = nil
	resealed, err := event.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdaptSignedAgentIdentityDelegationEvidenceV1(resealed, binding); err == nil {
		t.Fatal("unsigned agent evidence was accepted")
	}
}

func TestBindSignedAgentIdentityDelegationEvidenceV1DoesNotCreateLaterStageEvidence(t *testing.T) {
	event, binding := signedAgentIdentityDelegationFixture(t)
	trace := BuildAgentExecutionEvidenceTrace(
		"",
		binding.IntentSHA256,
		AgentIndependentObservationUnavailable,
		"",
		"",
		[]AgentExecutionStageEvidence{{
			Stage:        AgentExecutionStageAuthorization,
			ActionID:     "action-1",
			Outcome:      "allow",
			Status:       IntelligenceEvidenceVerified,
			EvidenceRefs: []string{"authorization:test"},
			Confidence:   1,
		}},
		time.Unix(1_789_000_000, 0).UTC(),
	)
	bound, err := BindSignedAgentIdentityDelegationEvidenceV1(trace, event, binding)
	if err != nil {
		t.Fatal(err)
	}
	if bound.ActorSubjectID != binding.ExpectedSubject.ID {
		t.Fatalf("actor subject=%q", bound.ActorSubjectID)
	}
	if agentIdentityDelegationStage(t, bound, AgentExecutionStageIdentity).Status != IntelligenceEvidenceVerified || agentIdentityDelegationStage(t, bound, AgentExecutionStageDelegation).Status != IntelligenceEvidenceVerified {
		t.Fatalf("identity/delegation evidence did not bind: %#v", bound)
	}
	if agentIdentityDelegationStage(t, bound, AgentExecutionStageEnforcement).Status != IntelligenceEvidenceUnverified ||
		agentIdentityDelegationStage(t, bound, AgentExecutionStageExecution).Status != IntelligenceEvidenceUnverified ||
		agentIdentityDelegationStage(t, bound, AgentExecutionStageEffect).Status != IntelligenceEvidenceUnverified {
		t.Fatalf("identity/delegation evidence manufactured later-stage proof: %#v", bound)
	}
	if bound.TraceStatus != IntelligenceEvidenceUnverified {
		t.Fatalf("incomplete trace status=%q want unverified", bound.TraceStatus)
	}
}

func TestBindSignedAgentIdentityDelegationEvidenceV1RejectsIntentMismatch(t *testing.T) {
	event, binding := signedAgentIdentityDelegationFixture(t)
	trace := BuildAgentExecutionEvidenceTrace("", strings.Repeat("e", 64), AgentIndependentObservationUnavailable, "", "", nil, time.Now().UTC())
	if _, err := BindSignedAgentIdentityDelegationEvidenceV1(trace, event, binding); err == nil {
		t.Fatal("identity/delegation evidence bound to a different execution intent")
	}
}

func TestBindSignedAgentIdentityDelegationEvidenceV1RejectsInvalidExistingIntent(t *testing.T) {
	event, binding := signedAgentIdentityDelegationFixture(t)
	trace := BuildAgentExecutionEvidenceTrace("", "", AgentIndependentObservationUnavailable, "", "", nil, time.Now().UTC())
	trace.IntentRef = "not-a-sha256"
	if _, err := BindSignedAgentIdentityDelegationEvidenceV1(trace, event, binding); err == nil {
		t.Fatal("agent evidence replaced an invalid existing trace intent")
	}
}

func TestBindSignedAgentIdentityDelegationEvidenceV1RejectsTamperedEvent(t *testing.T) {
	event, binding := signedAgentIdentityDelegationFixture(t)
	event.Subject.ID = "agent-b"
	trace := BuildAgentExecutionEvidenceTrace("", binding.IntentSHA256, AgentIndependentObservationUnavailable, "", "", nil, time.Now().UTC())
	if _, err := BindSignedAgentIdentityDelegationEvidenceV1(trace, event, binding); err == nil {
		t.Fatal("tampered signed event was accepted at bind boundary")
	}
}

func signedAgentIdentityDelegationFixture(t *testing.T) (securityevidence.Event, AgentIdentityDelegationEvidenceBindingV1) {
	t.Helper()
	identityDigest := strings.Repeat("a", 64)
	delegationDigest := strings.Repeat("b", 64)
	intentDigest := strings.Repeat("c", 64)
	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	subject := securityevidence.Subject{Chain: "agentic", Type: IntelligenceSubjectAgent, ID: "agent-a"}
	event, err := (securityevidence.Event{
		Producer:      "agent-trust-collector-v1",
		Subject:       subject,
		Window:        securityevidence.ObservationWindow{FromUnixMS: 1_789_000_000_000, ToUnixMS: 1_789_000_001_000},
		SourceDigests: []string{identityDigest, delegationDigest, intentDigest},
		Findings: []securityevidence.Finding{
			{ID: "identity", Kind: AgentIdentityEvidenceFindingKindV1, State: securityevidence.StateVerified, EvidenceSHA256: identityDigest},
			{ID: "delegation", Kind: AgentDelegationEvidenceFindingKindV1, State: securityevidence.StateVerified, EvidenceSHA256: delegationDigest},
		},
	}).SignEd25519(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return event, AgentIdentityDelegationEvidenceBindingV1{
		ExpectedProducer:         "agent-trust-collector-v1",
		TrustedPublicKey:         base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:          subject,
		IdentityArtifactSHA256:   identityDigest,
		DelegationArtifactSHA256: delegationDigest,
		IntentSHA256:             intentDigest,
	}
}

func agentIdentityDelegationStage(t *testing.T, trace AgentExecutionEvidenceTrace, stage string) AgentExecutionStageEvidence {
	t.Helper()
	for _, candidate := range trace.Stages {
		if candidate.Stage == stage {
			return candidate
		}
	}
	t.Fatalf("stage %q not found in %#v", stage, trace)
	return AgentExecutionStageEvidence{}
}
