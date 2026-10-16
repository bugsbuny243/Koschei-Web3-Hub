package http

import (
	"math/big"
	"strings"

	"koschei/api/internal/networktarget"
)

// customerApprovalExposure describes one observed ERC-20 token/owner/spender
// pair. It is not a wallet-wide permission inventory, risk grade, or receipt
// that a spender was blocked or an approval was revoked.
type customerApprovalExposure struct {
	Status                   string   `json:"status"`
	Scope                    string   `json:"scope"`
	ObservationPoint         string   `json:"observation_point"`
	EvidenceStatus           string   `json:"evidence_status"`
	SpenderAuthorityCoverage string   `json:"spender_authority_coverage"`
	RecommendedAction        string   `json:"recommended_action"`
	FinalRiskScoreDisabled   bool     `json:"final_risk_score_disabled"`
	EnforcementStatus        string   `json:"enforcement_status"`
	RevocationPerformed      bool     `json:"revocation_performed"`
	Limitations              []string `json:"limitations"`
}

var approvalExposureLimitations = []string{
	"ONLY_SPECIFIED_ERC20_OWNER_SPENDER_PAIR_CHECKED",
	"NO_HISTORICAL_OR_EXACT_BLOCK_PIN_ON_LATEST_PROBE",
	"PERMIT_AND_PERMIT2_OFFCHAIN_SIGNATURES_NOT_ENUMERATED",
	"NFT_OPERATORS_AND_OTHER_DELEGATES_NOT_CHECKED",
	"SPENDER_CONTROL_CAN_CHANGE_AFTER_OBSERVATION",
}

// buildCustomerApprovalExposure refuses to infer a clean wallet from a zero
// allowance. ProbeEVMAllowance performs chain ID validation separately; this
// projection cannot promote a failed observation to a clean bill of health.
func buildCustomerApprovalExposure(allowance networktarget.EVMAllowanceProbeResult) customerApprovalExposure {
	out := customerApprovalExposure{
		Status:                   "unverified_allowance",
		Scope:                    "single_erc20_token_owner_spender",
		ObservationPoint:         "rpc_latest_unpinned_block",
		EvidenceStatus:           "unverified",
		SpenderAuthorityCoverage: "not_completed",
		RecommendedAction:        "review",
		FinalRiskScoreDisabled:   true,
		EnforcementStatus:        "not_connected",
		RevocationPerformed:      false,
		Limitations:              append([]string(nil), approvalExposureLimitations...),
	}
	if allowance.EvidenceStatus != "observed" || allowance.LiveAvailability != "checked" ||
		strings.TrimSpace(allowance.Network) == "" || strings.TrimSpace(allowance.Token) == "" ||
		strings.TrimSpace(allowance.Owner) == "" || strings.TrimSpace(allowance.Spender) == "" {
		return out
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(allowance.Amount), 10)
	if !ok || amount.Sign() < 0 || amount.BitLen() > 256 {
		return out
	}
	// Unlimited is a derived property of the numeric ABI observation,
	// not a trustworthy independent caller label. Both mismatch directions
	// must fail closed; otherwise max Uint256 may appear as a finite grant.
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	isMaximum := amount.Cmp(max) == 0
	if allowance.Unlimited != isMaximum {
		out.Status = "conflicting_allowance_evidence"
		out.EvidenceStatus = "conflicting"
		return out
	}
	out.EvidenceStatus = "observed"
	if isMaximum {
		out.Status = "observed_unlimited_allowance"
		return out
	}
	if amount.Sign() == 0 {
		out.Status = "observed_zero_allowance_for_pair"
	} else {
		out.Status = "observed_nonzero_allowance"
	}
	// Zero allowance covers only the queried pair at one point in time:
	// it does not establish safety, revoke other operators or bind a permit.
	return out
}
