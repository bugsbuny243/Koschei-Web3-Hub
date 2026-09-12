package services

import (
	"errors"
	"strings"

	"koschei/api/internal/networktarget"
)

const (
	CustomerScanStatusNeedsContext         = "needs_context"
	CustomerScanStatusInsufficientEvidence = "insufficient_evidence"
	CustomerScanStatusObserved             = "observed"
	CustomerScanStatusEvidenceReady        = "evidence_ready"

	CustomerScanVerdictUnknown = "unknown"
	CustomerScanVerdictReview  = "review"
)

type CustomerScanResult struct {
	Target         CustomerScanTarget           `json:"target"`
	Status         string                       `json:"status"`
	Verdict        string                       `json:"verdict"`
	EvidenceStatus string                       `json:"evidence_status"`
	Trust          Web3TrustVector              `json:"trust"`
	Reasons        []string                     `json:"reasons,omitempty"`
	EvidenceRefs   []string                     `json:"evidence_refs,omitempty"`
	EVMAuthority   *EVMSpenderAuthoritySnapshot `json:"evm_authority,omitempty"`
}

// BuildCustomerScanResult builds the customer-facing evidence envelope. It is
// deliberately conservative: evidence maturity alone never becomes a "safe"
// verdict. Risk/safety verdicts must be produced by a separate evidence-backed
// decision layer.
func BuildCustomerScanResult(target CustomerScanTarget, trust Web3TrustVector, evidenceRefs []string) (CustomerScanResult, error) {
	if strings.TrimSpace(target.Raw) == "" {
		return CustomerScanResult{}, errors.New("classified scan target is required")
	}
	if err := ValidateWeb3TrustVector(trust); err != nil {
		return CustomerScanResult{}, err
	}

	reasons := append([]string{}, target.Reasons...)
	reasons = append(reasons, trust.Reasons...)
	result := CustomerScanResult{
		Target:         target,
		Verdict:        CustomerScanVerdictUnknown,
		EvidenceStatus: Web3TrustEvidenceStatus(trust),
		Trust:          trust,
		Reasons:        NormalizeWeb3TrustReasons(reasons),
		EvidenceRefs:   nonEmptyIntelligenceRefs(evidenceRefs),
	}

	switch {
	case target.Route == CustomerScanRouteUnresolved:
		result.Status = CustomerScanStatusInsufficientEvidence
	case target.RequiresNetwork:
		result.Status = CustomerScanStatusNeedsContext
	case trust.Verified:
		result.Status = CustomerScanStatusEvidenceReady
		result.Verdict = CustomerScanVerdictReview
	case trust.Observed:
		result.Status = CustomerScanStatusObserved
		result.Verdict = CustomerScanVerdictReview
	default:
		result.Status = CustomerScanStatusInsufficientEvidence
	}
	return result, nil
}

// CustomerScanResultFromNetworkProbe converts an already-adapted read-only
// network probe into a customer result. A network probe contributes observed
// evidence only; it does not manufacture authorization, verification, finality
// or a safety verdict. EIP-7702 delegation state, when present in the evidence,
// is surfaced as an observation reason only.
func CustomerScanResultFromNetworkProbe(target CustomerScanTarget, projection NetworkProbeIntelligenceProjection) (CustomerScanResult, error) {
	if target.Route != CustomerScanRouteEVMProbe && target.Route != CustomerScanRouteBitcoinProbe {
		return CustomerScanResult{}, errors.New("network probe result requires EVM or Bitcoin scan route")
	}
	if target.RequiresNetwork {
		return CustomerScanResult{}, errors.New("network context is required before network probe projection")
	}
	if strings.TrimSpace(projection.Subject.Raw) != strings.TrimSpace(target.Raw) {
		return CustomerScanResult{}, errors.New("network probe subject does not match customer target")
	}
	if projection.Evidence.Status != IntelligenceEvidenceObserved {
		return CustomerScanResult{}, errors.New("observed network probe evidence is required")
	}
	if projection.Evidence.SubjectID == "" || projection.Evidence.SubjectID != projection.Subject.ID {
		return CustomerScanResult{}, errors.New("network probe evidence is not bound to the projected subject")
	}

	reasons := []string{"READ_ONLY_NETWORK_OBSERVATION"}
	if state, ok := projection.Evidence.Attributes["delegation_state"].(string); ok && strings.TrimSpace(state) == networktarget.EVMDelegationStateObserved {
		reasons = append(reasons, "EIP7702_DELEGATION_OBSERVED")
	}
	trust := Web3TrustVector{
		Observed: true,
		Reasons:  reasons,
	}
	return BuildCustomerScanResult(target, trust, []string{projection.Evidence.ID})
}

// CustomerScanResultFromEVMAuthority attaches a current EVM authority snapshot
// to an observed EVM customer scan. The authority snapshot adds evidence and
// reason codes only; it never promotes authorization, verification, finality,
// or a safety verdict.
func CustomerScanResultFromEVMAuthority(target CustomerScanTarget, projection NetworkProbeIntelligenceProjection, authority EVMSpenderAuthoritySnapshot) (CustomerScanResult, error) {
	if target.Route != CustomerScanRouteEVMProbe {
		return CustomerScanResult{}, errors.New("EVM authority requires EVM scan route")
	}
	result, err := CustomerScanResultFromNetworkProbe(target, projection)
	if err != nil {
		return CustomerScanResult{}, err
	}
	if strings.TrimSpace(authority.Network) != strings.TrimSpace(target.NetworkHint) {
		return CustomerScanResult{}, errors.New("EVM authority network does not match customer target")
	}
	if strings.ToLower(strings.TrimSpace(authority.Spender)) != strings.ToLower(strings.TrimSpace(target.Raw)) {
		return CustomerScanResult{}, errors.New("EVM authority subject does not match customer target")
	}
	if err := ValidateWeb3TrustVector(authority.Trust); err != nil || !authority.Trust.Observed {
		return CustomerScanResult{}, errors.New("observed EVM authority evidence is required")
	}
	result.Reasons = NormalizeWeb3TrustReasons(append(result.Reasons, authority.Reasons...))
	result.Trust.Reasons = append([]string(nil), result.Reasons...)
	result.EVMAuthority = &authority
	return result, nil
}
