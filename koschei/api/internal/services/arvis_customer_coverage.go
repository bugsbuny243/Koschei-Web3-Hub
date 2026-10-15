package services

import "strings"

// ArvisWithheldArm documents a collector that did not provide verified evidence.
// Missing observation fields are explicitly marked as unavailable, never
// populated with fabricated program names, slots, or search scopes.
type ArvisWithheldArm struct {
	ModuleID        string `json:"module_id"`
	Module          string `json:"module"`
	ExecutionStatus string `json:"execution_status"`
	ReasonCode      string `json:"reason_code"`
	Searched        string `json:"searched"`
	Scope           string `json:"scope"`
	ObservedSlot    *int64 `json:"observed_slot"`
	Limitation      string `json:"limitation"`
}

type ArvisCoverageSummary struct {
	Status        string             `json:"status"`
	ArmsTotal     int                `json:"arms_total"`
	ArmsCompleted int                `json:"arms_completed"`
	ArmsWithheld  int                `json:"arms_withheld"`
	Withheld      []ArvisWithheldArm `json:"withheld"`
}

type ArvisArmCountSummary struct {
	ArmsTotal     int `json:"arms_total"`
	ArmsCompleted int `json:"arms_completed"`
	ArmsWithheld  int `json:"arms_withheld"`
}

func (c ArvisCoverageSummary) ArmCount() ArvisArmCountSummary {
	return ArvisArmCountSummary{ArmsTotal: c.ArmsTotal, ArmsCompleted: c.ArmsCompleted, ArmsWithheld: c.ArmsWithheld}
}

// UnrequestedArvisCoverage announces the architecture without claiming that
// this customer route ran an ARVIS investigation.
func UnrequestedArvisCoverage() ArvisCoverageSummary {
	return ArvisCoverageSummary{
		Status: "coverage_not_requested_on_this_route", ArmsTotal: 14,
		ArmsCompleted: 0, ArmsWithheld: 14, Withheld: []ArvisWithheldArm{},
	}
}

// ArvisCoverageFromArms is informational only. No grade, signature, rule,
// recommendation, or final verdict is affected by this projection.
func ArvisCoverageFromArms(arms []SecurityRadarVerdict) ArvisCoverageSummary {
	if len(arms) == 0 {
		return UnrequestedArvisCoverage()
	}
	summary := ArvisCoverageSummary{
		Status: "partial_investigation", ArmsTotal: len(arms),
		Withheld: make([]ArvisWithheldArm, 0, len(arms)),
	}
	for _, arm := range arms {
		if arvisArmExecutionStatus(arm) == ArvisExecutionCompleted &&
			SecurityRadarVerdictHasVerifiedEvidence(arm) && len(arm.Evidence) > 0 {
			summary.ArmsCompleted++
			continue
		}
		status := arvisArmExecutionStatus(arm)
		reasonCode := arvisSignalString(arm.Signals, "reason_code")
		if reasonCode == "" {
			reasonCode = "collector_evidence_not_completed"
		}
		searched := arvisSignalString(arm.Signals, "searched")
		if searched == "" {
			searched = "not_recorded"
		}
		scope := arvisSignalString(arm.Signals, "scope")
		if scope == "" {
			scope = "not_recorded"
		}
		limitation := arvisSignalString(arm.Signals, "limitation")
		if limitation == "" {
			limitation = "NO_VERIFIED_COLLECTOR_EVIDENCE"
		}
		var observedSlot *int64
		if arm.Signals != nil {
			switch slot := arm.Signals["observed_slot"].(type) {
			case int64:
				if slot > 0 {
					value := slot
					observedSlot = &value
				}
			case int:
				if slot > 0 {
					value := int64(slot)
					observedSlot = &value
				}
			case uint64:
				if slot > 0 && slot <= 1<<63-1 {
					value := int64(slot)
					observedSlot = &value
				}
			}
		}
		summary.Withheld = append(summary.Withheld, ArvisWithheldArm{
			ModuleID: strings.TrimSpace(arm.ModuleID), Module: strings.TrimSpace(arm.Module),
			ExecutionStatus: status, ReasonCode: reasonCode,
			Searched: searched, Scope: scope, ObservedSlot: observedSlot, Limitation: limitation,
		})
	}
	summary.ArmsWithheld = len(summary.Withheld)
	if summary.ArmsWithheld == 0 {
		summary.Status = "complete_investigation"
	} else if summary.ArmsCompleted == 0 {
		summary.Status = "insufficient_coverage"
	}
	return summary
}
