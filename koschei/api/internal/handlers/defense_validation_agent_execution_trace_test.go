package handlers

import (
	"strings"
	"testing"

	"koschei/api/internal/defense"
	"koschei/api/internal/services"
)

func TestDefenseValidationAgentTracePreservesIdentityAndDelegationGaps(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	traces, err := buildDefenseValidationAgentExecutionTraces(request, response.Report)
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != len(request.Cases) {
		t.Fatalf("trace count=%d want=%d", len(traces), len(request.Cases))
	}
	for _, trace := range traces {
		if trace.ActorSubjectID != "" || trace.TraceStatus != services.IntelligenceEvidenceUnverified {
			t.Fatalf("validation-only trace manufactured agent completeness: %#v", trace)
		}
		if trace.IndependentObservationState != services.AgentIndependentObservationVerified {
			t.Fatalf("independent observation state=%q", trace.IndependentObservationState)
		}
		identity := agentTraceStage(t, trace, services.AgentExecutionStageIdentity)
		delegation := agentTraceStage(t, trace, services.AgentExecutionStageDelegation)
		if identity.Status != services.IntelligenceEvidenceUnverified || delegation.Status != services.IntelligenceEvidenceUnverified {
			t.Fatalf("missing identity/delegation evidence was upgraded: identity=%#v delegation=%#v", identity, delegation)
		}
	}
}

func TestDefenseValidationAgentTraceSeparatesAuthorizationOutcomeFromEvidenceStatus(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	traces, err := buildDefenseValidationAgentExecutionTraces(request, response.Report)
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) < 2 {
		t.Fatalf("expected attack and benign traces, got %d", len(traces))
	}
	outcomes := map[string]bool{}
	for _, trace := range traces {
		authorization := agentTraceStage(t, trace, services.AgentExecutionStageAuthorization)
		if authorization.Status != services.IntelligenceEvidenceVerified {
			t.Fatalf("authorization evidence status=%q", authorization.Status)
		}
		outcomes[authorization.Outcome] = true
		enforcement := agentTraceStage(t, trace, services.AgentExecutionStageEnforcement)
		if enforcement.Status != services.IntelligenceEvidenceObserved || enforcement.Outcome != "policy_evidence_observed" {
			t.Fatalf("runtime policy was incorrectly promoted to enforced: %#v", enforcement)
		}
	}
	if !outcomes["allow"] || !outcomes["block"] {
		t.Fatalf("expected independently represented allow and block outcomes, got %#v", outcomes)
	}
}

func TestDefenseValidationAgentTraceBindsStagesToOneMaterialAction(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	traces, err := buildDefenseValidationAgentExecutionTraces(request, response.Report)
	if err != nil {
		t.Fatal(err)
	}
	for _, trace := range traces {
		bindingRef := ""
		for _, stageName := range []string{
			services.AgentExecutionStageAuthorization,
			services.AgentExecutionStageEnforcement,
			services.AgentExecutionStageExecution,
			services.AgentExecutionStageEffect,
		} {
			stage := agentTraceStage(t, trace, stageName)
			found := ""
			for _, ref := range stage.EvidenceRefs {
				if strings.HasPrefix(ref, "agent-material-action-binding:kamb_") {
					found = ref
					break
				}
			}
			if found == "" {
				t.Fatalf("stage %q missing material-action binding: %#v", stageName, stage)
			}
			if bindingRef == "" {
				bindingRef = found
			} else if bindingRef != found {
				t.Fatalf("trace stages reference different material actions: first=%q stage=%q ref=%q", bindingRef, stageName, found)
			}
		}
	}
}

func TestDefenseValidationAgentTraceWithoutSignedObservationDowngradesEffect(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	for index := range request.Cases {
		request.Cases[index].ObservationBinding = nil
		request.Cases[index].ObservationEvent = nil
	}
	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	traces, err := buildDefenseValidationAgentExecutionTraces(request, response.Report)
	if err != nil {
		t.Fatal(err)
	}
	for _, trace := range traces {
		if trace.IndependentObservationState != services.AgentIndependentObservationUnavailable {
			t.Fatalf("independent observation state=%q", trace.IndependentObservationState)
		}
		effect := agentTraceStage(t, trace, services.AgentExecutionStageEffect)
		if effect.Status != services.IntelligenceEvidenceObserved || effect.Confidence != 0.8 {
			t.Fatalf("receipt-only effect must remain observed: %#v", effect)
		}
	}
}

func TestDefenseValidationAgentTraceRejectsTamperedProof(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	resultByCase := defenseValidationUnifiedCaseResults(response.Report)
	raw := request.Cases[0]
	result := resultByCase[defenseValidationUnifiedCaseKey(raw.ControlRef, raw.CaseRef)]
	raw.ExecutionProof.Envelope.Runtime.PolicySHA256 = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if _, err := buildDefenseValidationAgentExecutionTrace(raw, result, defenseValidationUnifiedGeneratedAt(request)); err == nil {
		t.Fatal("tampered execution proof produced an agent evidence trace")
	}
}

func TestDefenseValidationAgentTraceKeepsSandboxScope(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Report.MainnetTransactionSent || response.Report.VerdictAuthority {
		t.Fatalf("fixture escaped validation-only scope: %#v", response.Report)
	}
	for _, control := range response.Report.Controls {
		for _, result := range control.Cases {
			if result.ExecutionMode != defense.DefenseValidationExecutionForkV02 && result.ExecutionMode != defense.DefenseValidationExecutionSandboxV02 {
				t.Fatalf("unexpected execution mode %q", result.ExecutionMode)
			}
		}
	}
}

func agentTraceStage(t *testing.T, trace services.AgentExecutionEvidenceTrace, stage string) services.AgentExecutionStageEvidence {
	t.Helper()
	for _, item := range trace.Stages {
		if item.Stage == stage {
			return item
		}
	}
	t.Fatalf("stage %q missing from trace", stage)
	return services.AgentExecutionStageEvidence{}
}
