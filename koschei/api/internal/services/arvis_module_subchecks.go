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
	deadlineStages := arvisSubcheckDeadlineStages(arm.Signals)
	for _, check := range append(append([]string{}, definition.required...), definition.optional...) {
		out.Checks[check] = "unknown"
		if status == ArvisExecutionCompleted && verified && arvisSubcheckProven(arm.Signals, check) {
			out.Checks[check] = "performed"
			if deadlineStages[arvisSubcheckStage[check]] {
				out.Checks[check] = "performed_partial"
			}
		} else if check == "mint_specific_initial_recipients" && status == ArvisExecutionCompleted && verified {
			// Recipients were analysed but some owner's ATA history did not
			// reach its start, so first recipients are not asserted.
			if _, ok := arvisLaunchRecipientProfiles(arm.Signals); ok && arvisSignalBool(arm.Signals, "ata_only_policy") {
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

// arvisSubcheckStage names the collector stage each check depends on. The
// evidence profile is shared by several arms, so only a deadline recorded for
// a check's own stage marks that check partial.
var arvisSubcheckStage = map[string]string{
	"mint_account_observation":  "account",
	"mint_and_freeze_authority": "account",
	"account_program_owner":     "account",
	"token_largest_accounts":    "largest_accounts",
	"holder_role_resolution":    "holder_roles",
	"token_signature_history":   "signatures",
	"holder_funding_edges":      "holder_cluster",
	"holder_funding_clusters":   "holder_cluster",
	"funding_cluster_links":     "holder_cluster",
	"funding_recurrence":        "holder_cluster",
}

// arvisSubcheckDeadlineStages returns the stages whose collector recorded
// a STAGE_DEADLINE_EXCEEDED marker in the arm's rpc_errors.
func arvisSubcheckDeadlineStages(signals map[string]any) map[string]bool {
	stages := map[string]bool{}
	var errs []string
	switch value := signals["rpc_errors"].(type) {
	case []string:
		errs = value
	case []any:
		for _, item := range value {
			errs = append(errs, anyString(item))
		}
	}
	for _, item := range errs {
		if _, stage, ok := strings.Cut(item, "STAGE_DEADLINE_EXCEEDED stage="); ok {
			stages[strings.TrimSpace(strings.Fields(stage + " ")[0])] = true
		}
	}
	return stages
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
		return arvisSignalBool(signals, "program_relation_observed") || arvisSignalBool(signals, "program_relation_verified") ||
			(arvisSignalBool(signals, "canonical_pool") && text("pool_program") != "")
	case "pool_reserves", "pool_reserve_evidence":
		// Both vault balance reads succeeding is the check; a drained (zero)
		// reserve is its result. A decoded pool with a failed read proves nothing.
		return arvisSignalBool(signals, "vault_balances_read")
	case "lp_control_evidence":
		status := strings.ToLower(text("lp_lock_status"))
		return status != "" && status != "unverified" && status != "unknown"
	case "liquidity_movement_history", "liquidity_delta_transactions":
		status := text("movement_status")
		return status == "observed" || strings.HasPrefix(status, "complete") || arvisSignalBool(signals, "liquidity_movement_transaction_verified")
	case "mint_account_observation", "account_program_owner":
		return text("account_owner") != ""
	case "mint_and_freeze_authority":
		return arvisSignalPresent(signals, "mint_authority_present") && arvisSignalPresent(signals, "freeze_authority_present")
	case "token_largest_accounts":
		return positive("largest_accounts")
	case "holder_role_resolution":
		// Resolution completes even when no protocol or burn balance needed excluding.
		return arvisSignalBool(signals, "holder_role_adjusted") || arvisAnalysisAvailable(signals["holder_role_analysis"])
	case "holder_funding_edges", "holder_funding_clusters", "funding_cluster_links":
		return arvisAnalysisAvailable(signals["holder_cluster_analysis"])
	case "funding_recurrence":
		return arvisAnalysisAvailable(signals["funding_recurrence"])
	case "creator_relation_evidence":
		return arvisSignalBool(signals, "creator_relation_verified")
	case "launch_trade_history":
		return positive("owners_with_trade_history")
	case "launch_signature_timing":
		// Launch forensics emits the launch slot; the base timing collector
		// emits an exhausted mint signature history or synchronized holders.
		return positive("launch_slot") || arvisSignalBool(signals, "mint_signature_history_exhausted") ||
			arvisSignalPresent(signals, "synchronized_wallet_count")
	case "claim_url_or_domain":
		// Only an explicitly supplied URL that was parsed proves this check;
		// on-chain distributor discovery does not inspect a URL.
		return text("evidence_status") == "verified_parsed_url" && text("url_host") != ""
	case "mint_specific_initial_recipients":
		complete, ok := arvisLaunchRecipientProfiles(signals)
		return ok && complete
	case "persistent_actor_memory_lookup":
		return arvisSignalBool(signals, "persistent_actor_index")
	default:
		return false
	}
}

// arvisAnalysisAvailable reads the availability flag of a funding analysis,
// which fresh scans carry as a typed struct and stored reports as a map.
func arvisAnalysisAvailable(value any) bool {
	switch analysis := value.(type) {
	case HolderClusterAnalysis:
		return analysis.Available
	case *HolderClusterAnalysis:
		return analysis != nil && analysis.Available
	case HolderRoleAnalysis:
		return analysis.Available && !analysis.BlockingEvidenceGap
	case *HolderRoleAnalysis:
		return analysis != nil && analysis.Available && !analysis.BlockingEvidenceGap
	case FundingRecurrenceAnalysis:
		return analysis.Available
	case *FundingRecurrenceAnalysis:
		return analysis != nil && analysis.Available
	case map[string]any:
		available, _ := analysis["available"].(bool)
		gap, _ := analysis["blocking_evidence_gap"].(bool)
		return available && !gap
	default:
		return false
	}
}

// arvisLaunchRecipientProfiles reports whether mint-specific recipient
// profiles exist (ok) and whether every profile's ATA history window reached
// its start (complete). Fresh scans carry typed profiles, stored reports maps.
func arvisLaunchRecipientProfiles(signals map[string]any) (complete bool, ok bool) {
	if !arvisSignalBool(signals, "ata_only_policy") {
		return false, false
	}
	// A profile is complete only when its ATA history reached its start and
	// every successful (non-failed) signature was parsed; pagination alone
	// does not prove the earliest acquisition transaction was decoded.
	exhausted := []bool{}
	switch profiles := signals["profiles"].(type) {
	case []LaunchActorProfile:
		for _, profile := range profiles {
			eligible := profile.SuccessfulSignatures
			if eligible == 0 {
				eligible = profile.SignaturesFetched
			}
			exhausted = append(exhausted, profile.WindowExhausted && eligible > 0 && profile.TransactionsParsed >= eligible)
		}
	case []any:
		for _, item := range profiles {
			profile, _ := item.(map[string]any)
			done, _ := profile["window_exhausted"].(bool)
			eligible, parsed := radarFloat(profile["successful_signatures"]), radarFloat(profile["transactions_parsed"])
			if eligible == 0 {
				// Profiles stored before failed signatures were counted fall
				// back to every fetched signature, which can only understate.
				eligible = radarFloat(profile["signatures_fetched"])
			}
			exhausted = append(exhausted, done && eligible > 0 && parsed >= eligible)
		}
	}
	if len(exhausted) == 0 {
		return false, false
	}
	for _, done := range exhausted {
		if !done {
			return false, true
		}
	}
	return true, true
}
