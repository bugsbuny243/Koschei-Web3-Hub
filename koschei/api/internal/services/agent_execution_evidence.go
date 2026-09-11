package services

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

const (
	AgentExecutionEvidenceContractVersion = "koschei-agent-execution-evidence-v1"

	AgentExecutionStageIdentity      = "identity"
	AgentExecutionStageDelegation    = "delegation"
	AgentExecutionStageAuthorization = "authorization"
	AgentExecutionStageEnforcement   = "enforcement"
	AgentExecutionStageExecution     = "execution"
	AgentExecutionStageEffect        = "effect"

	AgentIndependentObservationVerified    = "VERIFIED"
	AgentIndependentObservationPartial     = "PARTIAL"
	AgentIndependentObservationUnverified  = "UNVERIFIED"
	AgentIndependentObservationUnavailable = "UNAVAILABLE"
)

var requiredAgentExecutionStages = []string{
	AgentExecutionStageIdentity,
	AgentExecutionStageDelegation,
	AgentExecutionStageAuthorization,
	AgentExecutionStageEnforcement,
	AgentExecutionStageExecution,
	AgentExecutionStageEffect,
}

// AgentExecutionStageEvidence keeps each step in the agent security chain
// independently evidence-bound. A verified authorization stage therefore does
// not imply that enforcement, execution, or the requested effect occurred.
type AgentExecutionStageEvidence struct {
	Stage                string     `json:"stage"`
	SubjectID            string     `json:"subject_id,omitempty"`
	ActionID             string     `json:"action_id,omitempty"`
	PolicyRef            string     `json:"policy_ref,omitempty"`
	PolicyDigestSHA256   string     `json:"policy_digest_sha256,omitempty"`
	ArtifactRef          string     `json:"artifact_ref,omitempty"`
	ArtifactDigestSHA256 string     `json:"artifact_digest_sha256,omitempty"`
	Status               string     `json:"status"`
	ObservedAt           *time.Time `json:"observed_at,omitempty"`
	EvidenceRefs         []string   `json:"evidence_refs,omitempty"`
	Confidence           float64    `json:"confidence"`
}

// AgentExecutionEvidenceTrace is an additive evidence contract for agent-origin
// actions that may cross tools, services, and Web3 state. It is not an
// authorization or safety decision. TraceStatus only reports evidence
// completeness across the required stages.
type AgentExecutionEvidenceTrace struct {
	ContractVersion             string                        `json:"contract_version"`
	ID                          string                        `json:"id"`
	ActorSubjectID              string                        `json:"actor_subject_id"`
	IntentRef                   string                        `json:"intent_ref,omitempty"`
	Stages                      []AgentExecutionStageEvidence `json:"stages"`
	IndependentObservationState string                        `json:"independent_observation_state"`
	ReceiptRef                  string                        `json:"receipt_ref,omitempty"`
	ReceiptDigestSHA256         string                        `json:"receipt_digest_sha256,omitempty"`
	TraceStatus                 string                        `json:"trace_status"`
	GeneratedAt                 time.Time                     `json:"generated_at"`
}

func BuildAgentExecutionEvidenceTrace(actorSubjectID, intentRef, independentObservationState, receiptRef, receiptDigest string, stages []AgentExecutionStageEvidence, now time.Time) AgentExecutionEvidenceTrace {
	actorSubjectID = strings.TrimSpace(actorSubjectID)
	intentRef = strings.TrimSpace(intentRef)
	receiptRef = strings.TrimSpace(receiptRef)
	receiptDigest = normalizeAgentSHA256(receiptDigest)
	if now.IsZero() {
		now = time.Now().UTC()
	}

	normalizedStages := normalizeAgentExecutionStages(stages)
	observationState := normalizeAgentIndependentObservation(independentObservationState)
	if observationState == AgentIndependentObservationVerified && receiptDigest == "" {
		observationState = AgentIndependentObservationUnverified
	}

	traceStatus := deriveAgentExecutionTraceStatus(actorSubjectID, normalizedStages, observationState, receiptDigest)
	canonical := []string{actorSubjectID, intentRef, observationState, receiptDigest}
	for _, stage := range normalizedStages {
		canonical = append(canonical,
			stage.Stage,
			stage.SubjectID,
			stage.ActionID,
			stage.PolicyRef,
			stage.PolicyDigestSHA256,
			stage.ArtifactRef,
			stage.ArtifactDigestSHA256,
			stage.Status,
		)
	}

	return AgentExecutionEvidenceTrace{
		ContractVersion:             AgentExecutionEvidenceContractVersion,
		ID:                          intelligenceTypedStableID("kae_", strings.Join(canonical, "|")),
		ActorSubjectID:              actorSubjectID,
		IntentRef:                   intentRef,
		Stages:                      normalizedStages,
		IndependentObservationState: observationState,
		ReceiptRef:                  receiptRef,
		ReceiptDigestSHA256:         receiptDigest,
		TraceStatus:                 traceStatus,
		GeneratedAt:                 now.UTC(),
	}
}

func normalizeAgentExecutionStages(stages []AgentExecutionStageEvidence) []AgentExecutionStageEvidence {
	byStage := make(map[string]AgentExecutionStageEvidence, len(stages))
	for _, stage := range stages {
		kind := strings.ToLower(strings.TrimSpace(stage.Stage))
		if !isRequiredAgentExecutionStage(kind) {
			continue
		}
		stage.Stage = kind
		stage.SubjectID = strings.TrimSpace(stage.SubjectID)
		stage.ActionID = strings.TrimSpace(stage.ActionID)
		stage.PolicyRef = strings.TrimSpace(stage.PolicyRef)
		stage.PolicyDigestSHA256 = normalizeAgentSHA256(stage.PolicyDigestSHA256)
		stage.ArtifactRef = strings.TrimSpace(stage.ArtifactRef)
		stage.ArtifactDigestSHA256 = normalizeAgentSHA256(stage.ArtifactDigestSHA256)
		stage.EvidenceRefs = nonEmptyIntelligenceRefs(stage.EvidenceRefs)
		stage.Status = normalizeAgentStageStatus(stage.Status, len(stage.EvidenceRefs) > 0)
		stage.Confidence = clampIntelligenceConfidence(stage.Confidence)
		if stage.ObservedAt != nil {
			observed := stage.ObservedAt.UTC()
			stage.ObservedAt = &observed
		}
		byStage[kind] = stage
	}

	out := make([]AgentExecutionStageEvidence, 0, len(requiredAgentExecutionStages))
	for _, kind := range requiredAgentExecutionStages {
		if stage, ok := byStage[kind]; ok {
			out = append(out, stage)
			continue
		}
		out = append(out, AgentExecutionStageEvidence{Stage: kind, Status: IntelligenceEvidenceUnverified})
	}
	return out
}

func deriveAgentExecutionTraceStatus(actorSubjectID string, stages []AgentExecutionStageEvidence, observationState, receiptDigest string) string {
	if strings.TrimSpace(actorSubjectID) == "" {
		return IntelligenceEvidenceUnverified
	}
	allVerified := len(stages) == len(requiredAgentExecutionStages)
	allObservedOrBetter := allVerified
	for _, stage := range stages {
		if stage.Status != IntelligenceEvidenceVerified {
			allVerified = false
		}
		if stage.Status != IntelligenceEvidenceVerified && stage.Status != IntelligenceEvidenceObserved {
			allObservedOrBetter = false
		}
	}
	if allVerified && observationState == AgentIndependentObservationVerified && receiptDigest != "" {
		return IntelligenceEvidenceVerified
	}
	if allObservedOrBetter {
		return IntelligenceEvidenceObserved
	}
	return IntelligenceEvidenceUnverified
}

func normalizeAgentStageStatus(status string, hasEvidence bool) string {
	if !hasEvidence {
		return IntelligenceEvidenceUnverified
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case IntelligenceEvidenceVerified:
		return IntelligenceEvidenceVerified
	case IntelligenceEvidenceObserved:
		return IntelligenceEvidenceObserved
	case IntelligenceEvidenceInferred:
		return IntelligenceEvidenceInferred
	default:
		return IntelligenceEvidenceUnverified
	}
}

func normalizeAgentIndependentObservation(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case AgentIndependentObservationVerified:
		return AgentIndependentObservationVerified
	case AgentIndependentObservationPartial:
		return AgentIndependentObservationPartial
	case AgentIndependentObservationUnavailable:
		return AgentIndependentObservationUnavailable
	default:
		return AgentIndependentObservationUnverified
	}
}

func isRequiredAgentExecutionStage(stage string) bool {
	for _, required := range requiredAgentExecutionStages {
		if stage == required {
			return true
		}
	}
	return false
}

func normalizeAgentSHA256(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != sha256.Size*2 {
		return ""
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return ""
	}
	return value
}
