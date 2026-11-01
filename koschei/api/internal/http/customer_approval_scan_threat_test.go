package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
)

func TestCustomerApprovalResultRunsPB03OnObservedRPCResponse(t *testing.T) {
	original := customerApprovalScanResult{
		Network: "ethereum-mainnet",
		Token: "0x1111111111111111111111111111111111111111",
		Owner: "0x2222222222222222222222222222222222222222",
		Spender: "0x3333333333333333333333333333333333333333",
		Amount: "115792089237316195423570985008687907853269984665640564039457584007913129639935",
		Unlimited: true,
		EvidenceStatus: "observed", LiveAvailability: "checked",
		SpenderAuthority: &services.EVMSpenderAuthoritySnapshot{
			Network: "ethereum-mainnet",
			Spender: "0x3333333333333333333333333333333333333333",
			DelegationState: networktarget.EVMDelegationStateObserved,
			DelegationTarget: "0x4444444444444444444444444444444444444444",
			ProxyState: networktarget.EVMProxyAuthorityNotObserved,
			Trust: services.Web3TrustVector{Observed: true},
		},
	}
	emit := func(result customerApprovalScanResult) services.SOCApprovalThreatAssessment {
		t.Helper()
		rec := httptest.NewRecorder()
		writeCustomerApprovalScanResult(rec, http.StatusOK, result)
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("unexpected HTTP response: %d %+v", rec.Code, rec.Header())
		}
		var got struct {
			SchemaVersion string `json:"schema_version"`
			Result struct {
				ThreatAssessment services.SOCApprovalThreatAssessment `json:"threat_assessment"`
			} `json:"result"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.SchemaVersion != customerApprovalScanSchemaVersion {
			t.Fatalf("legacy approval scan API contract changed: %s", got.SchemaVersion)
		}
		return got.Result.ThreatAssessment
	}
	full := emit(original)
	if full.PlaybookID != services.SOCApprovalThreatPlaybookID || full.Severity != "high" ||
		full.FindingState != "unlimited_allowance_with_mutable_spender_authority" ||
		full.EnforcementState != "not_connected" ||
		full.ReportIntegrity != "unsigned_read_only_observation" {
		t.Fatalf("real scan response did not expose evidence-bound PB03: %+v", full)
	}
	missing := original
	missing.SpenderAuthority = nil
	withheld := emit(missing)
	if withheld.FindingState != "withheld" || withheld.EvidenceState != "incomplete" || withheld.Severity != "unknown" {
		t.Fatalf("incomplete scanner evidence was promoted into a finding: %+v", withheld)
	}
	finite := original
	finite.Amount = "500"
	finite.Unlimited = false
	finite.SpenderAuthority = nil
	incomplete := emit(finite)
	if incomplete.FindingState != "withheld" {
		t.Fatalf("missing authority on finite allowance incorrectly marked clear: %+v", incomplete)
	}
}
