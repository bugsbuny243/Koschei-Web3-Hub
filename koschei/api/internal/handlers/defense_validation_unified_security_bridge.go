package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/defense"
	"koschei/api/internal/securityevidence"
	"koschei/api/internal/services"
)

const defenseValidationUnifiedProjectionVersion = "koschei-defense-validation-unified/v1"

func buildDefenseValidationUnifiedSecurityProjection(input defenseValidationAPIRequest, report defense.DefenseValidationReportV02) (services.UnifiedSecurityInvestigation, error) {
	if strings.TrimSpace(report.ReportHash) == "" || strings.TrimSpace(report.ScenarioContractHash) == "" {
		return services.UnifiedSecurityInvestigation{}, errors.New("defense validation report identity is incomplete")
	}
	if report.MainnetTransactionSent || report.VerdictAuthority {
		return services.UnifiedSecurityInvestigation{}, errors.New("defense validation report exceeds the non-authoritative isolated-validation boundary")
	}

	generatedAt := defenseValidationUnifiedGeneratedAt(input)
	base := services.BuildIntelligenceInvestigation(nil, generatedAt)
	unified := services.BuildUnifiedSecurityInvestigation(base, generatedAt)

	controlByRef := make(map[string]defenseValidationAPIControl, len(input.Controls))
	for _, control := range input.Controls {
		controlByRef[strings.TrimSpace(control.ControlRef)] = control
	}
	resultByCase := defenseValidationUnifiedCaseResults(report)
	network := defenseValidationUnifiedNetwork(report.Chain, report.ChainID)

	for _, raw := range input.Cases {
		if raw.ObservationBinding == nil || raw.ObservationEvent == nil {
			continue
		}
		caseRef := strings.TrimSpace(raw.CaseRef)
		controlRef := strings.TrimSpace(raw.ControlRef)
		control, ok := controlByRef[controlRef]
		if !ok {
			return services.UnifiedSecurityInvestigation{}, fmt.Errorf("unified projection cannot resolve control %q", controlRef)
		}
		result, ok := resultByCase[defenseValidationUnifiedCaseKey(controlRef, caseRef)]
		if !ok {
			return services.UnifiedSecurityInvestigation{}, fmt.Errorf("unified projection cannot resolve validated case %q", caseRef)
		}

		expectedSubject := securityevidence.Subject{
			Chain: strings.ToLower(strings.TrimSpace(report.Chain)),
			Type:  defense.DefenseValidationObservationSubjectTypeV02,
			ID:    caseRef,
		}
		projection, err := services.AdaptUnifiedSignedSecurityEvidence(*raw.ObservationEvent, services.UnifiedSignedEvidenceBinding{
			Domain:           services.UnifiedSecurityDomainBlockchain,
			SubjectKind:      services.IntelligenceSubjectValidationCase,
			Network:          network,
			ExpectedProducer: strings.TrimSpace(control.CollectorRef),
			TrustedPublicKey: strings.TrimSpace(control.CollectorPublicKey),
			ExpectedSubject:  expectedSubject,
		})
		if err != nil {
			return services.UnifiedSecurityInvestigation{}, fmt.Errorf("case %q unified signed-evidence projection: %w", caseRef, err)
		}
		appendIntelligenceSubjectIfMissing(&base, projection.Subject)
		base.Evidence = append(base.Evidence, projection.Evidence...)

		signedRefs := defenseValidationUnifiedEvidenceIDs(projection.Evidence)
		status := defenseValidationUnifiedCaseEvidenceStatus(result, projection.Evidence)
		reportEvidenceID := fmt.Sprintf("defense-validation:%s:%s:%s", strings.ToLower(strings.TrimSpace(report.ReportHash)), controlRef, caseRef)
		allRefs := defenseValidationUnifiedUniqueRefs(append(signedRefs, reportEvidenceID))
		confidence := defenseValidationUnifiedConfidence(status)

		base.Evidence = append(base.Evidence, services.IntelligenceEvidence{
			ID:          reportEvidenceID,
			SubjectID:   projection.Subject.ID,
			ChainFamily: projection.Subject.ChainFamily,
			Chain:       projection.Subject.Chain,
			Network:     projection.Subject.Network,
			Source:      defenseValidationUnifiedProjectionVersion,
			Status:      status,
			ObservedAt:  defenseValidationUnifiedCaseObservedAt(*raw.ObservationEvent),
			Method:      strings.TrimSpace(result.TechniqueID),
			StateChange: strings.TrimSpace(result.Outcome),
			Provenance:  "deterministic_defense_validation_report",
			Confidence:  confidence,
			Attributes: map[string]any{
				"run_ref":                    strings.TrimSpace(report.RunRef),
				"report_hash":                strings.ToLower(strings.TrimSpace(report.ReportHash)),
				"report_verdict":             strings.TrimSpace(report.Verdict),
				"scenario_ref":               strings.TrimSpace(report.ScenarioRef),
				"scenario_version":           strings.TrimSpace(report.ScenarioVersion),
				"scenario_contract_hash":     strings.ToLower(strings.TrimSpace(report.ScenarioContractHash)),
				"ruleset_version":            strings.TrimSpace(report.RulesetVersion),
				"control_ref":                strings.TrimSpace(result.ControlRef),
				"case_ref":                   strings.TrimSpace(result.CaseRef),
				"case_kind":                  strings.TrimSpace(result.CaseKind),
				"technique_id":               strings.TrimSpace(result.TechniqueID),
				"execution_mode":             strings.TrimSpace(result.ExecutionMode),
				"execution_evidence_state":   strings.TrimSpace(result.ExecutionEvidenceState),
				"observation_status":         strings.TrimSpace(result.ObservationStatus),
				"observation_evidence_state": strings.TrimSpace(result.ObservationEvidenceState),
				"outcome":                    strings.TrimSpace(result.Outcome),
				"detection_ms":               result.DetectionMS,
				"lead_time_ms":               result.LeadTimeMS,
				"triggered_rules":            append([]string(nil), result.TriggeredRules...),
				"source_evidence_refs":       append([]string(nil), result.EvidenceRefs...),
				"source_evidence_hashes":     append([]string(nil), result.EvidenceHashes...),
				"limitations":                append([]string(nil), result.Limitations...),
				"mainnet_transaction_sent":   false,
				"verdict_authority":          false,
			},
		})

		base.Behaviors = append(base.Behaviors, services.IntelligenceBehaviorFinding{
			Kind:         "defense_validation_" + strings.TrimSpace(result.Outcome),
			Summary:      defenseValidationUnifiedBehaviorSummary(result),
			Status:       status,
			Confidence:   confidence,
			EvidenceRefs: allRefs,
		})

		action := services.BuildIntelligenceAction(
			projection.Subject.ID,
			"",
			"",
			services.UnifiedSecurityDomainBlockchain,
			"defense_validation_case_execution",
			controlRef,
			"",
			strings.TrimSpace(result.Outcome),
			status,
			allRefs,
			confidence,
		)
		unified.Actions = append(unified.Actions, action)

		if status == services.IntelligenceEvidenceVerified && result.CaseKind == defense.DefenseValidationCaseAttackV02 && defenseValidationUnifiedOutcomeIsGap(result.Outcome) {
			summary := defenseValidationUnifiedGapSummary(result)
			consequence := services.BuildIntelligenceConsequence(
				services.UnifiedSecurityDomainProtocol,
				"defense_validation_gap",
				controlRef,
				summary,
				strings.TrimSpace(result.Outcome),
				services.IntelligenceEvidenceVerified,
				allRefs,
				1,
			)
			unified.AttackPaths = append(unified.AttackPaths, services.BuildUnifiedSecurityAttackPath(
				"Validated defense gap: "+strings.TrimSpace(result.TechniqueID),
				projection.Subject.ID,
				services.IntelligenceEvidenceVerified,
				[]string{
					"isolated_fork_or_sandbox_validation_only",
					"exact_scenario_contract=" + strings.ToLower(strings.TrimSpace(report.ScenarioContractHash)),
					"not_a_production_exploit_or_compromise_claim",
				},
				[]services.UnifiedSecurityAttackPathStep{{
					SubjectID:    projection.Subject.ID,
					ActionID:     action.ID,
					Effect:       summary,
					EvidenceRefs: allRefs,
				}},
				[]services.IntelligenceConsequence{consequence},
				allRefs,
				1,
			))
		}
	}

	unified.Base = base
	return services.FinalizeUnifiedSecurityInvestigation(unified), nil
}

func defenseValidationUnifiedCaseResults(report defense.DefenseValidationReportV02) map[string]defense.DefenseValidationCaseResultV02 {
	out := make(map[string]defense.DefenseValidationCaseResultV02)
	for _, control := range report.Controls {
		for _, result := range control.Cases {
			out[defenseValidationUnifiedCaseKey(result.ControlRef, result.CaseRef)] = result
		}
	}
	return out
}

func defenseValidationUnifiedCaseKey(controlRef, caseRef string) string {
	return strings.TrimSpace(controlRef) + "\x00" + strings.TrimSpace(caseRef)
}

func defenseValidationUnifiedGeneratedAt(input defenseValidationAPIRequest) time.Time {
	var latest int64
	for _, item := range input.Cases {
		if item.ObservationEvent != nil && item.ObservationEvent.Window.ToUnixMS > latest {
			latest = item.ObservationEvent.Window.ToUnixMS
		}
	}
	return time.UnixMilli(latest).UTC()
}

func defenseValidationUnifiedCaseObservedAt(event securityevidence.Event) time.Time {
	return time.UnixMilli(event.Window.ToUnixMS).UTC()
}

func defenseValidationUnifiedNetwork(chain string, chainID uint64) string {
	chain = strings.ToLower(strings.TrimSpace(chain))
	if chainID > 0 && (chain == "evm" || strings.Contains(chain, "evm") || strings.HasPrefix(chain, "eip155:")) {
		return "eip155:" + strconv.FormatUint(chainID, 10)
	}
	return chain
}

func defenseValidationUnifiedEvidenceIDs(evidence []services.IntelligenceEvidence) []string {
	refs := make([]string, 0, len(evidence))
	for _, item := range evidence {
		if ref := strings.TrimSpace(item.ID); ref != "" {
			refs = append(refs, ref)
		}
	}
	return defenseValidationUnifiedUniqueRefs(refs)
}

func defenseValidationUnifiedCaseEvidenceStatus(result defense.DefenseValidationCaseResultV02, evidence []services.IntelligenceEvidence) string {
	if result.ExecutionEvidenceState != defense.DefenseValidationEvidenceVerifiedV02 || result.ObservationEvidenceState != defense.DefenseValidationEvidenceVerifiedV02 || result.Outcome == defense.DefenseValidationOutcomeIncompleteV02 || len(evidence) == 0 {
		return services.IntelligenceEvidenceUnverified
	}
	for _, item := range evidence {
		if item.Status != services.IntelligenceEvidenceVerified {
			return services.IntelligenceEvidenceUnverified
		}
	}
	return services.IntelligenceEvidenceVerified
}

func defenseValidationUnifiedConfidence(status string) float64 {
	if status == services.IntelligenceEvidenceVerified {
		return 1
	}
	return 0
}

func defenseValidationUnifiedOutcomeIsGap(outcome string) bool {
	switch strings.TrimSpace(outcome) {
	case defense.DefenseValidationOutcomeMissedV02, defense.DefenseValidationOutcomeCaughtLateV02:
		return true
	default:
		return false
	}
}

func defenseValidationUnifiedBehaviorSummary(result defense.DefenseValidationCaseResultV02) string {
	controlRef := strings.TrimSpace(result.ControlRef)
	caseRef := strings.TrimSpace(result.CaseRef)
	switch strings.TrimSpace(result.Outcome) {
	case defense.DefenseValidationOutcomeCaughtInTimeV02:
		return fmt.Sprintf("Control %s caught attack case %s before the validation impact deadline.", controlRef, caseRef)
	case defense.DefenseValidationOutcomeCaughtLateV02:
		return fmt.Sprintf("Control %s detected attack case %s only after the validation impact deadline.", controlRef, caseRef)
	case defense.DefenseValidationOutcomeMissedV02:
		return fmt.Sprintf("Control %s did not signal for attack case %s during the completed validation observation window.", controlRef, caseRef)
	case defense.DefenseValidationOutcomeFalsePositiveV02:
		return fmt.Sprintf("Control %s signaled on benign validation case %s.", controlRef, caseRef)
	case defense.DefenseValidationOutcomeCleanV02:
		return fmt.Sprintf("Control %s remained quiet on benign validation case %s.", controlRef, caseRef)
	default:
		return fmt.Sprintf("Defense validation case %s for control %s is incomplete.", caseRef, controlRef)
	}
}

func defenseValidationUnifiedGapSummary(result defense.DefenseValidationCaseResultV02) string {
	if result.Outcome == defense.DefenseValidationOutcomeCaughtLateV02 {
		return fmt.Sprintf("In the isolated validation scenario, control %s detected technique %s after the configured impact deadline.", strings.TrimSpace(result.ControlRef), strings.TrimSpace(result.TechniqueID))
	}
	return fmt.Sprintf("In the isolated validation scenario, control %s did not detect technique %s during the completed observation window.", strings.TrimSpace(result.ControlRef), strings.TrimSpace(result.TechniqueID))
}

func defenseValidationUnifiedUniqueRefs(refs []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if _, exists := seen[ref]; exists {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	return out
}
