package services

import (
	"strings"
	"testing"

	"koschei/api/internal/networktarget"
)

func approvalDetectionTestObservation(amount string, unlimited bool) networktarget.EVMAllowanceProbeResult {
	return networktarget.EVMAllowanceProbeResult{
		Network: "ethereum-mainnet", Token: "0x1111111111111111111111111111111111111111",
		Owner:   "0x2222222222222222222222222222222222222222",
		Spender: "0x3333333333333333333333333333333333333333",
		Amount:  amount, Unlimited: unlimited,
		EvidenceStatus: "observed", LiveAvailability: "checked",
	}
}

func approvalDetectionTestAuthority() *EVMSpenderAuthoritySnapshot {
	return &EVMSpenderAuthoritySnapshot{
		Network: "ethereum-mainnet", Spender: "0x3333333333333333333333333333333333333333",
		DelegationState: networktarget.EVMDelegationStateNotObserved,
		ProxyState:      networktarget.EVMProxyAuthorityNotObserved,
		Trust:           Web3TrustVector{Observed: true},
	}
}

func TestSOCApprovalThreatDetectsCompoundExposureWithoutClaimingExploitOrEnforcement(t *testing.T) {
	max := "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	tests := []struct {
		name, amount, delegation, implementation, finding, severity string
		unlimited                                                   bool
	}{
		{"unlimited_delegated", max, "0x4444444444444444444444444444444444444444", "", "unlimited_allowance_with_mutable_spender_authority", "high", true},
		{"unlimited_upgradeable", max, "", "0x5555555555555555555555555555555555555555", "unlimited_allowance_with_mutable_spender_authority", "high", true},
		{"unlimited_ordinary", max, "", "", "unlimited_allowance_observed", "medium", true},
		{"finite_delegated", "500", "0x4444444444444444444444444444444444444444", "", "nonzero_allowance_with_mutable_spender_authority", "medium", false},
		{"finite_ordinary", "500", "", "", "no_compound_spender_exposure_observed", "none", false},
		{"zero_delegated", "0", "0x4444444444444444444444444444444444444444", "", "no_active_allowance_for_pair_observed", "none", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			authority := approvalDetectionTestAuthority()
			if tc.delegation != "" {
				authority.DelegationState = networktarget.EVMDelegationStateObserved
				authority.DelegationTarget = tc.delegation
			}
			if tc.implementation != "" {
				authority.ProxyState = networktarget.EVMProxyAuthorityObserved
				authority.ProxyImplementation = tc.implementation
			}
			got := DetectSOCApprovalThreat(approvalDetectionTestObservation(tc.amount, tc.unlimited), authority)
			if got.PlaybookID != SOCApprovalThreatPlaybookID || got.SchemaVersion != SOCApprovalThreatSchemaVersion ||
				got.FindingState != tc.finding || got.Severity != tc.severity ||
				got.EvidenceState != "observed_latest_unpinned" ||
				got.EnforcementState != "not_connected" || got.ReportIntegrity != "unsigned_read_only_observation" {
				t.Fatalf("invalid evidence-bound assessment: %+v", got)
			}
			if got.FindingState == "safe" || got.RecommendedAction == "block_applied" {
				t.Fatalf("observations cannot be promoted to safe or autonomous block: %+v", got)
			}
		})
	}
}

func TestSOCApprovalThreatWithholdsOnMissingConflictingOrForgedEvidence(t *testing.T) {
	max := "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	tests := []struct {
		name    string
		prepare func(*networktarget.EVMAllowanceProbeResult, *EVMSpenderAuthoritySnapshot)
		missing bool
	}{
		{"missing_authority", nil, true},
		{"wrong_spender", func(_ *networktarget.EVMAllowanceProbeResult, a *EVMSpenderAuthoritySnapshot) {
			a.Spender = "0x7777777777777777777777777777777777777777"
		}, false},
		{"wrong_network", func(_ *networktarget.EVMAllowanceProbeResult, a *EVMSpenderAuthoritySnapshot) {
			a.Network = "base-mainnet"
		}, false},
		{"missing_observation", func(_ *networktarget.EVMAllowanceProbeResult, a *EVMSpenderAuthoritySnapshot) {
			a.Trust.Observed = false
		}, false},
		{"fabricated_verified_authority", func(_ *networktarget.EVMAllowanceProbeResult, a *EVMSpenderAuthoritySnapshot) {
			a.Trust.Verified = true
		}, false},
		{"forged_unlimited_label", func(o *networktarget.EVMAllowanceProbeResult, _ *EVMSpenderAuthoritySnapshot) { o.Unlimited = false }, false},
		{"negative_allowance", func(o *networktarget.EVMAllowanceProbeResult, _ *EVMSpenderAuthoritySnapshot) { o.Amount = "-1" }, false},
		{"oversized_allowance", func(o *networktarget.EVMAllowanceProbeResult, _ *EVMSpenderAuthoritySnapshot) { o.Amount = max + "0" }, false},
		{"unobserved_allowance", func(o *networktarget.EVMAllowanceProbeResult, _ *EVMSpenderAuthoritySnapshot) {
			o.EvidenceStatus = "unverified"
		}, false},
		{"unavailable_allowance", func(o *networktarget.EVMAllowanceProbeResult, _ *EVMSpenderAuthoritySnapshot) {
			o.LiveAvailability = "unavailable"
		}, false},
		{"unrecognized_delegation", func(_ *networktarget.EVMAllowanceProbeResult, a *EVMSpenderAuthoritySnapshot) {
			a.DelegationState = "claimed_delegation"
		}, false},
		{"unbound_delegation", func(_ *networktarget.EVMAllowanceProbeResult, a *EVMSpenderAuthoritySnapshot) {
			a.DelegationState = networktarget.EVMDelegationStateObserved
		}, false},
		{"empty_proxy_authority", func(_ *networktarget.EVMAllowanceProbeResult, a *EVMSpenderAuthoritySnapshot) {
			a.ProxyState = networktarget.EVMProxyAuthorityObserved
		}, false},
		{"bad_subject", func(o *networktarget.EVMAllowanceProbeResult, _ *EVMSpenderAuthoritySnapshot) { o.Token = "0xdead" }, false},
		{"uppercase_subject_mismatch", func(o *networktarget.EVMAllowanceProbeResult, _ *EVMSpenderAuthoritySnapshot) {
			o.Owner = strings.ToUpper(o.Owner)
		}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			observation := approvalDetectionTestObservation(max, true)
			authority := approvalDetectionTestAuthority()
			if tc.prepare != nil {
				tc.prepare(&observation, authority)
			}
			if tc.missing {
				authority = nil
			}
			got := DetectSOCApprovalThreat(observation, authority)
			if got.FindingState != "withheld" || got.Severity != "unknown" ||
				got.EnforcementState != "not_connected" || got.EvidenceState == "observed_latest_unpinned" ||
				len(got.ReasonCodes) == 0 {
				t.Fatalf("untrusted RPC evidence manufactured a threat finding: %+v", got)
			}
		})
	}
}
