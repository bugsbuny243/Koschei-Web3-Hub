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

func TestArvisSubchecksReportPerformedChecksFromArmSignalsOnly(t *testing.T) {
	verifiedArm := func(moduleID string, signals map[string]any) SecurityRadarVerdict {
		signals["execution_status"] = ArvisExecutionCompleted
		signals["real_onchain_evidence"] = true
		signals["verified_evidence"] = true
		return SecurityRadarVerdict{ModuleID: moduleID, EvidenceVerified: true, Evidence: []string{"ci fixture evidence"}, Signals: signals}
	}
	authority := arvisModuleSubchecks(verifiedArm(ModuleTokenAuthorityScanner, map[string]any{
		"account_owner": "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb", "mint_authority_present": false, "freeze_authority_present": false,
	}), ArvisExecutionCompleted)
	if authority.RequiredChecksStatus != "passed" || !authority.RequiredChecksPassed ||
		authority.Checks["mint_and_freeze_authority"] != "performed" || authority.Checks["token_2022_extensions"] != "unknown" {
		t.Fatalf("authority checks = %#v", authority)
	}

	liquidity := arvisModuleSubchecks(verifiedArm(ModuleLiquidityMovement, map[string]any{
		"canonical_pool": true, "pool_program": "ci-fixture-pool-program", "effective_quote_reserve": 27.3, "lp_lock_status": "unverified",
	}), ArvisExecutionCompleted)
	if liquidity.RequiredChecksStatus != "partial" || liquidity.RequiredChecksPassed || liquidity.Checks["lp_control_evidence"] != "unknown" {
		t.Fatalf("unverified LP control must leave the module partial: %#v", liquidity)
	}

	deadline := arvisModuleSubchecks(verifiedArm(ModuleHolderConcentration, map[string]any{
		"largest_accounts": 20, "holder_role_adjusted": true, "evidence_status": "partial_rpc_evidence_deadline",
	}), ArvisExecutionCompleted)
	if deadline.RequiredChecksStatus != "unknown" || deadline.Checks["token_largest_accounts"] != "performed_partial" {
		t.Fatalf("checks cut short by a deadline must not count as performed: %#v", deadline)
	}

	pending := arvisModuleSubchecks(SecurityRadarVerdict{ModuleID: ModuleTokenAuthorityScanner, Signals: map[string]any{
		"account_owner": "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb", "mint_authority_present": false, "freeze_authority_present": false,
	}}, ArvisExecutionEvidencePending)
	if pending.RequiredChecksStatus != "unknown" || pending.Checks["mint_account_observation"] != "unknown" {
		t.Fatalf("an arm without verified completed evidence proves no check: %#v", pending)
	}
}

// A collection that found liquidity movements reports movement_status
// "observed"; that is the check succeeding, not an unperformed one.
func TestArvisSubchecksCountObservedLiquidityMovementsAsPerformed(t *testing.T) {
	for _, signals := range []map[string]any{
		{"movement_status": "observed", "liquidity_movement_transaction_verified": true},
		{"movement_status": "complete_no_movement_observed"},
	} {
		signals["execution_status"] = ArvisExecutionCompleted
		signals["real_onchain_evidence"] = true
		arm := SecurityRadarVerdict{ModuleID: ModuleLiquidityMovement, EvidenceVerified: true, Evidence: []string{"ci fixture evidence"}, Signals: signals}
		if got := arvisModuleSubchecks(arm, ArvisExecutionCompleted).Checks["liquidity_delta_transactions"]; got != "performed" {
			t.Fatalf("movement_status %v: liquidity_delta_transactions = %q, want performed", signals["movement_status"], got)
		}
	}
	arm := SecurityRadarVerdict{ModuleID: ModuleLiquidityMovement, EvidenceVerified: true, Evidence: []string{"ci fixture evidence"}, Signals: map[string]any{
		"execution_status": ArvisExecutionCompleted, "real_onchain_evidence": true, "movement_status": "collection_failed",
	}}
	if got := arvisModuleSubchecks(arm, ArvisExecutionCompleted).Checks["liquidity_delta_transactions"]; got != "unknown" {
		t.Fatalf("a failed movement collection must stay unknown, got %q", got)
	}
}

// Fresh scans carry the funding analyses as typed structs; stored reports carry
// them as JSON maps. Both must prove the funding checks, and only when available.
func TestArvisSubchecksReadTypedAndStoredFundingAnalyses(t *testing.T) {
	arm := func(cluster, recurrence any) ArvisModuleSubchecks {
		return arvisModuleSubchecks(SecurityRadarVerdict{ModuleID: ModuleFundingClusterDetector, EvidenceVerified: true, Evidence: []string{"ci fixture evidence"}, Signals: map[string]any{
			"execution_status": ArvisExecutionCompleted, "real_onchain_evidence": true,
			"holder_cluster_analysis": cluster, "funding_recurrence": recurrence,
		}}, ArvisExecutionCompleted)
	}
	if got := arm(HolderClusterAnalysis{Available: true}, FundingRecurrenceAnalysis{Available: true}); got.RequiredChecksStatus != "passed" || got.Checks["funding_recurrence"] != "performed" {
		t.Fatalf("typed analyses not read: %#v", got)
	}
	if got := arm(map[string]any{"available": true}, map[string]any{"available": true}); got.RequiredChecksStatus != "passed" {
		t.Fatalf("stored analyses not read: %#v", got)
	}
	if got := arm(HolderClusterAnalysis{Available: false}, map[string]any{"available": false, "status": "unavailable"}); got.RequiredChecksStatus != "unknown" || got.Checks["funding_recurrence"] != "unknown" {
		t.Fatalf("unavailable analyses must not prove a check: %#v", got)
	}
}
