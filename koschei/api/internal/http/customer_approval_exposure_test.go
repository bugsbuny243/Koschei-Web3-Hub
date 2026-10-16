package http

import (
	"encoding/json"
	"math/big"
	"net/http/httptest"
	"testing"

	"koschei/api/internal/networktarget"
)

func approvalExposureFixture(amount string, unlimited bool) networktarget.EVMAllowanceProbeResult {
	return networktarget.EVMAllowanceProbeResult{
		Network:          "ethereum-mainnet",
		Token:            "0x0000000000000000000000000000000000000001",
		Owner:            "0x0000000000000000000000000000000000000002",
		Spender:          "0x0000000000000000000000000000000000000003",
		Amount:           amount,
		Unlimited:        unlimited,
		EvidenceStatus:   "observed",
		LiveAvailability: "checked",
	}
}

func TestApprovalExposureZeroIsNotCleanWalletClaim(t *testing.T) {
	e := buildCustomerApprovalExposure(approvalExposureFixture("0", false))
	if e.Status != "observed_zero_allowance_for_pair" || e.EvidenceStatus != "observed" ||
		e.RecommendedAction != "review" || e.Scope != "single_erc20_token_owner_spender" {
		t.Fatalf("zero pair allowance cannot imply wallet safety: %+v", e)
	}
	if e.EnforcementStatus != "not_connected" || e.RevocationPerformed || !e.FinalRiskScoreDisabled {
		t.Fatalf("read-only query must not claim to revoke/block or score: %+v", e)
	}
	if len(e.Limitations) < 4 || e.SpenderAuthorityCoverage != "not_completed" {
		t.Fatalf("other signatures, NFT operators and spender control must remain unknown: %+v", e)
	}
}

func TestApprovalExposureUnlimitedAndFiniteAreDistinct(t *testing.T) {
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
	unlimited := buildCustomerApprovalExposure(approvalExposureFixture(max, true))
	if unlimited.Status != "observed_unlimited_allowance" {
		t.Fatalf("max Uint256 unlimited allowance lost: %+v", unlimited)
	}
	finite := buildCustomerApprovalExposure(approvalExposureFixture("100000000", false))
	if finite.Status != "observed_nonzero_allowance" {
		t.Fatalf("finite allowance mislabeled as unlimited: %+v", finite)
	}
	// Call-site metadata must never downgrade a literal maximum authorization
	// into a seemingly finite approval.
	conflict := buildCustomerApprovalExposure(approvalExposureFixture(max, false))
	if conflict.Status != "conflicting_allowance_evidence" || conflict.EvidenceStatus != "conflicting" ||
		conflict.RecommendedAction != "review" || conflict.EnforcementStatus != "not_connected" {
		t.Fatalf("maximum allowance with false unlimited flag must fail closed: %+v", conflict)
	}
}

func TestApprovalExposureMalformedAndMissingEvidenceFailClosed(t *testing.T) {
	for _, f := range []networktarget.EVMAllowanceProbeResult{
		approvalExposureFixture("-10", false),
		approvalExposureFixture("not-numeric", false),
		approvalExposureFixture("0", true),
		func() networktarget.EVMAllowanceProbeResult {
			f := approvalExposureFixture("0", false)
			f.EvidenceStatus = "unavailable"
			return f
		}(),
	} {
		e := buildCustomerApprovalExposure(f)
		if e.Status == "observed_zero_allowance_for_pair" || e.Status == "observed_unlimited_allowance" || e.Status == "observed_nonzero_allowance" {
			t.Fatalf("malformed/unsupported evidence must not become an observed classification: %+v", e)
		}
		if e.RecommendedAction != "review" || e.EnforcementStatus != "not_connected" {
			t.Fatalf("missing evidence cannot become a permission to send: %+v", e)
		}
	}
}

func TestApprovalCustomerWireIncludesNonFinalExposure(t *testing.T) {
	allowance := approvalExposureFixture("0", false)
	e := buildCustomerApprovalExposure(allowance)
	rec := httptest.NewRecorder()
	writeCustomerApprovalScanResult(rec, 200, customerApprovalScanResult{
		Network: allowance.Network, Token: allowance.Token, Owner: allowance.Owner,
		Spender: allowance.Spender, Amount: allowance.Amount, Unlimited: allowance.Unlimited,
		EvidenceStatus: allowance.EvidenceStatus, LiveAvailability: allowance.LiveAvailability,
		Exposure: e,
	})
	if rec.Code != 200 {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	result, ok := envelope["result"].(map[string]any)
	if !ok {
		t.Fatalf("customer result missing: %s", rec.Body.String())
	}
	got, ok := result["exposure"].(map[string]any)
	if !ok || got["scope"] != "single_erc20_token_owner_spender" ||
		got["recommended_action"] != "review" || got["enforcement_status"] != "not_connected" {
		t.Fatalf("bounded exposure fields missing from customer API: %+v", got)
	}
	for _, key := range []string{"token", "owner", "spender", "amount", "unlimited", "evidence_status", "live_availability"} {
		if _, exists := result[key]; !exists {
			t.Fatalf("backwards-compatible customer field %s lost", key)
		}
	}
}
