package handlers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/defense"
	"koschei/api/internal/executioncontainment"
	"koschei/api/internal/executionproof"
	"koschei/api/internal/services"
)

// buildDefenseValidationAgentExecutionTraces projects only evidence that has
// already passed the Defense Validation API's execution-integrity and signed
// observation gates. The isolated validation corpus has no agent identity or
// delegation evidence, so those stages are intentionally left UNVERIFIED.
func buildDefenseValidationAgentExecutionTraces(input defenseValidationAPIRequest, report defense.DefenseValidationReportV02) ([]services.AgentExecutionEvidenceTrace, error) {
	results := defenseValidationUnifiedCaseResults(report)
	generatedAt := defenseValidationUnifiedGeneratedAt(input)
	traces := make([]services.AgentExecutionEvidenceTrace, 0, len(input.Cases))
	for _, raw := range input.Cases {
		result, ok := results[defenseValidationUnifiedCaseKey(raw.ControlRef, raw.CaseRef)]
		if !ok {
			return nil, fmt.Errorf("agent execution trace cannot resolve validated case %q", strings.TrimSpace(raw.CaseRef))
		}
		trace, err := buildDefenseValidationAgentExecutionTrace(raw, result, generatedAt)
		if err != nil {
			return nil, fmt.Errorf("case %q agent execution trace: %w", strings.TrimSpace(raw.CaseRef), err)
		}
		traces = append(traces, trace)
	}
	return traces, nil
}

func buildDefenseValidationAgentExecutionTrace(raw defenseValidationAPICase, result defense.DefenseValidationCaseResultV02, generatedAt time.Time) (services.AgentExecutionEvidenceTrace, error) {
	if result.ExecutionEvidenceState != defense.DefenseValidationEvidenceVerifiedV02 {
		return services.AgentExecutionEvidenceTrace{}, errors.New("execution evidence is not verified")
	}
	if !executioncontainment.Verify(raw.ContainmentReceipt) {
		return services.AgentExecutionEvidenceTrace{}, errors.New("containment receipt verification failed")
	}
	if !defenseValidationAgentProofMatches(raw.ExecutionProof) {
		return services.AgentExecutionEvidenceTrace{}, errors.New("execution proof verification failed")
	}
	materialBinding, err := services.BuildAgentMaterialActionBindingV1(raw.ExecutionProof, raw.ContainmentReceipt)
	if err != nil {
		return services.AgentExecutionEvidenceTrace{}, fmt.Errorf("material action binding failed: %w", err)
	}

	proofDigest := materialBinding.ExecutionProofSHA256
	receiptDigest := materialBinding.ContainmentReceiptSHA256
	proofRef := "execution-proof:" + proofDigest
	receiptRef := "execution-containment:" + receiptDigest
	bindingRef := "agent-material-action-binding:" + materialBinding.ID
	actionID := materialBinding.ActionSHA256

	var observedAt *time.Time
	independentState := services.AgentIndependentObservationUnavailable
	effectStatus := services.IntelligenceEvidenceObserved
	effectConfidence := 0.8
	effectRefs := []string{receiptRef, bindingRef}
	if raw.ObservationEvent != nil && result.ObservationEvidenceState == defense.DefenseValidationEvidenceVerifiedV02 &&
		defenseValidationAgentEventBindsDigests(raw.ObservationEvent.SourceDigests, receiptDigest, proofDigest) {
		when := time.UnixMilli(raw.ObservationEvent.Window.ToUnixMS).UTC()
		observedAt = &when
		independentState = services.AgentIndependentObservationVerified
		effectStatus = services.IntelligenceEvidenceVerified
		effectConfidence = 1
		if eventDigest := strings.ToLower(strings.TrimSpace(raw.ObservationEvent.EventSHA256)); eventDigest != "" {
			effectRefs = append(effectRefs, "security-evidence:"+eventDigest)
		}
	}

	stages := []services.AgentExecutionStageEvidence{
		{
			Stage:              services.AgentExecutionStageAuthorization,
			ActionID:           actionID,
			PolicyRef:          "executionproof.authorization.signing_policy",
			PolicyDigestSHA256: materialBinding.AuthorizationPolicySHA256,
			ArtifactRef:        materialBinding.AuthorizationRef,
			Outcome:            strings.ToLower(string(raw.ExecutionProof.Evaluation.Decision)),
			Status:             services.IntelligenceEvidenceVerified,
			EvidenceRefs:       []string{proofRef, bindingRef},
			Confidence:         1,
		},
		{
			Stage:              services.AgentExecutionStageEnforcement,
			ActionID:           actionID,
			PolicyRef:          "executionproof.runtime.policy",
			PolicyDigestSHA256: materialBinding.RuntimePolicySHA256,
			Outcome:            "policy_evidence_observed",
			Status:             services.IntelligenceEvidenceObserved,
			EvidenceRefs:       []string{proofRef, bindingRef},
			Confidence:         0.8,
		},
		{
			Stage:                services.AgentExecutionStageExecution,
			ActionID:             actionID,
			ArtifactRef:          "executioncontainment.action",
			ArtifactDigestSHA256: actionID,
			Outcome:              strings.ToLower(string(raw.ContainmentReceipt.Decision)),
			Status:               services.IntelligenceEvidenceVerified,
			EvidenceRefs:         []string{proofRef, receiptRef, bindingRef},
			Confidence:           1,
		},
		{
			Stage:                services.AgentExecutionStageEffect,
			ActionID:             actionID,
			ArtifactRef:          "executioncontainment.effect_set",
			ArtifactDigestSHA256: raw.ContainmentReceipt.Observation.EffectSetSHA256,
			Outcome:              defenseValidationAgentEffectOutcome(raw.ContainmentReceipt),
			Status:               effectStatus,
			ObservedAt:           observedAt,
			EvidenceRefs:         effectRefs,
			Confidence:           effectConfidence,
		},
	}

	return services.BuildAgentExecutionEvidenceTrace(
		"", // This validation corpus proves no agent identity.
		materialBinding.IntentSHA256,
		independentState,
		receiptRef,
		receiptDigest,
		stages,
		generatedAt,
	), nil
}

func defenseValidationAgentProofMatches(proof executionproof.Proof) bool {
	recomputed, err := executionproof.Evaluate(proof.Envelope)
	if err != nil || !strings.EqualFold(recomputed.EnvelopeSHA256, proof.EnvelopeSHA256) || recomputed.Evaluation.Decision != proof.Evaluation.Decision {
		return false
	}
	if len(recomputed.Evaluation.Reasons) != len(proof.Evaluation.Reasons) {
		return false
	}
	for index := range recomputed.Evaluation.Reasons {
		if recomputed.Evaluation.Reasons[index] != proof.Evaluation.Reasons[index] {
			return false
		}
	}
	return true
}

func defenseValidationAgentEventBindsDigests(sourceDigests []string, required ...string) bool {
	available := make(map[string]struct{}, len(sourceDigests))
	for _, digest := range sourceDigests {
		digest = strings.ToLower(strings.TrimSpace(digest))
		if digest != "" {
			available[digest] = struct{}{}
		}
	}
	for _, digest := range required {
		if _, ok := available[strings.ToLower(strings.TrimSpace(digest))]; !ok {
			return false
		}
	}
	return true
}

func defenseValidationAgentEffectOutcome(receipt executioncontainment.Receipt) string {
	if !receipt.Observation.BackendAvailable {
		return "unavailable"
	}
	if !receipt.Observation.ExecutionPathFullyObserved {
		return "partially_observed"
	}
	return "observed"
}
