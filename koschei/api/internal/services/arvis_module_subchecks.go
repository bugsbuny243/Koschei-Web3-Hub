package services

// ArvisModuleSubchecks describes the expected evidence checks, not an assertion
// that every collector has implemented or satisfied those checks.
type ArvisModuleSubchecks struct {
	Required             []string `json:"required"`
	Optional             []string `json:"optional"`
	Ran                  bool     `json:"ran"`
	EvidencePresent      bool     `json:"evidence_present"`
	RequiredChecksPassed bool     `json:"required_checks_passed"`
	RequiredChecksStatus string   `json:"required_checks_status"`
	NoEvidenceReason     string   `json:"no_evidence_reason,omitempty"`
}

type arvisModuleSubcheckDefinition struct {
	required []string
	optional []string
}

// Check labels are explicit requirements/observations, not synthetic evidence.
// Missing confirmation of a required check remains unknown.
var arvisModuleSubcheckContracts = map[string]arvisModuleSubcheckDefinition{
	ModulePumpSybilRadar:        {[]string{"token_signature_history", "pump_program_relation"}, []string{"funding_cluster_links"}},
	ModuleRaydiumPoolGuardian:   {[]string{"pool_program_relation", "pool_reserves"}, []string{"liquidity_movement_history"}},
	ModuleWalletlessClaimShield: {[]string{"parsed_claim_instruction", "signer_writable_accounts"}, []string{"claim_domain_context"}},
	ModuleMEVShield:              {[]string{"swap_route", "slippage_and_pool_state"}, []string{"sandwich_transaction_linkage"}},
	ModuleTokenAuthorityScanner: {[]string{"mint_account_observation", "mint_and_freeze_authority"}, []string{"token_2022_extensions"}},
	ModuleHolderConcentration:    {[]string{"token_largest_accounts", "holder_role_resolution"}, []string{"holder_funding_clusters"}},
	ModuleLiquidityMovement:     {[]string{"pool_reserve_evidence", "lp_control_evidence"}, []string{"liquidity_delta_transactions"}},
	ModuleCreatorLinkAnalysis:   {[]string{"creator_relation_evidence"}, []string{"creator_funding_origin"}},
	ModuleFundingClusterDetector: {[]string{"holder_funding_edges"}, []string{"funding_recurrence"}},
	ModuleLaunchDistribution:     {[]string{"mint_specific_initial_recipients", "launch_trade_history"}, []string{"creator_linkage"}},
	ModuleRepeatActorScan:        {[]string{"persistent_actor_memory_lookup"}, []string{"verified_incident_history"}},
	ModuleSniperTimingDetector:   {[]string{"launch_signature_timing"}, []string{"verified_actor_linkage"}},
	ModuleClaimSurfaceRisk:       {[]string{"claim_url_or_domain"}, []string{"phishing_indicators"}},
	ModuleProgramRelationScan:    {[]string{"account_program_owner"}, []string{"program_upgrade_authority"}},
}

func arvisModuleSubchecks(arm SecurityRadarVerdict, status string) ArvisModuleSubchecks {
	definition := arvisModuleSubcheckContracts[arm.ModuleID]
	verified := SecurityRadarVerdictHasVerifiedEvidence(arm)
	out := ArvisModuleSubchecks{
		Required: append([]string{}, definition.required...),
		Optional: append([]string{}, definition.optional...),
		Ran: status == ArvisExecutionCompleted || arvisSignalBool(arm.Signals, "collector_attempted"),
		EvidencePresent: verified && len(arm.Evidence) > 0,
		RequiredChecksStatus: "unknown",
	}
	if status == ArvisExecutionNotApplicable {
		out.RequiredChecksStatus = "not_applicable"
		out.NoEvidenceReason = "not_applicable"
		return out
	}
	if arvisSignalPresent(arm.Signals, "required_checks_passed") && status == ArvisExecutionCompleted && verified {
		out.RequiredChecksPassed = arvisSignalBool(arm.Signals, "required_checks_passed")
		if out.RequiredChecksPassed {
			out.RequiredChecksStatus = "passed"
		} else {
			out.RequiredChecksStatus = "failed"
		}
	}
	if status == ArvisExecutionCompleted && arvisSignalPresent(arm.Signals, "finding_observed") && !arvisSignalBool(arm.Signals, "finding_observed") {
		out.NoEvidenceReason = "no_matching_observation"
	} else if !out.EvidencePresent {
		out.NoEvidenceReason = "source_unavailable"
	}
	return out
}
