package services

import (
	"strings"
	"time"
)

const (
	UnifiedSecurityContractVersion = "koschei-unified-security-contract-v1"

	UnifiedSecurityDomainBlockchain       = "blockchain"
	UnifiedSecurityDomainAIAgent          = "ai_agent"
	UnifiedSecurityDomainIdentity         = "identity"
	UnifiedSecurityDomainInformationModel = "information_model"
	UnifiedSecurityDomainVirtualWorld     = "virtual_world"
	UnifiedSecurityDomainMachineEconomy   = "machine_economy"
	UnifiedSecurityDomainPostQuantum      = "post_quantum"
	UnifiedSecurityDomainProtocol         = "protocol"
	UnifiedSecurityDomainUnknown          = "unknown"

	IntelligenceSubjectAgent      = "agent"
	IntelligenceSubjectIdentity   = "identity"
	IntelligenceSubjectDevice     = "device"
	IntelligenceSubjectContract   = "contract"
	IntelligenceSubjectTreasury   = "treasury"
	IntelligenceSubjectOracle     = "oracle"
	IntelligenceSubjectCredential = "credential"
	IntelligenceSubjectAsset      = "asset"

	IntelligenceCapabilitySigner        = "signer"
	IntelligenceCapabilityToolUse       = "tool_use"
	IntelligenceCapabilityAPIAccess     = "api_access"
	IntelligenceCapabilitySpend         = "spend"
	IntelligenceCapabilityDelegate      = "delegate"
	IntelligenceCapabilityDeviceCommand = "device_command"
	IntelligenceCapabilityCredential    = "credential"
	IntelligenceCapabilityGovernance    = "governance"

	IntelligenceCapabilityLifecycleActive  = "active"
	IntelligenceCapabilityLifecycleRevoked = "revoked"
	IntelligenceCapabilityLifecycleExpired = "expired"
	IntelligenceCapabilityLifecycleUnknown = "unknown"
)

// IntelligenceCapability describes a bounded authority held by a subject.
// It does not imply that the authority is safe, legitimate, or currently active.
// Status is the evidence status; Lifecycle is the observed authority lifecycle.
type IntelligenceCapability struct {
	ID                   string     `json:"id"`
	SubjectID            string     `json:"subject_id"`
	Domain               string     `json:"domain"`
	Kind                 string     `json:"kind"`
	Scope                string     `json:"scope"`
	ResourceRef          string     `json:"resource_ref,omitempty"`
	DelegatedBySubjectID string     `json:"delegated_by_subject_id,omitempty"`
	Constraints          []string   `json:"constraints,omitempty"`
	Lifecycle            string     `json:"lifecycle"`
	Status               string     `json:"status"`
	ValidFrom            *time.Time `json:"valid_from,omitempty"`
	ExpiresAt            *time.Time `json:"expires_at,omitempty"`
	RevokedAt            *time.Time `json:"revoked_at,omitempty"`
	EvidenceRefs         []string   `json:"evidence_refs,omitempty"`
	Confidence           float64    `json:"confidence"`
}

// IntelligenceAction is an evidence-linked operation performed or attempted by
// a subject. It may be an on-chain transaction, agent tool call, API request,
// credential operation, machine command, governance action, or another adapter-
// supplied operation that can affect the Web3/digital-economy trust graph.
type IntelligenceAction struct {
	ID              string     `json:"id"`
	SubjectID       string     `json:"subject_id"`
	CapabilityID    string     `json:"capability_id,omitempty"`
	TargetSubjectID string     `json:"target_subject_id,omitempty"`
	Domain          string     `json:"domain"`
	Kind            string     `json:"kind"`
	ResourceRef     string     `json:"resource_ref,omitempty"`
	TransactionHash string     `json:"transaction_hash,omitempty"`
	StateChange     string     `json:"state_change,omitempty"`
	ObservedAt      *time.Time `json:"observed_at,omitempty"`
	Status          string     `json:"status"`
	EvidenceRefs    []string   `json:"evidence_refs,omitempty"`
	Confidence      float64    `json:"confidence"`
}

// IntelligenceTrustBoundaryTransition records movement from one security
// domain to another, for example AI agent -> treasury API -> blockchain.
// The transition is an evidence object, not a claim that either side of the
// boundary is implemented inside Koschei Web3.
type IntelligenceTrustBoundaryTransition struct {
	ID           string   `json:"id"`
	SubjectID    string   `json:"subject_id"`
	CapabilityID string   `json:"capability_id,omitempty"`
	ActionID     string   `json:"action_id,omitempty"`
	FromDomain   string   `json:"from_domain"`
	ToDomain     string   `json:"to_domain"`
	Mechanism    string   `json:"mechanism"`
	Status       string   `json:"status"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	Confidence   float64  `json:"confidence"`
}

// IntelligenceConsequence describes an evidence-backed or explicitly inferred
// effect. It intentionally avoids manufacturing a universal numeric risk score.
type IntelligenceConsequence struct {
	ID           string   `json:"id"`
	Domain       string   `json:"domain"`
	Kind         string   `json:"kind"`
	AssetRef     string   `json:"asset_ref,omitempty"`
	Summary      string   `json:"summary"`
	StateChange  string   `json:"state_change,omitempty"`
	Status       string   `json:"status"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	Confidence   float64  `json:"confidence"`
}

type UnifiedSecurityAttackPathStep struct {
	Order                int      `json:"order"`
	SubjectID            string   `json:"subject_id"`
	CapabilityID         string   `json:"capability_id,omitempty"`
	ActionID             string   `json:"action_id,omitempty"`
	TargetSubjectID      string   `json:"target_subject_id,omitempty"`
	BoundaryTransitionID string   `json:"boundary_transition_id,omitempty"`
	Effect               string   `json:"effect"`
	EvidenceRefs         []string `json:"evidence_refs,omitempty"`
}

type UnifiedSecurityAttackPath struct {
	ContractVersion string                          `json:"contract_version"`
	ID              string                          `json:"id"`
	Title           string                          `json:"title"`
	EntrySubjectID  string                          `json:"entry_subject_id"`
	Status          string                          `json:"status"`
	Preconditions   []string                        `json:"preconditions,omitempty"`
	Steps           []UnifiedSecurityAttackPathStep `json:"steps"`
	Consequences    []IntelligenceConsequence       `json:"consequences,omitempty"`
	EvidenceRefs    []string                        `json:"evidence_refs,omitempty"`
	Confidence      float64                         `json:"confidence"`
}

// UnifiedSecurityInvestigation is an additive envelope around the existing
// chain-neutral intelligence contract. The base investigation remains the
// authoritative ARVIS projection; this envelope adds capability/action/boundary
// semantics without re-grading the existing decision.
type UnifiedSecurityInvestigation struct {
	ContractVersion     string                                `json:"contract_version"`
	Base                IntelligenceInvestigation             `json:"base"`
	Capabilities        []IntelligenceCapability              `json:"capabilities,omitempty"`
	Actions             []IntelligenceAction                  `json:"actions,omitempty"`
	BoundaryTransitions []IntelligenceTrustBoundaryTransition `json:"boundary_transitions,omitempty"`
	AttackPaths         []UnifiedSecurityAttackPath           `json:"attack_paths,omitempty"`
	GeneratedAt         time.Time                             `json:"generated_at"`
}

func BuildIntelligenceCapability(subjectID, domain, kind, scope, resourceRef, delegatedBySubjectID, lifecycle, requestedStatus string, constraints, evidenceRefs []string, confidence float64) IntelligenceCapability {
	subjectID = strings.TrimSpace(subjectID)
	domain = normalizeUnifiedSecurityDomain(domain)
	kind = strings.TrimSpace(kind)
	scope = strings.TrimSpace(scope)
	resourceRef = strings.TrimSpace(resourceRef)
	delegatedBySubjectID = strings.TrimSpace(delegatedBySubjectID)
	constraints = nonEmptyIntelligenceRefs(constraints)
	evidenceRefs = nonEmptyIntelligenceRefs(evidenceRefs)
	lifecycle = normalizeCapabilityLifecycle(lifecycle)
	complete := subjectID != "" && domain != UnifiedSecurityDomainUnknown && kind != "" && scope != ""
	status := normalizeEvidenceBoundStatus(requestedStatus, complete, evidenceRefs)
	canonical := strings.Join([]string{subjectID, domain, kind, scope, resourceRef, delegatedBySubjectID}, "|")
	return IntelligenceCapability{
		ID:                   intelligenceTypedStableID("kic_", canonical),
		SubjectID:            subjectID,
		Domain:               domain,
		Kind:                 kind,
		Scope:                scope,
		ResourceRef:          resourceRef,
		DelegatedBySubjectID: delegatedBySubjectID,
		Constraints:          constraints,
		Lifecycle:            lifecycle,
		Status:               status,
		EvidenceRefs:         evidenceRefs,
		Confidence:           clampIntelligenceConfidence(confidence),
	}
}

func BuildIntelligenceAction(subjectID, capabilityID, targetSubjectID, domain, kind, resourceRef, transactionHash, stateChange, requestedStatus string, evidenceRefs []string, confidence float64) IntelligenceAction {
	subjectID = strings.TrimSpace(subjectID)
	capabilityID = strings.TrimSpace(capabilityID)
	targetSubjectID = strings.TrimSpace(targetSubjectID)
	domain = normalizeUnifiedSecurityDomain(domain)
	kind = strings.TrimSpace(kind)
	resourceRef = strings.TrimSpace(resourceRef)
	transactionHash = strings.TrimSpace(transactionHash)
	stateChange = strings.TrimSpace(stateChange)
	evidenceRefs = nonEmptyIntelligenceRefs(evidenceRefs)
	complete := subjectID != "" && domain != UnifiedSecurityDomainUnknown && kind != ""
	status := normalizeEvidenceBoundStatus(requestedStatus, complete, evidenceRefs)
	canonical := strings.Join([]string{subjectID, capabilityID, targetSubjectID, domain, kind, resourceRef, transactionHash, stateChange}, "|")
	return IntelligenceAction{
		ID:              intelligenceTypedStableID("kia_", canonical),
		SubjectID:       subjectID,
		CapabilityID:    capabilityID,
		TargetSubjectID: targetSubjectID,
		Domain:          domain,
		Kind:            kind,
		ResourceRef:     resourceRef,
		TransactionHash: transactionHash,
		StateChange:     stateChange,
		Status:          status,
		EvidenceRefs:    evidenceRefs,
		Confidence:      clampIntelligenceConfidence(confidence),
	}
}

func BuildTrustBoundaryTransition(subjectID, capabilityID, actionID, fromDomain, toDomain, mechanism, requestedStatus string, evidenceRefs []string, confidence float64) IntelligenceTrustBoundaryTransition {
	subjectID = strings.TrimSpace(subjectID)
	capabilityID = strings.TrimSpace(capabilityID)
	actionID = strings.TrimSpace(actionID)
	fromDomain = normalizeUnifiedSecurityDomain(fromDomain)
	toDomain = normalizeUnifiedSecurityDomain(toDomain)
	mechanism = strings.TrimSpace(mechanism)
	evidenceRefs = nonEmptyIntelligenceRefs(evidenceRefs)
	complete := subjectID != "" && fromDomain != UnifiedSecurityDomainUnknown && toDomain != UnifiedSecurityDomainUnknown && mechanism != ""
	status := normalizeEvidenceBoundStatus(requestedStatus, complete, evidenceRefs)
	canonical := strings.Join([]string{subjectID, capabilityID, actionID, fromDomain, toDomain, mechanism}, "|")
	return IntelligenceTrustBoundaryTransition{
		ID:           intelligenceTypedStableID("kit_", canonical),
		SubjectID:    subjectID,
		CapabilityID: capabilityID,
		ActionID:     actionID,
		FromDomain:   fromDomain,
		ToDomain:     toDomain,
		Mechanism:    mechanism,
		Status:       status,
		EvidenceRefs: evidenceRefs,
		Confidence:   clampIntelligenceConfidence(confidence),
	}
}

func BuildIntelligenceConsequence(domain, kind, assetRef, summary, stateChange, requestedStatus string, evidenceRefs []string, confidence float64) IntelligenceConsequence {
	domain = normalizeUnifiedSecurityDomain(domain)
	kind = strings.TrimSpace(kind)
	assetRef = strings.TrimSpace(assetRef)
	summary = strings.TrimSpace(summary)
	stateChange = strings.TrimSpace(stateChange)
	evidenceRefs = nonEmptyIntelligenceRefs(evidenceRefs)
	complete := domain != UnifiedSecurityDomainUnknown && kind != "" && summary != ""
	status := normalizeEvidenceBoundStatus(requestedStatus, complete, evidenceRefs)
	canonical := strings.Join([]string{domain, kind, assetRef, summary, stateChange}, "|")
	return IntelligenceConsequence{
		ID:           intelligenceTypedStableID("kix_", canonical),
		Domain:       domain,
		Kind:         kind,
		AssetRef:     assetRef,
		Summary:      summary,
		StateChange:  stateChange,
		Status:       status,
		EvidenceRefs: evidenceRefs,
		Confidence:   clampIntelligenceConfidence(confidence),
	}
}

func BuildUnifiedSecurityAttackPath(title, entrySubjectID, requestedStatus string, preconditions []string, steps []UnifiedSecurityAttackPathStep, consequences []IntelligenceConsequence, evidenceRefs []string, confidence float64) UnifiedSecurityAttackPath {
	title = strings.TrimSpace(title)
	entrySubjectID = strings.TrimSpace(entrySubjectID)
	preconditions = nonEmptyIntelligenceRefs(preconditions)
	evidenceRefs = nonEmptyIntelligenceRefs(evidenceRefs)
	steps = normalizeUnifiedSecuritySteps(steps)
	status := normalizeEvidenceBoundStatus(requestedStatus, title != "" && entrySubjectID != "" && unifiedSecurityPathEvidenceComplete(steps, consequences), evidenceRefs)

	parts := []string{title, entrySubjectID}
	for _, step := range steps {
		parts = append(parts, step.SubjectID, step.CapabilityID, step.ActionID, step.TargetSubjectID, step.BoundaryTransitionID, step.Effect)
	}
	return UnifiedSecurityAttackPath{
		ContractVersion: UnifiedSecurityContractVersion,
		ID:              intelligenceTypedStableID("kip_", strings.Join(parts, "|")),
		Title:           title,
		EntrySubjectID:  entrySubjectID,
		Status:          status,
		Preconditions:   preconditions,
		Steps:           steps,
		Consequences:    append([]IntelligenceConsequence(nil), consequences...),
		EvidenceRefs:    evidenceRefs,
		Confidence:      clampIntelligenceConfidence(confidence),
	}
}

func BuildUnifiedSecurityInvestigation(base IntelligenceInvestigation, now time.Time) UnifiedSecurityInvestigation {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return UnifiedSecurityInvestigation{
		ContractVersion: UnifiedSecurityContractVersion,
		Base:            base,
		GeneratedAt:     now.UTC(),
	}
}

func normalizeEvidenceBoundStatus(requested string, complete bool, evidenceRefs []string) string {
	if !complete || len(nonEmptyIntelligenceRefs(evidenceRefs)) == 0 {
		return IntelligenceEvidenceUnverified
	}
	switch strings.ToLower(strings.TrimSpace(requested)) {
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

func normalizeCapabilityLifecycle(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case IntelligenceCapabilityLifecycleActive:
		return IntelligenceCapabilityLifecycleActive
	case IntelligenceCapabilityLifecycleRevoked:
		return IntelligenceCapabilityLifecycleRevoked
	case IntelligenceCapabilityLifecycleExpired:
		return IntelligenceCapabilityLifecycleExpired
	default:
		return IntelligenceCapabilityLifecycleUnknown
	}
}

func normalizeUnifiedSecurityDomain(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case UnifiedSecurityDomainBlockchain:
		return UnifiedSecurityDomainBlockchain
	case UnifiedSecurityDomainAIAgent:
		return UnifiedSecurityDomainAIAgent
	case UnifiedSecurityDomainIdentity:
		return UnifiedSecurityDomainIdentity
	case UnifiedSecurityDomainInformationModel:
		return UnifiedSecurityDomainInformationModel
	case UnifiedSecurityDomainVirtualWorld:
		return UnifiedSecurityDomainVirtualWorld
	case UnifiedSecurityDomainMachineEconomy:
		return UnifiedSecurityDomainMachineEconomy
	case UnifiedSecurityDomainPostQuantum:
		return UnifiedSecurityDomainPostQuantum
	case UnifiedSecurityDomainProtocol:
		return UnifiedSecurityDomainProtocol
	default:
		return UnifiedSecurityDomainUnknown
	}
}

func normalizeUnifiedSecuritySteps(steps []UnifiedSecurityAttackPathStep) []UnifiedSecurityAttackPathStep {
	out := make([]UnifiedSecurityAttackPathStep, 0, len(steps))
	for i, step := range steps {
		step.Order = i + 1
		step.SubjectID = strings.TrimSpace(step.SubjectID)
		step.CapabilityID = strings.TrimSpace(step.CapabilityID)
		step.ActionID = strings.TrimSpace(step.ActionID)
		step.TargetSubjectID = strings.TrimSpace(step.TargetSubjectID)
		step.BoundaryTransitionID = strings.TrimSpace(step.BoundaryTransitionID)
		step.Effect = strings.TrimSpace(step.Effect)
		step.EvidenceRefs = nonEmptyIntelligenceRefs(step.EvidenceRefs)
		out = append(out, step)
	}
	return out
}

func unifiedSecurityPathEvidenceComplete(steps []UnifiedSecurityAttackPathStep, consequences []IntelligenceConsequence) bool {
	if len(steps) == 0 {
		return false
	}
	for _, step := range steps {
		if step.SubjectID == "" || step.Effect == "" || len(step.EvidenceRefs) == 0 {
			return false
		}
	}
	for _, consequence := range consequences {
		if consequence.Status == IntelligenceEvidenceUnverified || len(consequence.EvidenceRefs) == 0 {
			return false
		}
	}
	return true
}

func intelligenceTypedStableID(prefix, canonical string) string {
	base := intelligenceStableID(canonical)
	return strings.TrimSpace(prefix) + strings.TrimPrefix(base, "kis_")
}
