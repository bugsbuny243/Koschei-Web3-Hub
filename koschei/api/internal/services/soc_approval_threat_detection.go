package services

import (
	"math/big"
	"strings"

	"koschei/api/internal/networktarget"
)

const SOCApprovalThreatSchemaVersion = "koschei.soc-approval-threat-observation.v1"
const SOCApprovalThreatPlaybookID = "SOC-PB-03"

// SOCApprovalThreatAssessment is an actionable *observation*, never an
// authorization verdict or an enforcement receipt. It is derived from the
// live, chain-ID-verified ERC-20 allowance and spender-code/proxy RPC probes.
// It cannot claim owner intent, allowance provenance or a malicious actor.
type SOCApprovalThreatAssessment struct {
	SchemaVersion     string   `json:"schema_version"`
	PlaybookID        string   `json:"playbook_id"`
	ExecutionState    string   `json:"execution_state"`
	EvidenceState     string   `json:"evidence_state"`
	FindingState      string   `json:"finding_state"`
	Severity          string   `json:"severity"`
	RecommendedAction string   `json:"recommended_action"`
	EnforcementState  string   `json:"enforcement_state"`
	ReportIntegrity   string   `json:"report_integrity"`
	ReasonCodes       []string `json:"reason_codes"`
}

// DetectSOCApprovalThreat binds a real observed allowance to independently
// observed spender control for a single token/owner/spender pair. Nil or
// untrusted authority does not become "no threat". The strongest state is an
// exposure requiring review: a legitimate owner can intentionally authorize
// a delegated or upgradeable spender.
func DetectSOCApprovalThreat(allowance networktarget.EVMAllowanceProbeResult, authority *EVMSpenderAuthoritySnapshot) SOCApprovalThreatAssessment {
	result := SOCApprovalThreatAssessment{
		SchemaVersion:     SOCApprovalThreatSchemaVersion,
		PlaybookID:        SOCApprovalThreatPlaybookID,
		ExecutionState:    "read_only_completed",
		EvidenceState:     "incomplete",
		FindingState:      "withheld",
		Severity:          "unknown",
		RecommendedAction: "complete_evidence_and_review",
		EnforcementState:  "not_connected",
		ReportIntegrity:   "unsigned_read_only_observation",
		ReasonCodes:       []string{},
	}
	// Never trust caller-supplied Unlimited flags without the exact ABI number.
	number := new(big.Int)
	_, supported := networktarget.ExpectedEVMChainID(allowance.Network)
	token, tokenErr := normalizeEVMApprovalAddress(allowance.Token)
	owner, ownerErr := normalizeEVMApprovalAddress(allowance.Owner)
	spender, spenderErr := normalizeEVMApprovalAddress(allowance.Spender)
	_, numeric := number.SetString(strings.TrimSpace(allowance.Amount), 10)
	if !supported || tokenErr != nil || ownerErr != nil || spenderErr != nil ||
		token != allowance.Token || owner != allowance.Owner || spender != allowance.Spender ||
		allowance.EvidenceStatus != "observed" || allowance.LiveAvailability != "checked" ||
		!numeric || number.Sign() < 0 || number.Cmp(maxUint256) > 0 ||
		allowance.Unlimited != (number.Cmp(maxUint256) == 0) {
		result.ExecutionState = "incomplete"
		result.EvidenceState = "conflicting_or_unverified"
		result.ReasonCodes = append(result.ReasonCodes, "ALLOWANCE_EVIDENCE_INVALID")
		return result
	}
	if authority == nil {
		result.ReasonCodes = append(result.ReasonCodes, "SPENDER_AUTHORITY_EVIDENCE_MISSING")
		return result
	}
	if authority.Network != allowance.Network || authority.Spender != spender ||
		!authority.Trust.Observed || authority.Trust.Authorized || authority.Trust.Verified ||
		authority.Trust.Finalized ||
		(authority.DelegationState != networktarget.EVMDelegationStateObserved &&
			authority.DelegationState != networktarget.EVMDelegationStateNotObserved) ||
		(authority.ProxyState != networktarget.EVMProxyAuthorityObserved &&
			authority.ProxyState != networktarget.EVMProxyAuthorityNotObserved) ||
		(authority.DelegationState == networktarget.EVMDelegationStateObserved && authority.DelegationTarget == "") ||
		(authority.ProxyState == networktarget.EVMProxyAuthorityObserved &&
			authority.ProxyImplementation == "" && authority.ProxyAdmin == "" && authority.ProxyBeacon == "") {
		result.ExecutionState = "incomplete"
		result.EvidenceState = "conflicting_or_unverified"
		result.ReasonCodes = append(result.ReasonCodes, "SPENDER_AUTHORITY_EVIDENCE_INVALID")
		return result
	}
	result.EvidenceState = "observed_latest_unpinned"
	result.RecommendedAction = "review_authorization"
	if number.Sign() == 0 {
		result.FindingState = "no_active_allowance_for_pair_observed"
		result.Severity = "none"
		result.ReasonCodes = append(result.ReasonCodes, "ZERO_ALLOWANCE_FOR_QUERIED_PAIR")
		return result
	}
	delegated := authority.DelegationState == networktarget.EVMDelegationStateObserved
	upgradeable := authority.ProxyState == networktarget.EVMProxyAuthorityObserved
	if allowance.Unlimited {
		result.ReasonCodes = append(result.ReasonCodes, "UNLIMITED_ERC20_ALLOWANCE_OBSERVED")
	}
	if delegated {
		result.ReasonCodes = append(result.ReasonCodes, "EIP7702_SPENDER_DELEGATION_OBSERVED")
	}
	if upgradeable {
		result.ReasonCodes = append(result.ReasonCodes, "ERC1967_SPENDER_UPGRADE_AUTHORITY_OBSERVED")
	}
	// Severity describes observed *exposure* only, not intent or exploitation.
	switch {
	case allowance.Unlimited && (delegated || upgradeable):
		result.FindingState = "unlimited_allowance_with_mutable_spender_authority"
		result.Severity = "high"
		result.RecommendedAction = "urgent_owner_review_and_revoke_if_unauthorized"
	case allowance.Unlimited:
		result.FindingState = "unlimited_allowance_observed"
		result.Severity = "medium"
		result.RecommendedAction = "review_spender_and_revoke_if_unauthorized"
	case delegated || upgradeable:
		result.FindingState = "nonzero_allowance_with_mutable_spender_authority"
		result.Severity = "medium"
		result.RecommendedAction = "review_spender_control_and_allowance"
	default:
		result.FindingState = "no_compound_spender_exposure_observed"
		result.Severity = "none"
	}
	return result
}
