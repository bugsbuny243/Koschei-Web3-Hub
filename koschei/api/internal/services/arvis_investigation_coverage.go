package services

import "strings"

const (
	ArvisExecutionCompleted         = "completed"
	ArvisExecutionNotApplicable     = "not_applicable"
	ArvisExecutionEvidencePending   = "evidence_pending"
	ArvisExecutionSourceUnavailable = "source_unavailable"
	ArvisExecutionInsufficient      = "insufficient_evidence"
)

type ArvisArmCoverage struct {
	ModuleID        string `json:"module_id"`
	Module          string `json:"module"`
	ExecutionStatus string `json:"execution_status"`
	EvidenceStatus  string `json:"evidence_status"`
	Applicable      bool   `json:"applicable"`
	Attempted       bool   `json:"attempted"`
	Signed          bool   `json:"signed"`
	EvidenceCount   int    `json:"evidence_count"`
	Reason          string `json:"reason,omitempty"`
}

type ArvisInvestigationCoverage struct {
	Status                string             `json:"status"`
	Summary               string             `json:"summary"`
	CapabilityTotal       int                `json:"capability_total"`
	Attempted             int                `json:"attempted"`
	Completed             int                `json:"completed"`
	EvidenceProducing     int                `json:"evidence_producing"`
	CompletedWithoutMatch int                `json:"completed_without_match"`
	NotApplicable         int                `json:"not_applicable"`
	EvidencePending       int                `json:"evidence_pending"`
	SourceUnavailable     int                `json:"source_unavailable"`
	InsufficientEvidence  int                `json:"insufficient_evidence"`
	CriticalGaps          []string           `json:"critical_gaps"`
	Arms                  []ArvisArmCoverage `json:"arms"`
}

var arvisCriticalCoverageModules = map[string]string{
	ModuleTokenAuthorityScanner:  "token_authority",
	ModuleHolderConcentration:    "owner_resolved_holder_distribution",
	ModuleLiquidityMovement:      "liquidity_depth_and_control",
	ModuleCreatorLinkAnalysis:    "creator_deployer_relation",
	ModuleFundingClusterDetector: "holder_funding_relations",
	ModuleLaunchDistribution:     "launch_and_first_buyer_history",
	ModuleRepeatActorScan:        "persistent_actor_memory",
}

// BuildArvisInvestigationCoverage reports what the evidence collectors actually
// completed. Architecture arm count is never represented as completed work.
// This contract is informational only and cannot change grades, rules or signing.
func BuildArvisInvestigationCoverage(arms []SecurityRadarVerdict) ArvisInvestigationCoverage {
	coverage := ArvisInvestigationCoverage{
		CapabilityTotal: len(arms),
		CriticalGaps:    []string{},
		Arms:            make([]ArvisArmCoverage, 0, len(arms)),
	}
	completedModules := map[string]bool{}

	for _, arm := range arms {
		status := arvisArmExecutionStatus(arm)
		applicable := status != ArvisExecutionNotApplicable
		attempted := status != ""
		evidenceStatus := arvisSignalString(arm.Signals, "evidence_status")
		entry := ArvisArmCoverage{
			ModuleID: arm.ModuleID, Module: arm.Module, ExecutionStatus: status,
			EvidenceStatus: evidenceStatus, Applicable: applicable, Attempted: attempted,
			Signed: arm.Signed, EvidenceCount: len(arm.Evidence), Reason: arvisFirstEvidence(arm.Evidence),
		}
		coverage.Arms = append(coverage.Arms, entry)
		if attempted {
			coverage.Attempted++
		}
		switch status {
		case ArvisExecutionCompleted:
			coverage.Completed++
			completedModules[arm.ModuleID] = true
			if arm.Signed && len(arm.Evidence) > 0 {
				coverage.EvidenceProducing++
			}
			if !arvisSignalBool(arm.Signals, "finding_observed") && arvisSignalPresent(arm.Signals, "finding_observed") {
				coverage.CompletedWithoutMatch++
			}
		case ArvisExecutionNotApplicable:
			coverage.NotApplicable++
		case ArvisExecutionSourceUnavailable:
			coverage.SourceUnavailable++
		case ArvisExecutionInsufficient:
			coverage.InsufficientEvidence++
		default:
			coverage.EvidencePending++
		}
	}

	for moduleID, gap := range arvisCriticalCoverageModules {
		if !completedModules[moduleID] {
			coverage.CriticalGaps = append(coverage.CriticalGaps, gap)
		}
	}
	coverage.Status = "insufficient_coverage"
	if coverage.EvidenceProducing > 0 {
		coverage.Status = "partial_investigation"
	}
	if len(coverage.CriticalGaps) == 0 && coverage.EvidencePending == 0 && coverage.SourceUnavailable == 0 && coverage.InsufficientEvidence == 0 {
		coverage.Status = "complete_investigation"
	}
	coverage.Summary = arvisCoverageSummary(coverage)
	return coverage
}

func ApplyArvisInvestigationCoverage(analysis ArvisAnalysis) ArvisAnalysis {
	arms := ArvisArmsFromBundle(analysis.Bundle)
	if len(arms) == 0 {
		arms = analysis.Arms
	}
	coverage := BuildArvisInvestigationCoverage(arms)
	if analysis.Bundle.Metadata == nil {
		analysis.Bundle.Metadata = map[string]any{}
	}
	analysis.Bundle.Metadata["investigation_coverage"] = coverage
	analysis.Bundle.Metadata["investigation_output_policy"] = SharedInvestigationOutputPolicy()
	analysis.Bundle.Metadata["architecture_arm_count"] = coverage.CapabilityTotal
	analysis.Bundle.Metadata["runtime_arm_count"] = coverage.Attempted
	analysis.Bundle.Metadata["evidence_producing_arm_count"] = coverage.EvidenceProducing
	analysis.Bundle.Metadata["completed_without_match_arm_count"] = coverage.CompletedWithoutMatch
	analysis.Bundle.Metadata["not_applicable_arm_count"] = coverage.NotApplicable
	analysis.Bundle.Metadata["evidence_pending_arm_count"] = coverage.EvidencePending
	analysis.Bundle.Metadata["source_unavailable_arm_count"] = coverage.SourceUnavailable
	return analysis
}

func markArvisArmExecution(arm SecurityRadarVerdict, status, reasonCode string, applicable, findingObserved bool) SecurityRadarVerdict {
	if arm.Signals == nil {
		arm.Signals = map[string]any{}
	}
	arm.Signals["execution_status"] = status
	arm.Signals["collector_attempted"] = true
	arm.Signals["applicable"] = applicable
	arm.Signals["finding_observed"] = findingObserved
	if strings.TrimSpace(reasonCode) != "" {
		arm.Signals["reason_code"] = reasonCode
	}
	return arm
}

func evidencePendingArm(module, moduleID string, req SecurityRadarRequest, generatedAt, reason, reasonCode string) SecurityRadarVerdict {
	arm := unavailableArm(module, moduleID, req, generatedAt, reason)
	return markArvisArmExecution(arm, ArvisExecutionEvidencePending, reasonCode, true, false)
}

func notApplicableArm(module, moduleID string, req SecurityRadarRequest, generatedAt, reason, reasonCode string) SecurityRadarVerdict {
	arm := unavailableArm(module, moduleID, req, generatedAt, reason)
	arm.Recommendation = "not_applicable"
	return markArvisArmExecution(arm, ArvisExecutionNotApplicable, reasonCode, false, false)
}

func arvisArmExecutionStatus(arm SecurityRadarVerdict) string {
	if explicit := strings.ToLower(strings.TrimSpace(arvisSignalString(arm.Signals, "execution_status"))); explicit != "" {
		switch explicit {
		case ArvisExecutionCompleted, ArvisExecutionNotApplicable, ArvisExecutionEvidencePending, ArvisExecutionSourceUnavailable, ArvisExecutionInsufficient:
			return explicit
		}
	}
	if arm.Signed {
		return ArvisExecutionCompleted
	}
	// These transaction-specific collectors are intentionally not applicable to
	// a mint-only investigation unless a transaction/claim target is supplied.
	switch arm.ModuleID {
	case ModuleWalletlessClaimShield, ModuleMEVShield:
		return ArvisExecutionNotApplicable
	}
	return ArvisExecutionEvidencePending
}

func arvisSignalString(signals map[string]any, key string) string {
	if signals == nil {
		return ""
	}
	value, _ := signals[key].(string)
	return strings.TrimSpace(value)
}

func arvisSignalBool(signals map[string]any, key string) bool {
	if signals == nil {
		return false
	}
	value, _ := signals[key].(bool)
	return value
}

func arvisSignalPresent(signals map[string]any, key string) bool {
	if signals == nil {
		return false
	}
	_, ok := signals[key]
	return ok
}

func arvisFirstEvidence(evidence []string) string {
	for _, item := range evidence {
		if item = strings.TrimSpace(item); item != "" {
			return item
		}
	}
	return ""
}

func arvisCoverageSummary(coverage ArvisInvestigationCoverage) string {
	switch coverage.Status {
	case "complete_investigation":
		return "All applicable critical investigation collectors completed."
	case "partial_investigation":
		return "The investigation produced evidence, but one or more critical evidence collectors remain incomplete."
	default:
		return "The investigation did not produce enough collector coverage for a broad evidence report."
	}
}
