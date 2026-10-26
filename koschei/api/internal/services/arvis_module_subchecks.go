package services

import "strings"

// ArvisModuleSubchecks describes the expected evidence checks, not an assertion
// that every collector has implemented or satisfied those checks.
type ArvisModuleSubchecks struct {
	Required             []string `json:"required"`
	Optional             []string `json:"optional"`
	Ran                  bool     `json:"ran"`
	EvidencePresent      bool     `json:"evidence_present"`
	RequiredChecksPassed bool     `json:"required_checks_passed"`
	RequiredChecksStatus string   `json:"required_checks_status"`
	// Checks reports each required and optional check as "performed" when a
	// named signal of the arm's verified evidence proves it ran, as
	// "performed_partial" when the arm's evidence was cut short, else
	// "unknown". "performed" says the check ran, not that its result is safe,
	// and a check is never reported as failed from missing signals.
	Checks           map[string]string `json:"checks"`
	NoEvidenceReason string            `json:"no_evidence_reason,omitempty"`
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
	out.Checks = map[string]string{}
	passedRequired := 0
	partialEvidence := arvisSubcheckEvidencePartial(arm.Signals)
	for _, check := range append(append([]string{}, definition.required...), definition.optional...) {
		out.Checks[check] = "unknown"
		if status == ArvisExecutionCompleted && verified && arvisSubcheckProven(arm.Signals, check) {
			out.Checks[check] = "performed"
			if partialEvidence {
				out.Checks[check] = "performed_partial"
			}
		}
	}
	for _, check := range definition.required {
		if out.Checks[check] == "performed" {
			passedRequired++
		}
	}
	switch {
	case len(definition.required) > 0 && passedRequired == len(definition.required):
		out.RequiredChecksPassed = true
		out.RequiredChecksStatus = "passed"
	case passedRequired > 0:
		out.RequiredChecksStatus = "partial"
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

// arvisSubcheckEvidencePartial reports an arm whose collector stopped early,
// for example at a stage deadline; its checks ran but did not finish.
func arvisSubcheckEvidencePartial(signals map[string]any) bool {
	for _, key := range []string{"evidence_status", "data_quality"} {
		if strings.Contains(strings.ToLower(strings.TrimSpace(anyString(signals[key]))), "partial") {
			return true
		}
	}
	return false
}

// arvisSubcheckProven names, per check, the arm signal that proves it was
// performed with real evidence. A check without such a signal stays unknown;
// these predicates never infer a result from an absent field.
func arvisSubcheckProven(signals map[string]any, check string) bool {
	positive := func(key string) bool { return radarFloat(signals[key]) > 0 }
	text := func(key string) string { return strings.TrimSpace(anyString(signals[key])) }
	switch check {
	case "token_signature_history":
		return positive("owners_with_trade_history") || positive("recent_signature_count")
	case "pump_program_relation", "pool_program_relation":
		return arvisSignalBool(signals, "program_relation_observed") || (arvisSignalBool(signals, "canonical_pool") && text("pool_program") != "")
	case "pool_reserves", "pool_reserve_evidence":
		return arvisSignalBool(signals, "canonical_pool") && positive("effective_quote_reserve")
	case "lp_control_evidence":
		status := strings.ToLower(text("lp_lock_status"))
		return status != "" && status != "unverified" && status != "unknown"
	case "liquidity_movement_history", "liquidity_delta_transactions":
		return strings.HasPrefix(text("movement_status"), "complete")
	case "mint_account_observation", "account_program_owner":
		return text("account_owner") != ""
	case "mint_and_freeze_authority":
		return arvisSignalPresent(signals, "mint_authority_present") && arvisSignalPresent(signals, "freeze_authority_present")
	case "token_largest_accounts":
		return positive("largest_accounts")
	case "holder_role_resolution":
		return arvisSignalBool(signals, "holder_role_adjusted")
	case "holder_funding_edges", "holder_funding_clusters", "funding_cluster_links":
		cluster, _ := signals["holder_cluster_analysis"].(map[string]any)
		return len(cluster) > 0
	case "funding_recurrence":
		recurrence, _ := signals["funding_recurrence"].(map[string]any)
		return len(recurrence) > 0
	case "creator_relation_evidence":
		return arvisSignalBool(signals, "creator_relation_verified")
	case "launch_trade_history":
		return positive("owners_with_trade_history")
	case "launch_signature_timing":
		return positive("launch_slot")
	case "persistent_actor_memory_lookup":
		return arvisSignalBool(signals, "persistent_actor_index")
	default:
		return false
	}
}
