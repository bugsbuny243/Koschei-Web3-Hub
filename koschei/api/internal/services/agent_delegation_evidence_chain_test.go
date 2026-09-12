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

func TestBuildSignedAgentDelegationEvidenceChainV1(t *testing.T) {
	inputs := signedAgentDelegationChainFixture(t)
	chain, err := BuildSignedAgentDelegationEvidenceChainV1(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if chain.ContractVersion != AgentDelegationEvidenceChainContractVersionV1 || chain.ID == "" || chain.ChainDigestSHA256 == "" {
		t.Fatalf("delegation chain identity incomplete: %#v", chain)
	}
	if chain.RootSubjectID != "principal-a" || chain.TerminalSubjectID != "agent-c" || len(chain.Hops) != 2 {
		t.Fatalf("delegation chain topology drifted: %#v", chain)
	}
	if chain.ChainStatus != IntelligenceEvidenceVerified {
		t.Fatalf("chain status=%q want verified", chain.ChainStatus)
	}
	if chain.ConstraintSemanticsEvaluated || chain.AuthorityDecision != AgentDelegationChainAuthorityNotEvaluatedV1 {
		t.Fatalf("evidence chain manufactured authority semantics: %#v", chain)
	}
}

func TestBuildSignedAgentDelegationEvidenceChainV1RejectsBrokenParentHash(t *testing.T) {
	inputs := signedAgentDelegationChainFixture(t)
	inputs[1] = signedAgentDelegationHopFixture(t, 8, "agent-b", "agent-c", strings.Repeat("d", 64), strings.Repeat("9", 64), strings.Repeat("c", 64), strings.Repeat("f", 64))
	if _, err := BuildSignedAgentDelegationEvidenceChainV1(inputs); err == nil {
		t.Fatal("delegation chain accepted a broken parent hash link")
	}
}

func TestBuildSignedAgentDelegationEvidenceChainV1RejectsIntentSubstitution(t *testing.T) {
	inputs := signedAgentDelegationChainFixture(t)
	inputs[1] = signedAgentDelegationHopFixture(t, 8, "agent-b", "agent-c", strings.Repeat("d", 64), strings.Repeat("a", 64), strings.Repeat("e", 64), strings.Repeat("f", 64))
	if _, err := BuildSignedAgentDelegationEvidenceChainV1(inputs); err == nil {
		t.Fatal("delegation chain accepted a different downstream intent")
	}
}

func TestBuildSignedAgentDelegationEvidenceChainV1RejectsSubjectDiscontinuity(t *testing.T) {
	inputs := signedAgentDelegationChainFixture(t)
	inputs[1] = signedAgentDelegationHopFixture(t, 8, "agent-x", "agent-c", strings.Repeat("d", 64), strings.Repeat("a", 64), strings.Repeat("c", 64), strings.Repeat("f", 64))
	if _, err := BuildSignedAgentDelegationEvidenceChainV1(inputs); err == nil {
		t.Fatal("delegation chain accepted a disconnected subject path")
	}
}

func TestBuildSignedAgentDelegationEvidenceChainV1RejectsCycle(t *testing.T) {
	first := signedAgentDelegationHopFixture(t, 7, "agent-a", "agent-b", strings.Repeat("a", 64), "", strings.Repeat("c", 64), strings.Repeat("e", 64))
	second := signedAgentDelegationHopFixture(t, 8, "agent-b", "agent-a", strings.Repeat("d", 64), strings.Repeat("a", 64), strings.Repeat("c", 64), strings.Repeat("f", 64))
	if _, err := BuildSignedAgentDelegationEvidenceChainV1([]SignedAgentDelegationHopInputV1{first, second}); err == nil {
		t.Fatal("delegation chain accepted a subject cycle")
	}
}

func TestBuildSignedAgentDelegationEvidenceChainV1RejectsWrongSigner(t *testing.T) {
	inputs := signedAgentDelegationChainFixture(t)
	wrongKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	inputs[1].Binding.TrustedPublicKey = base64.RawURLEncoding.EncodeToString(wrongKey)
	if _, err := BuildSignedAgentDelegationEvidenceChainV1(inputs); err == nil {
		t.Fatal("delegation chain accepted a hop signed by an untrusted producer key")
	}
}

func TestBindSignedAgentDelegationEvidenceChainV1OnlyFillsDelegationStage(t *testing.T) {
	inputs := signedAgentDelegationChainFixture(t)
	intent := inputs[0].Binding.IntentSHA256
	now := time.Unix(1_789_100_000, 0).UTC()
	trace := BuildAgentExecutionEvidenceTrace(
		"agent-c",
		intent,
		AgentIndependentObservationUnavailable,
		"",
		"",
		[]AgentExecutionStageEvidence{{
			Stage:                AgentExecutionStageIdentity,
			SubjectID:            "agent-c",
			ArtifactRef:          "agent-identity-binding",
			ArtifactDigestSHA256: strings.Repeat("1", 64),
			Outcome:              "bound",
			Status:               IntelligenceEvidenceVerified,
			EvidenceRefs:         []string{"identity:test"},
			Confidence:           1,
		}},
		now,
	)
	bound, err := BindSignedAgentDelegationEvidenceChainV1(trace, inputs)
	if err != nil {
		t.Fatal(err)
	}
	delegation := delegationChainStage(t, bound, AgentExecutionStageDelegation)
	if delegation.Status != IntelligenceEvidenceVerified || !strings.HasPrefix(delegation.ArtifactRef, "kadc_") || delegation.ArtifactDigestSHA256 == "" {
		t.Fatalf("delegation chain was not bound to the trace: %#v", delegation)
	}
	for _, stageName := range []string{AgentExecutionStageAuthorization, AgentExecutionStageEnforcement, AgentExecutionStageExecution, AgentExecutionStageEffect} {
		if stage := delegationChainStage(t, bound, stageName); stage.Status != IntelligenceEvidenceUnverified {
			t.Fatalf("delegation chain manufactured %s evidence: %#v", stageName, stage)
		}
	}
	if bound.TraceStatus != IntelligenceEvidenceUnverified {
		t.Fatalf("identity+delegation alone completed execution trace: %#v", bound)
	}
}

func TestBindSignedAgentDelegationEvidenceChainV1RequiresIndependentActorIdentity(t *testing.T) {
	inputs := signedAgentDelegationChainFixture(t)
	trace := BuildAgentExecutionEvidenceTrace("", inputs[0].Binding.IntentSHA256, AgentIndependentObservationUnavailable, "", "", nil, time.Now().UTC())
	if _, err := BindSignedAgentDelegationEvidenceChainV1(trace, inputs); err == nil {
		t.Fatal("delegation evidence was allowed to manufacture agent identity")
	}
}

func signedAgentDelegationChainFixture(t *testing.T) []SignedAgentDelegationHopInputV1 {
	t.Helper()
	intent := strings.Repeat("c", 64)
	first := signedAgentDelegationHopFixture(t, 7, "principal-a", "agent-b", strings.Repeat("a", 64), "", intent, strings.Repeat("e", 64))
	second := signedAgentDelegationHopFixture(t, 8, "agent-b", "agent-c", strings.Repeat("d", 64), strings.Repeat("a", 64), intent, strings.Repeat("f", 64))
	return []SignedAgentDelegationHopInputV1{first, second}
}

func signedAgentDelegationHopFixture(t *testing.T, seed byte, delegator, delegatee, delegationDigest, parentDigest, intentDigest, constraintDigest string) SignedAgentDelegationHopInputV1 {
	t.Helper()
	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	subject := securityevidence.Subject{Chain: "agentic", Type: IntelligenceSubjectAgent, ID: delegatee}
	sourceDigests := []string{delegationDigest, intentDigest, constraintDigest}
	if parentDigest != "" {
		sourceDigests = append(sourceDigests, parentDigest)
	}
	event, err := (securityevidence.Event{
		Producer:      "delegation-collector-" + delegatee,
		Subject:       subject,
		Window:        securityevidence.ObservationWindow{FromUnixMS: 1_789_100_000_000, ToUnixMS: 1_789_100_001_000},
		SourceDigests: sourceDigests,
		Findings: []securityevidence.Finding{{
			ID:             "delegation-" + delegatee,
			Kind:           AgentDelegationEvidenceFindingKindV1,
			State:          securityevidence.StateVerified,
			EvidenceSHA256: delegationDigest,
		}},
	}).SignEd25519(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return SignedAgentDelegationHopInputV1{
		Event: event,
		Binding: AgentDelegationHopBindingV1{
			ExpectedProducer:               "delegation-collector-" + delegatee,
			TrustedPublicKey:               base64.RawURLEncoding.EncodeToString(publicKey),
			ExpectedSubject:                subject,
			DelegatorSubjectID:             delegator,
			DelegateeSubjectID:             delegatee,
			DelegationArtifactSHA256:       delegationDigest,
			ParentDelegationArtifactSHA256: parentDigest,
			IntentSHA256:                   intentDigest,
			ConstraintSetSHA256:            constraintDigest,
		},
	}
}

func delegationChainStage(t *testing.T, trace AgentExecutionEvidenceTrace, name string) AgentExecutionStageEvidence {
	t.Helper()
	for _, stage := range trace.Stages {
		if stage.Stage == name {
			return stage
		}
	}
	t.Fatalf("stage %q missing", name)
	return AgentExecutionStageEvidence{}
}
