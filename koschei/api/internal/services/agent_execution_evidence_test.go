package services

import (
	"strings"
	"testing"
	"time"
)

func TestAgentExecutionTraceDoesNotEquateAuthorizationWithExecutionOrEffect(t *testing.T) {
	now := time.Date(2026, 9, 11, 16, 30, 0, 0, time.UTC)
	stages := []AgentExecutionStageEvidence{
		{Stage: AgentExecutionStageIdentity, Status: IntelligenceEvidenceVerified, EvidenceRefs: []string{"identity-evidence"}, Confidence: 1},
		{Stage: AgentExecutionStageDelegation, Status: IntelligenceEvidenceVerified, EvidenceRefs: []string{"delegation-evidence"}, Confidence: 1},
		{Stage: AgentExecutionStageAuthorization, Status: IntelligenceEvidenceVerified, EvidenceRefs: []string{"authorization-evidence"}, Confidence: 1},
	}
	trace := BuildAgentExecutionEvidenceTrace("agent-1", "intent-1", AgentIndependentObservationUnavailable, "", "", stages, now)
	if trace.TraceStatus != IntelligenceEvidenceUnverified {
		t.Fatalf("authorization-only trace must remain unverified, got %q", trace.TraceStatus)
	}
	if len(trace.Stages) != 6 {
		t.Fatalf("expected six explicit stages, got %d", len(trace.Stages))
	}
	for _, stage := range trace.Stages {
		if stage.Stage == AgentExecutionStageExecution || stage.Stage == AgentExecutionStageEffect {
			if stage.Status != IntelligenceEvidenceUnverified {
				t.Fatalf("missing %s evidence was upgraded to %q", stage.Stage, stage.Status)
			}
		}
	}
}

func TestAgentExecutionTraceRequiresReceiptDigestForVerifiedIndependentObservation(t *testing.T) {
	stages := verifiedAgentExecutionStages()
	trace := BuildAgentExecutionEvidenceTrace("agent-1", "intent-1", AgentIndependentObservationVerified, "receipt://1", "", stages, time.Time{})
	if trace.IndependentObservationState != AgentIndependentObservationUnverified {
		t.Fatalf("verified independent observation without receipt digest must fail closed, got %q", trace.IndependentObservationState)
	}
	if trace.TraceStatus == IntelligenceEvidenceVerified {
		t.Fatal("trace became verified without a receipt digest")
	}
}

func TestAgentExecutionTraceCanVerifyOnlyWithCompleteStagesAndIndependentReceipt(t *testing.T) {
	digest := strings.Repeat("a", 64)
	trace := BuildAgentExecutionEvidenceTrace("agent-1", "intent-1", AgentIndependentObservationVerified, "receipt://1", digest, verifiedAgentExecutionStages(), time.Time{})
	if trace.IndependentObservationState != AgentIndependentObservationVerified {
		t.Fatalf("independent observation state=%q", trace.IndependentObservationState)
	}
	if trace.TraceStatus != IntelligenceEvidenceVerified {
		t.Fatalf("complete independently observed trace status=%q", trace.TraceStatus)
	}
	if trace.ReceiptDigestSHA256 != digest {
		t.Fatalf("receipt digest=%q", trace.ReceiptDigestSHA256)
	}
}

func TestAgentExecutionTracePolicyAndEffectDigestsAffectStableID(t *testing.T) {
	stagesA := verifiedAgentExecutionStages()
	stagesB := verifiedAgentExecutionStages()
	stagesA[2].PolicyDigestSHA256 = strings.Repeat("b", 64)
	stagesB[2].PolicyDigestSHA256 = strings.Repeat("c", 64)
	stagesA[5].ArtifactDigestSHA256 = strings.Repeat("d", 64)
	stagesB[5].ArtifactDigestSHA256 = strings.Repeat("e", 64)
	receipt := strings.Repeat("f", 64)
	traceA := BuildAgentExecutionEvidenceTrace("agent-1", "intent-1", AgentIndependentObservationVerified, "receipt://1", receipt, stagesA, time.Time{})
	traceB := BuildAgentExecutionEvidenceTrace("agent-1", "intent-1", AgentIndependentObservationVerified, "receipt://1", receipt, stagesB, time.Time{})
	if traceA.ID == traceB.ID {
		t.Fatal("trace identity did not bind policy/effect digests")
	}
}

func verifiedAgentExecutionStages() []AgentExecutionStageEvidence {
	stages := make([]AgentExecutionStageEvidence, 0, 6)
	for _, stage := range requiredAgentExecutionStages {
		stages = append(stages, AgentExecutionStageEvidence{
			Stage:        stage,
			SubjectID:    "agent-1",
			ActionID:     "action-1",
			Status:       IntelligenceEvidenceVerified,
			EvidenceRefs: []string{"evidence-" + stage},
			Confidence:   1,
		})
	}
	return stages
}
