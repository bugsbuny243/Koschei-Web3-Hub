package services

import (
	"errors"
	"strings"

	"koschei/api/internal/networktarget"
)

// EVMSpenderAuthoritySnapshot combines two independent read-only observations
// about a spender address: executable account/code state and standard ERC-1967
// proxy authority state. It is evidence only and never proves that the spender
// is safe, malicious, authorized by a token owner, or immutable.
type EVMSpenderAuthoritySnapshot struct {
	Network string `json:"network"`
	Spender string `json:"spender"`

	ContractCodeState string `json:"contract_code_state"`
	ContractCodeHash  string `json:"contract_code_sha256,omitempty"`
	DelegationState   string `json:"delegation_state"`
	DelegationTarget  string `json:"delegation_target,omitempty"`

	ProxyState          string `json:"proxy_state"`
	ProxyImplementation string `json:"proxy_implementation,omitempty"`
	ProxyAdmin          string `json:"proxy_admin,omitempty"`
	ProxyBeacon         string `json:"proxy_beacon,omitempty"`
	ConflictingSlots    bool   `json:"conflicting_slots"`

	Trust   Web3TrustVector `json:"trust"`
	Reasons []string        `json:"reasons"`
}

func BuildEVMSpenderAuthoritySnapshot(code networktarget.EVMProbeResult, proxy networktarget.EVMProxyAuthorityResult) (EVMSpenderAuthoritySnapshot, error) {
	if !code.AnalysisPerformed || code.EvidenceStatus != IntelligenceEvidenceObserved || code.LiveAvailability != "checked" {
		return EVMSpenderAuthoritySnapshot{}, errors.New("completed observed EVM code probe is required")
	}
	if !proxy.AnalysisPerformed || proxy.EvidenceStatus != IntelligenceEvidenceObserved || proxy.LiveAvailability != "checked" {
		return EVMSpenderAuthoritySnapshot{}, errors.New("completed observed ERC-1967 probe is required")
	}

	codeNetwork := strings.TrimSpace(code.Resolution.Network.ID)
	proxyNetwork := strings.TrimSpace(proxy.Resolution.Network.ID)
	if codeNetwork == "" || codeNetwork != proxyNetwork {
		return EVMSpenderAuthoritySnapshot{}, errors.New("spender authority probe network mismatch")
	}
	expectedChainID, ok := networktarget.ExpectedEVMChainID(codeNetwork)
	if !ok || strings.ToLower(strings.TrimSpace(code.ChainID)) != expectedChainID || strings.ToLower(strings.TrimSpace(proxy.ChainID)) != expectedChainID {
		return EVMSpenderAuthoritySnapshot{}, errors.New("spender authority chain identity mismatch")
	}

	spender, err := normalizeEVMApprovalAddress(code.Resolution.Address)
	if err != nil {
		return EVMSpenderAuthoritySnapshot{}, errors.New("invalid spender address in code probe")
	}
	proxySpender, err := normalizeEVMApprovalAddress(proxy.Resolution.Address)
	if err != nil || spender != proxySpender {
		return EVMSpenderAuthoritySnapshot{}, errors.New("spender authority subject mismatch")
	}

	if code.ContractCodeState != "contract_code_observed" && code.ContractCodeState != "no_contract_code_observed" {
		return EVMSpenderAuthoritySnapshot{}, errors.New("unsupported spender code state")
	}
	delegationState := strings.TrimSpace(code.DelegationState)
	if delegationState == "" {
		delegationState = networktarget.EVMDelegationStateNotObserved
	}
	if delegationState != networktarget.EVMDelegationStateNotObserved && delegationState != networktarget.EVMDelegationStateObserved {
		return EVMSpenderAuthoritySnapshot{}, errors.New("unsupported spender delegation state")
	}
	delegationTarget, err := normalizeOptionalEVMApprovalAddress(code.DelegationTarget)
	if err != nil {
		return EVMSpenderAuthoritySnapshot{}, errors.New("invalid spender delegation target")
	}
	if delegationState == networktarget.EVMDelegationStateObserved && delegationTarget == "" {
		return EVMSpenderAuthoritySnapshot{}, errors.New("observed spender delegation requires target")
	}
	if delegationState != networktarget.EVMDelegationStateObserved && delegationTarget != "" {
		return EVMSpenderAuthoritySnapshot{}, errors.New("spender delegation target requires observed delegation")
	}

	if proxy.State != networktarget.EVMProxyAuthorityNotObserved && proxy.State != networktarget.EVMProxyAuthorityObserved {
		return EVMSpenderAuthoritySnapshot{}, errors.New("unsupported ERC-1967 proxy authority state")
	}
	implementation, err := normalizeOptionalEVMApprovalAddress(proxy.Implementation)
	if err != nil {
		return EVMSpenderAuthoritySnapshot{}, errors.New("invalid proxy implementation")
	}
	admin, err := normalizeOptionalEVMApprovalAddress(proxy.Admin)
	if err != nil {
		return EVMSpenderAuthoritySnapshot{}, errors.New("invalid proxy admin")
	}
	beacon, err := normalizeOptionalEVMApprovalAddress(proxy.Beacon)
	if err != nil {
		return EVMSpenderAuthoritySnapshot{}, errors.New("invalid proxy beacon")
	}
	if proxy.State == networktarget.EVMProxyAuthorityObserved && implementation == "" && admin == "" && beacon == "" {
		return EVMSpenderAuthoritySnapshot{}, errors.New("observed ERC-1967 authority requires a populated standard slot")
	}
	if proxy.State == networktarget.EVMProxyAuthorityNotObserved && (implementation != "" || admin != "" || beacon != "") {
		return EVMSpenderAuthoritySnapshot{}, errors.New("empty ERC-1967 authority state cannot carry standard slot values")
	}
	if proxy.ConflictingSlots != (implementation != "" && beacon != "") {
		return EVMSpenderAuthoritySnapshot{}, errors.New("ERC-1967 conflicting slot flag mismatch")
	}

	reasons := []string{"READ_ONLY_SPENDER_AUTHORITY_OBSERVATION"}
	if code.ContractCodeState == "contract_code_observed" {
		reasons = append(reasons, "SPENDER_CODE_OBSERVED")
	}
	if delegationState == networktarget.EVMDelegationStateObserved {
		reasons = append(reasons, "EIP7702_SPENDER_DELEGATION_OBSERVED")
	}
	if proxy.State == networktarget.EVMProxyAuthorityObserved {
		reasons = append(reasons, "SPENDER_UPGRADE_AUTHORITY_OBSERVED")
	} else {
		// Absence of standard ERC-1967 slots is not evidence of immutability.
		reasons = append(reasons, "NO_STANDARD_PROXY_AUTHORITY_OBSERVED")
	}
	if proxy.ConflictingSlots {
		reasons = append(reasons, "ERC1967_CONFLICTING_AUTHORITY_SLOTS")
	}

	trust := Web3TrustVector{Observed: true, Reasons: append([]string(nil), reasons...)}
	return EVMSpenderAuthoritySnapshot{
		Network:             codeNetwork,
		Spender:             spender,
		ContractCodeState:   code.ContractCodeState,
		ContractCodeHash:    strings.ToLower(strings.TrimSpace(code.ContractCodeHash)),
		DelegationState:     delegationState,
		DelegationTarget:    delegationTarget,
		ProxyState:          proxy.State,
		ProxyImplementation: implementation,
		ProxyAdmin:          admin,
		ProxyBeacon:         beacon,
		ConflictingSlots:    proxy.ConflictingSlots,
		Trust:               trust,
		Reasons:             reasons,
	}, nil
}

// BindEVMApprovalSpenderAuthority attaches current spender authority evidence to
// an approval observation only when network and spender identity match exactly.
// The resulting approval intelligence remains observed evidence only.
func BindEVMApprovalSpenderAuthority(observation EVMApprovalObservation, authority EVMSpenderAuthoritySnapshot) (EVMApprovalIntelligence, error) {
	spender, err := normalizeEVMApprovalAddress(observation.Spender)
	if err != nil {
		return EVMApprovalIntelligence{}, errors.New("invalid approval spender address")
	}
	if strings.TrimSpace(observation.Network) != authority.Network || spender != authority.Spender {
		return EVMApprovalIntelligence{}, errors.New("approval and spender authority identity mismatch")
	}
	if err := ValidateWeb3TrustVector(authority.Trust); err != nil || !authority.Trust.Observed {
		return EVMApprovalIntelligence{}, errors.New("observed spender authority evidence is required")
	}

	observation.SpenderCodeHash = authority.ContractCodeHash
	observation.SpenderDelegationTarget = authority.DelegationTarget
	observation.ProxyImplementation = authority.ProxyImplementation
	observation.ProxyAdmin = authority.ProxyAdmin
	observation.ProxyBeacon = authority.ProxyBeacon

	result, err := BuildEVMApprovalIntelligence(observation)
	if err != nil {
		return EVMApprovalIntelligence{}, err
	}
	reasons := append(result.Reasons, authority.Reasons...)
	result.Reasons = NormalizeWeb3TrustReasons(reasons)
	result.Trust.Reasons = append([]string(nil), result.Reasons...)
	if authority.ConflictingSlots {
		result.Reasons = NormalizeWeb3TrustReasons(append(result.Reasons, "ERC1967_CONFLICTING_AUTHORITY_SLOTS"))
		result.Trust.Reasons = append([]string(nil), result.Reasons...)
	}
	return result, nil
}
