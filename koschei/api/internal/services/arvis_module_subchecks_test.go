package services

import (
	"reflect"
	"testing"
)

func TestArvisSubchecksCoverAllFourteenArms(t *testing.T) {
	if got := len(arvisModuleSubcheckContracts); got != 14 {
		t.Fatalf("module contracts=%d want=14", got)
	}
	for moduleID, contract := range arvisModuleSubcheckContracts {
		if len(contract.required) == 0 || len(contract.optional) == 0 {
			t.Fatalf("module %s missing machine-readable checks: %#v", moduleID, contract)
		}
	}
}

func TestArvisCoverageDeterministicAndFailClosed(t *testing.T) {
	arms := []SecurityRadarVerdict{
		{ModuleID: ModuleRepeatActorScan, Module: "Repeat Actors", EvidenceVerified: true, Evidence: []string{"verified memory lookup; no matching actor"}, Signals: map[string]any{
			"execution_status": ArvisExecutionCompleted, "finding_observed": false, "required_checks_passed": true, "collector_attempted": true,
		}},
		{ModuleID: ModuleHolderConcentration, Module: "Holders", Signals: map[string]any{
			"execution_status": ArvisExecutionEvidencePending, "required_checks_passed": true,
		}},
		{ModuleID: ModuleMEVShield, Module: "MEV"},
	}
	one := BuildArvisInvestigationCoverage(arms)
	two := BuildArvisInvestigationCoverage(arms)
	if !reflect.DeepEqual(one, two) {
		t.Fatalf("same fixture produced different coverage:\nfirst=%#v\nsecond=%#v", one, two)
	}
	if one.Arms[0].Subchecks.NoEvidenceReason != "no_matching_observation" || !one.Arms[0].Subchecks.RequiredChecksPassed {
		t.Fatalf("verified no-match completion lost: %#v", one.Arms[0].Subchecks)
	}
	if one.Arms[1].Subchecks.RequiredChecksPassed || one.Arms[1].Subchecks.RequiredChecksStatus != "unknown" || one.Arms[1].Subchecks.NoEvidenceReason != "source_unavailable" {
		t.Fatalf("unverified claims must not pass: %#v", one.Arms[1].Subchecks)
	}
	if one.Arms[2].Subchecks.NoEvidenceReason != "not_applicable" {
		t.Fatalf("mint-only MEV applicability misclassified: %#v", one.Arms[2].Subchecks)
	}
	for i := 1; i < len(one.CriticalGaps); i++ {
		if one.CriticalGaps[i-1] > one.CriticalGaps[i] {
			t.Fatalf("critical gaps must be stable and sorted: %v", one.CriticalGaps)
		}
	}
}

func TestArvisModuleSubcheckContractReturnsIndependentSlices(t *testing.T) {
	arm := SecurityRadarVerdict{ModuleID: ModuleTokenAuthorityScanner}
	one := arvisModuleSubchecks(arm, ArvisExecutionEvidencePending)
	two := arvisModuleSubchecks(arm, ArvisExecutionEvidencePending)
	one.Required[0] = "mutated"
	if two.Required[0] == "mutated" {
		t.Fatal("module contract slices must not be shared")
	}
}
