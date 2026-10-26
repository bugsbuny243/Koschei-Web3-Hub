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
		"vault_balances_read": true,
	}), ArvisExecutionCompleted)
	if liquidity.RequiredChecksStatus != "partial" || liquidity.RequiredChecksPassed || liquidity.Checks["lp_control_evidence"] != "unknown" {
		t.Fatalf("unverified LP control must leave the module partial: %#v", liquidity)
	}

	deadline := arvisModuleSubchecks(verifiedArm(ModuleHolderConcentration, map[string]any{
		"largest_accounts": 20, "holder_role_adjusted": true, "evidence_status": "partial_rpc_evidence_deadline",
		"rpc_errors": []string{"STAGE_DEADLINE_EXCEEDED stage=largest_accounts"},
	}), ArvisExecutionCompleted)
	if deadline.RequiredChecksStatus != "partial" || deadline.Checks["token_largest_accounts"] != "performed_partial" || deadline.Checks["holder_role_resolution"] != "performed" {
		t.Fatalf("only the check whose stage hit its deadline is partial: %#v", deadline)
	}

	// The profile is shared, so a holder-cluster deadline is copied into every
	// profile-backed arm; it must not downgrade the authority checks.
	unrelated := arvisModuleSubchecks(verifiedArm(ModuleTokenAuthorityScanner, map[string]any{
		"account_owner": "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb", "mint_authority_present": false, "freeze_authority_present": false,
		"evidence_status": "partial_rpc_evidence_deadline", "rpc_errors": []any{"STAGE_DEADLINE_EXCEEDED stage=holder_cluster"},
	}), ArvisExecutionCompleted)
	if unrelated.RequiredChecksStatus != "passed" || unrelated.Checks["mint_account_observation"] != "performed" {
		t.Fatalf("an unrelated stage deadline must not downgrade completed checks: %#v", unrelated)
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

// A drained pool still had its reserves read, and a role scan that found
// nothing to exclude still resolved roles; both checks ran.
func TestArvisSubchecksSeparateTheCheckFromItsResult(t *testing.T) {
	arm := func(moduleID string, signals map[string]any) ArvisModuleSubchecks {
		signals["execution_status"] = ArvisExecutionCompleted
		signals["real_onchain_evidence"] = true
		return arvisModuleSubchecks(SecurityRadarVerdict{ModuleID: moduleID, EvidenceVerified: true, Evidence: []string{"ci fixture evidence"}, Signals: signals}, ArvisExecutionCompleted)
	}
	drained := arm(ModuleRaydiumPoolGuardian, map[string]any{
		"canonical_pool": true, "pool_program": "ci-fixture-pool-program", "effective_quote_reserve": 0,
		"vault_balances_read": true, "read_slot": 461256126, "token_vault": "ci-token-vault", "quote_vault": "ci-quote-vault",
	})
	if drained.Checks["pool_reserves"] != "performed" || drained.RequiredChecksStatus != "passed" {
		t.Fatalf("a verified zero-reserve read is a performed check: %#v", drained)
	}
	// A decoded pool whose vault balance RPC failed still has a read slot and
	// vault addresses; that is not a reserve read.
	if noRead := arm(ModuleRaydiumPoolGuardian, map[string]any{"canonical_pool": true, "pool_program": "ci-fixture-pool-program",
		"reserve_snapshot_verified": true, "read_slot": 461256126, "token_vault": "ci-token-vault", "quote_vault": "ci-quote-vault",
		"vault_balances_read": false}); noRead.Checks["pool_reserves"] != "unknown" {
		t.Fatalf("without a reserve read the check stays unknown: %#v", noRead)
	}

	plain := arm(ModuleHolderConcentration, map[string]any{
		"largest_accounts": 20, "holder_role_adjusted": false,
		"holder_role_analysis": HolderRoleAnalysis{Available: true, Status: "verified_role_resolution"},
	})
	if plain.Checks["holder_role_resolution"] != "performed" || plain.RequiredChecksStatus != "passed" {
		t.Fatalf("role resolution with nothing to exclude still ran: %#v", plain)
	}
	gap := arm(ModuleHolderConcentration, map[string]any{
		"largest_accounts": 20, "holder_role_analysis": map[string]any{"available": true, "blocking_evidence_gap": true},
	})
	if gap.Checks["holder_role_resolution"] != "unknown" {
		t.Fatalf("a role analysis with a blocking evidence gap proves nothing: %#v", gap)
	}
}

// Timing evidence comes from launch forensics (launch_slot) or from the base
// collector (an exhausted mint signature history or synchronized holders);
// recipient analysis is complete only when every owner's ATA history reached
// its start.
func TestArvisSubchecksRecognizeTimingAndRecipientCollectors(t *testing.T) {
	arm := func(moduleID string, signals map[string]any) ArvisModuleSubchecks {
		signals["execution_status"] = ArvisExecutionCompleted
		signals["real_onchain_evidence"] = true
		return arvisModuleSubchecks(SecurityRadarVerdict{ModuleID: moduleID, EvidenceVerified: true, Evidence: []string{"ci fixture evidence"}, Signals: signals}, ArvisExecutionCompleted)
	}
	for _, signals := range []map[string]any{
		{"launch_slot": 447298339},
		{"mint_signature_history_exhausted": true, "recent_signature_count": 12},
		{"synchronized_wallet_count": 0, "synchronization_slot_spread": 0},
	} {
		if got := arm(ModuleSniperTimingDetector, signals).Checks["launch_signature_timing"]; got != "performed" {
			t.Fatalf("timing collector %v: launch_signature_timing = %q", signals, got)
		}
	}

	complete := arm(ModuleLaunchDistribution, map[string]any{"ata_only_policy": true, "owners_with_trade_history": 2,
		"profiles": []LaunchActorProfile{{WindowExhausted: true, SignaturesFetched: 5, TransactionsParsed: 5}, {WindowExhausted: true, SignaturesFetched: 2, TransactionsParsed: 2}}})
	if complete.Checks["mint_specific_initial_recipients"] != "performed" || complete.RequiredChecksStatus != "passed" {
		t.Fatalf("exhausted recipient histories must prove the check: %#v", complete)
	}
	bounded := arm(ModuleLaunchDistribution, map[string]any{"ata_only_policy": true, "owners_with_trade_history": 2,
		"profiles": []any{map[string]any{"window_exhausted": true, "signatures_fetched": 5, "transactions_parsed": 5}, map[string]any{"window_exhausted": false, "signatures_fetched": 5, "transactions_parsed": 5}}})
	if bounded.Checks["mint_specific_initial_recipients"] != "performed_partial" || bounded.RequiredChecksStatus != "partial" {
		t.Fatalf("a bounded recipient history is partial, not first recipients: %#v", bounded)
	}
	if none := arm(ModuleLaunchDistribution, map[string]any{"owners_with_trade_history": 2}); none.Checks["mint_specific_initial_recipients"] != "unknown" {
		t.Fatalf("without recipient profiles the check stays unknown: %#v", none)
	}
}

func TestArvisSubchecksRequireProducerCompletionEvidence(t *testing.T) {
	arm := func(moduleID string, signals map[string]any) ArvisModuleSubchecks {
		signals["execution_status"] = ArvisExecutionCompleted
		signals["real_onchain_evidence"] = true
		return arvisModuleSubchecks(SecurityRadarVerdict{ModuleID: moduleID, EvidenceVerified: true, Evidence: []string{"ci fixture evidence"}, Signals: signals}, ArvisExecutionCompleted)
	}
	if got := arm(ModulePumpSybilRadar, map[string]any{"program_relation_verified": true}).Checks["pump_program_relation"]; got != "performed" {
		t.Fatalf("a transaction-verified program relation must prove the check, got %q", got)
	}

	parsedURL := arm(ModuleClaimSurfaceRisk, map[string]any{"evidence_status": "verified_parsed_url", "url_host": "claim.example.invalid", "url_scheme": "https"})
	if parsedURL.Checks["claim_url_or_domain"] != "performed" || parsedURL.RequiredChecksStatus != "passed" {
		t.Fatalf("a parsed claim URL must prove the check: %#v", parsedURL)
	}
	if onchain := arm(ModuleClaimSurfaceRisk, map[string]any{"evidence_status": "verified_rpc_observation", "scope": "top_token_holder_owner_programs"}); onchain.Checks["claim_url_or_domain"] != "unknown" {
		t.Fatalf("on-chain distributor discovery does not inspect a URL: %#v", onchain)
	}

	unparsed := arm(ModuleLaunchDistribution, map[string]any{"ata_only_policy": true, "owners_with_trade_history": 1,
		"profiles": []LaunchActorProfile{{WindowExhausted: true, SignaturesFetched: 9, TransactionsParsed: 4}}})
	if unparsed.Checks["mint_specific_initial_recipients"] != "performed_partial" {
		t.Fatalf("pagination without parsing every signature must stay partial: %#v", unparsed)
	}
}
