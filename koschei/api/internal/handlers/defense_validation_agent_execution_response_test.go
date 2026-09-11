package handlers

import (
	"testing"

	"koschei/api/internal/services"
)

func TestDefenseValidationResponseIncludesAgentExecutionEvidenceV1(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.AgentExecutionEvidence) != len(request.Cases) {
		t.Fatalf("agent execution trace count=%d want=%d", len(response.AgentExecutionEvidence), len(request.Cases))
	}
	for _, trace := range response.AgentExecutionEvidence {
		if trace.ContractVersion != services.AgentExecutionEvidenceContractVersion {
			t.Fatalf("contract version=%q", trace.ContractVersion)
		}
		if trace.ActorSubjectID != "" || trace.TraceStatus != services.IntelligenceEvidenceUnverified {
			t.Fatalf("validation response manufactured agent authority: %#v", trace)
		}
		if trace.IndependentObservationState != services.AgentIndependentObservationVerified {
			t.Fatalf("independent observation state=%q", trace.IndependentObservationState)
		}
	}
}

func TestDefenseValidationResponseWithoutIndependentObservationKeepsEffectObserved(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	for index := range request.Cases {
		request.Cases[index].ObservationBinding = nil
		request.Cases[index].ObservationEvent = nil
	}
	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, trace := range response.AgentExecutionEvidence {
		if trace.IndependentObservationState != services.AgentIndependentObservationUnavailable {
			t.Fatalf("independent observation state=%q", trace.IndependentObservationState)
		}
		effect := agentTraceStage(t, trace, services.AgentExecutionStageEffect)
		if effect.Status != services.IntelligenceEvidenceObserved {
			t.Fatalf("receipt-only effect status=%q", effect.Status)
		}
	}
}
