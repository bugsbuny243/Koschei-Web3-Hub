package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/securityevidence"
)

const (
	AgentDelegationEvidenceChainContractVersionV1 = "koschei-agent-delegation-evidence-chain-v1"
	AgentDelegationChainAuthorityNotEvaluatedV1   = "not_evaluated"
	maxAgentDelegationEvidenceHopsV1              = 32
)

type AgentDelegationHopBindingV1 struct {
	ExpectedProducer               string
	TrustedPublicKey               string
	ExpectedSubject                securityevidence.Subject
	DelegatorSubjectID             string
	DelegateeSubjectID             string
	DelegationArtifactSHA256       string
	ParentDelegationArtifactSHA256 string
	IntentSHA256                   string
	ConstraintSetSHA256            string
}

type SignedAgentDelegationHopInputV1 struct {
	Event   securityevidence.Event
	Binding AgentDelegationHopBindingV1
}

type AgentDelegationHopEvidenceV1 struct {
	DelegatorSubjectID             string     `json:"delegator_subject_id"`
	DelegateeSubjectID             string     `json:"delegatee_subject_id"`
	DelegationArtifactSHA256       string     `json:"delegation_artifact_sha256"`
	ParentDelegationArtifactSHA256 string     `json:"parent_delegation_artifact_sha256,omitempty"`
	IntentSHA256                   string     `json:"intent_sha256"`
	ConstraintSetSHA256            string     `json:"constraint_set_sha256"`
	EventSHA256                    string     `json:"event_sha256"`
	Status                         string     `json:"status"`
	ObservedAt                     *time.Time `json:"observed_at,omitempty"`
	EvidenceRefs                   []string   `json:"evidence_refs"`
	Confidence                     float64    `json:"confidence"`
}

type AgentDelegationEvidenceChainV1 struct {
	ContractVersion              string                         `json:"contract_version"`
	ID                           string                         `json:"id"`
	ChainDigestSHA256            string                         `json:"chain_digest_sha256"`
	RootSubjectID                string                         `json:"root_subject_id"`
	TerminalSubjectID            string                         `json:"terminal_subject_id"`
	IntentSHA256                 string                         `json:"intent_sha256"`
	Hops                         []AgentDelegationHopEvidenceV1 `json:"hops"`
	ChainStatus                  string                         `json:"chain_status"`
	ConstraintSemanticsEvaluated bool                           `json:"constraint_semantics_evaluated"`
	AuthorityDecision            string                         `json:"authority_decision"`
}

// AdaptSignedAgentDelegationHopEvidenceV1 authenticates one delegation hop as
// evidence. It does not decide whether the delegation is sufficient authority.
func AdaptSignedAgentDelegationHopEvidenceV1(event securityevidence.Event, binding AgentDelegationHopBindingV1) (AgentDelegationHopEvidenceV1, error) {
	expectedProducer := strings.TrimSpace(binding.ExpectedProducer)
	trustedPublicKey := strings.TrimSpace(binding.TrustedPublicKey)
	delegator := strings.TrimSpace(binding.DelegatorSubjectID)
	delegatee := strings.TrimSpace(binding.DelegateeSubjectID)
	if expectedProducer == "" || trustedPublicKey == "" {
		return AgentDelegationHopEvidenceV1{}, errors.New("trusted producer identity and public key are required")
	}
	if delegator == "" || delegatee == "" || delegator == delegatee {
		return AgentDelegationHopEvidenceV1{}, errors.New("delegation hop requires distinct delegator and delegatee subjects")
	}

	expectedSubject := binding.ExpectedSubject
	expectedSubject.Chain = strings.ToLower(strings.TrimSpace(expectedSubject.Chain))
	expectedSubject.Type = strings.ToLower(strings.TrimSpace(expectedSubject.Type))
	expectedSubject.ID = strings.TrimSpace(expectedSubject.ID)
	if expectedSubject.Chain == "" || expectedSubject.Type != IntelligenceSubjectAgent || expectedSubject.ID != delegatee {
		return AgentDelegationHopEvidenceV1{}, errors.New("expected subject must bind the delegatee agent")
	}

	delegationDigest := normalizeAgentSHA256(binding.DelegationArtifactSHA256)
	parentDigest := normalizeOptionalAgentSHA256(binding.ParentDelegationArtifactSHA256)
	intentDigest := normalizeAgentSHA256(binding.IntentSHA256)
	constraintDigest := normalizeAgentSHA256(binding.ConstraintSetSHA256)
	if delegationDigest == "" || intentDigest == "" || constraintDigest == "" {
		return AgentDelegationHopEvidenceV1{}, errors.New("delegation, intent and constraint bindings require sha256 digests")
	}
	if strings.TrimSpace(binding.ParentDelegationArtifactSHA256) != "" && parentDigest == "" {
		return AgentDelegationHopEvidenceV1{}, errors.New("parent delegation binding must be sha256 when present")
	}

	eventSHA256 := strings.ToLower(strings.TrimSpace(event.EventSHA256))
	if err := event.VerifyEd25519(expectedProducer, trustedPublicKey); err != nil {
		return AgentDelegationHopEvidenceV1{}, fmt.Errorf("authenticate delegation hop evidence event: %w", err)
	}
	canonical, err := event.Canonical()
	if err != nil {
		return AgentDelegationHopEvidenceV1{}, fmt.Errorf("canonicalize delegation hop evidence event: %w", err)
	}
	canonical.EventSHA256 = eventSHA256
	if !unifiedSignedEvidenceSubjectMatches(canonical.Subject, expectedSubject) {
		return AgentDelegationHopEvidenceV1{}, errors.New("delegation event subject does not match trusted delegatee binding")
	}
	requiredDigests := []string{delegationDigest, intentDigest, constraintDigest}
	if parentDigest != "" {
		requiredDigests = append(requiredDigests, parentDigest)
	}
	if !agentEventHasSourceDigests(canonical.SourceDigests, requiredDigests...) {
		return AgentDelegationHopEvidenceV1{}, errors.New("delegation event does not bind the required chain artifacts")
	}
	finding, err := agentBindingFinding(canonical.Findings, AgentDelegationEvidenceFindingKindV1, delegationDigest)
	if err != nil {
		return AgentDelegationHopEvidenceV1{}, fmt.Errorf("delegation evidence: %w", err)
	}
	status := unifiedSignedFindingStatus(finding.State)
	return AgentDelegationHopEvidenceV1{
		DelegatorSubjectID:             delegator,
		DelegateeSubjectID:             delegatee,
		DelegationArtifactSHA256:       delegationDigest,
		ParentDelegationArtifactSHA256: parentDigest,
		IntentSHA256:                   intentDigest,
		ConstraintSetSHA256:            constraintDigest,
		EventSHA256:                    eventSHA256,
		Status:                         status,
		ObservedAt:                     agentEvidenceObservedAt(canonical.Window),
		EvidenceRefs: []string{
			"security-evidence:" + eventSHA256,
			"agent-delegation:" + delegationDigest,
			"agent-constraints:" + constraintDigest,
		},
		Confidence: unifiedSignedFindingConfidence(status),
	}, nil
}

// BuildSignedAgentDelegationEvidenceChainV1 authenticates every hop and then
// verifies hash-link and subject continuity. Constraint semantics are carried
// as evidence but deliberately not evaluated as authorization policy here.
func BuildSignedAgentDelegationEvidenceChainV1(inputs []SignedAgentDelegationHopInputV1) (AgentDelegationEvidenceChainV1, error) {
	if len(inputs) == 0 || len(inputs) > maxAgentDelegationEvidenceHopsV1 {
		return AgentDelegationEvidenceChainV1{}, fmt.Errorf("delegation chain must contain between 1 and %d hops", maxAgentDelegationEvidenceHopsV1)
	}
	hops := make([]AgentDelegationHopEvidenceV1, 0, len(inputs))
	for index, input := range inputs {
		hop, err := AdaptSignedAgentDelegationHopEvidenceV1(input.Event, input.Binding)
		if err != nil {
			return AgentDelegationEvidenceChainV1{}, fmt.Errorf("delegation hop %d: %w", index, err)
		}
		hops = append(hops, hop)
	}
	return buildAgentDelegationEvidenceChainV1(hops)
}

func buildAgentDelegationEvidenceChainV1(hops []AgentDelegationHopEvidenceV1) (AgentDelegationEvidenceChainV1, error) {
	if len(hops) == 0 || len(hops) > maxAgentDelegationEvidenceHopsV1 {
		return AgentDelegationEvidenceChainV1{}, errors.New("delegation chain hop count is invalid")
	}
	intent := normalizeAgentSHA256(hops[0].IntentSHA256)
	if intent == "" || normalizeOptionalAgentSHA256(hops[0].ParentDelegationArtifactSHA256) != "" {
		return AgentDelegationEvidenceChainV1{}, errors.New("delegation chain root must have a valid intent and no parent delegation")
	}

	seenArtifacts := make(map[string]struct{}, len(hops))
	seenSubjects := map[string]struct{}{strings.TrimSpace(hops[0].DelegatorSubjectID): {}}
	chainStatus := IntelligenceEvidenceVerified
	canonical := []string{AgentDelegationEvidenceChainContractVersionV1, intent}
	for index, hop := range hops {
		delegator := strings.TrimSpace(hop.DelegatorSubjectID)
		delegatee := strings.TrimSpace(hop.DelegateeSubjectID)
		delegationDigest := normalizeAgentSHA256(hop.DelegationArtifactSHA256)
		constraintDigest := normalizeAgentSHA256(hop.ConstraintSetSHA256)
		if delegator == "" || delegatee == "" || delegator == delegatee || delegationDigest == "" || constraintDigest == "" || normalizeAgentSHA256(hop.IntentSHA256) != intent {
			return AgentDelegationEvidenceChainV1{}, fmt.Errorf("delegation hop %d is incomplete or intent-mismatched", index)
		}
		if _, exists := seenArtifacts[delegationDigest]; exists {
			return AgentDelegationEvidenceChainV1{}, errors.New("delegation chain repeats an artifact")
		}
		seenArtifacts[delegationDigest] = struct{}{}
		if _, exists := seenSubjects[delegatee]; exists {
			return AgentDelegationEvidenceChainV1{}, errors.New("delegation chain contains a subject cycle")
		}
		seenSubjects[delegatee] = struct{}{}
		if index > 0 {
			previous := hops[index-1]
			if delegator != strings.TrimSpace(previous.DelegateeSubjectID) || normalizeAgentSHA256(hop.ParentDelegationArtifactSHA256) != normalizeAgentSHA256(previous.DelegationArtifactSHA256) {
				return AgentDelegationEvidenceChainV1{}, fmt.Errorf("delegation hop %d does not continue the prior signed hop", index)
			}
		}
		status := normalizeAgentStageStatus(hop.Status, len(hop.EvidenceRefs) > 0)
		if status != IntelligenceEvidenceVerified {
			if status != IntelligenceEvidenceObserved || chainStatus == IntelligenceEvidenceUnverified {
				chainStatus = IntelligenceEvidenceUnverified
			} else {
				chainStatus = IntelligenceEvidenceObserved
			}
		}
		canonical = append(canonical, delegator, delegatee, delegationDigest, normalizeOptionalAgentSHA256(hop.ParentDelegationArtifactSHA256), constraintDigest, strings.ToLower(strings.TrimSpace(hop.EventSHA256)), status)
	}
	payload := strings.Join(canonical, "|")
	sum := sha256.Sum256([]byte(payload))
	chainDigest := hex.EncodeToString(sum[:])
	return AgentDelegationEvidenceChainV1{
		ContractVersion:              AgentDelegationEvidenceChainContractVersionV1,
		ID:                           intelligenceTypedStableID("kadc_", payload),
		ChainDigestSHA256:            chainDigest,
		RootSubjectID:                strings.TrimSpace(hops[0].DelegatorSubjectID),
		TerminalSubjectID:            strings.TrimSpace(hops[len(hops)-1].DelegateeSubjectID),
		IntentSHA256:                 intent,
		Hops:                         append([]AgentDelegationHopEvidenceV1(nil), hops...),
		ChainStatus:                  chainStatus,
		ConstraintSemanticsEvaluated: false,
		AuthorityDecision:            AgentDelegationChainAuthorityNotEvaluatedV1,
	}, nil
}

// BindSignedAgentDelegationEvidenceChainV1 fills only the delegation stage of
// a trace whose actor identity and intent are already independently bound.
func BindSignedAgentDelegationEvidenceChainV1(trace AgentExecutionEvidenceTrace, inputs []SignedAgentDelegationHopInputV1) (AgentExecutionEvidenceTrace, error) {
	chain, err := BuildSignedAgentDelegationEvidenceChainV1(inputs)
	if err != nil {
		return AgentExecutionEvidenceTrace{}, err
	}
	if strings.TrimSpace(trace.ActorSubjectID) == "" || strings.TrimSpace(trace.ActorSubjectID) != chain.TerminalSubjectID {
		return AgentExecutionEvidenceTrace{}, errors.New("delegation chain terminal subject does not match independently bound trace actor")
	}
	if normalizeAgentSHA256(trace.IntentRef) == "" || normalizeAgentSHA256(trace.IntentRef) != chain.IntentSHA256 {
		return AgentExecutionEvidenceTrace{}, errors.New("delegation chain intent does not match execution trace intent")
	}

	refs := []string{"agent-delegation-chain:" + chain.ID}
	for _, hop := range chain.Hops {
		refs = append(refs, hop.EvidenceRefs...)
	}
	confidence := 0.0
	if chain.ChainStatus == IntelligenceEvidenceVerified {
		confidence = 1
	} else if chain.ChainStatus == IntelligenceEvidenceObserved {
		confidence = 0.8
	}
	stages := append([]AgentExecutionStageEvidence(nil), trace.Stages...)
	stages = append(stages, AgentExecutionStageEvidence{
		Stage:                AgentExecutionStageDelegation,
		SubjectID:            chain.TerminalSubjectID,
		ArtifactRef:          chain.ID,
		ArtifactDigestSHA256: chain.ChainDigestSHA256,
		Outcome:              "chain_bound",
		Status:               chain.ChainStatus,
		EvidenceRefs:         nonEmptyIntelligenceRefs(refs),
		Confidence:           confidence,
	})
	return BuildAgentExecutionEvidenceTrace(
		trace.ActorSubjectID,
		trace.IntentRef,
		trace.IndependentObservationState,
		trace.ReceiptRef,
		trace.ReceiptDigestSHA256,
		stages,
		trace.GeneratedAt,
	), nil
}

func normalizeOptionalAgentSHA256(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return normalizeAgentSHA256(value)
}
