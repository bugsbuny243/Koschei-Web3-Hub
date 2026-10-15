package services

import (
	"encoding/json"
	"testing"
)

func TestCustomerCoverageUnrequestedDoesNotClaimEvidence(t *testing.T) {
	c := ArvisCoverageFromArms(nil)
	if c.Status != "coverage_not_requested_on_this_route" || c.ArmsTotal != 14 || c.ArmsCompleted != 0 || c.ArmsWithheld != 14 {
		t.Fatalf("unrequested route must not claim completed arms: %+v", c)
	}
	if len(c.Withheld) != 0 {
		t.Fatalf("no collectors ran, so no fabricated per-arm observations: %+v", c.Withheld)
	}
}

func TestCustomerCoveragePreservesBoundedNegativeReasonAndSlot(t *testing.T) {
	arms := []SecurityRadarVerdict{
		{
			ModuleID: ModuleWalletlessClaimShield, Module: "Walletless Claim Shield",
			Signals: map[string]any{
				"execution_status": ArvisExecutionEvidencePending,
				"reason_code":      "no_known_distributor_program_found",
				"searched":         "1 fixture-backed distributor program",
				"scope":            "token_largest_accounts",
				"observed_slot":    int64(457310747),
				"limitation":       "UNKNOWN_CUSTOM_DISTRIBUTOR_NOT_RULED_OUT",
			},
		},
		{
			ModuleID: ModuleClaimSurfaceRisk, Module: "Claim Surface Risk",
			Signals: map[string]any{"execution_status": ArvisExecutionSourceUnavailable, "reason_code": "no_declared_origin"},
		},
	}
	c := ArvisCoverageFromArms(arms)
	if c.ArmsTotal != 2 || c.ArmsCompleted != 0 || c.ArmsWithheld != 2 {
		t.Fatalf("unexpected coverage counts: %+v", c)
	}
	if c.Withheld[0].ObservedSlot == nil || *c.Withheld[0].ObservedSlot != 457310747 ||
		c.Withheld[0].ReasonCode != "no_known_distributor_program_found" ||
		c.Withheld[0].Limitation != "UNKNOWN_CUSTOM_DISTRIBUTOR_NOT_RULED_OUT" {
		t.Fatalf("bounded negative evidence fields lost: %+v", c.Withheld[0])
	}
	if c.Withheld[1].ObservedSlot != nil || c.Withheld[1].Searched != "not_recorded" {
		t.Fatalf("missing source must not invent a slot or a search: %+v", c.Withheld[1])
	}
}

func TestCustomerScanResultHasArmDenominatorWithoutChangingVerdict(t *testing.T) {
	c := UnrequestedArvisCoverage()
	if c.ArmCount().ArmsTotal != c.ArmsTotal || c.ArmCount().ArmsWithheld != c.ArmsWithheld {
		t.Fatalf("arm_count diverged from coverage: %+v", c)
	}
	result := CustomerScanResult{
		Status: CustomerScanStatusObserved, Verdict: CustomerScanVerdictReview,
		Coverage: &c,
	}
	count := c.ArmCount()
	result.ArmCount = &count
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	coverage, ok := decoded["coverage"].(map[string]any)
	if !ok || coverage["arms_total"] != float64(14) || coverage["arms_completed"] != float64(0) {
		t.Fatalf("coverage missing on /api/scan result: %s", raw)
	}
	if decoded["verdict"] != CustomerScanVerdictReview || decoded["status"] != CustomerScanStatusObserved {
		t.Fatalf("coverage must not change existing decision fields: %s", raw)
	}
}
