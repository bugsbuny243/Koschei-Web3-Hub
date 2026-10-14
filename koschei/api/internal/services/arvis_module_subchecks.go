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
	ModulePumpSybilRadar: {
		required: []string{"token_signature_history", "pump_program_relation"},
		optional: []string{"funding_cluster_links"},
	},
	ModuleRaydiumPoolGuardian: {
		required: []string{"pool_program_relation", "pool_reserves"},
		optional: []string{"liquidity_movement_history"},
	},
	ModuleWalletlessClaimShield: {
		required: []string{"parsed_claim_instruction", "signer_writable_accounts"},
		optional: []string{"claim_domain_context"},
	},
	ModuleMEVShield: {
		required: []string{"swap_route", "slippage_and_pool_state"},
		optional: []string{"sandwich_transaction_linkage"},
	},
	ModuleTokenAuthorityScanner: {
		required: []string{"mint_account_observation", "mint_and_freeze_authority"},
		optional: []string{"token_2022_extensions"},
	},
	ModuleHolderConcentration: {
		required: []string{"token_largest_accounts", "holder_role_resolution"},
		optional: []string{"holder_funding_clusters"},
	},
	ModuleLiquidityMovement: {
		required: []string{"pool_reserve_evidence", "lp_control_evidence"},
		optional: []string{"liquidity_delta_transactions"},
	},
	ModuleCreatorLinkAnalysis: {
		required: []string{"creator_relation_evidence"},
		optional: []string{"creator_funding_origin"},
	},
	ModuleFundingClusterDetector: {
		required: []string{"holder_funding_edges"},
		optional: []string{"funding_recurrence"},
	},
	ModuleLaunchDistribution: {
		required: []string{"mint_specific_initial_recipients", "launch_trade_history"},
		optional: []string{"creator_linkage"},
	},
	ModuleRepeatActorScan: {
		required: []string{"persistent_actor_memory_lookup"},
		optional: []string{"verified_incident_history"},
	},
	ModuleSniperTimingDetector: {
		required: []string{"launch_signature_timing"},
		optional: []string{"verified_actor_linkage"},
	},
	ModuleClaimSurfaceRisk: {
		required: []string{"claim_url_or_domain"},
		optional: []string{"phishing_indicators"},
	},
	ModuleProgramRelationScan: {
		required: []string{"account_program_owner"},
		optional: []string{"program_upgrade_authority"},
	},
}

func arvisModuleSubchecks(arm SecurityRadarVerdict, status string) ArvisModuleSubchecks {
	definition := arvisModuleSubcheckContracts[arm.ModuleID]
	verified := SecurityRadarVerdictHasVerifiedEvidence(arm)
	out := ArvisModuleSubchecks{
		Required:             append([]string{}, definition.required...),
		Optional:             append([]string{}, definition.optional...),
		Ran:                  status == ArvisExecutionCompleted || arvisSignalBool(arm.Signals, "collector_attempted"),
		EvidencePresent:      verified && len(arm.Evidence) > 0,
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
