package services

import (
	"context"
	"errors"
	"strings"

	"koschei/api/internal/radaradapter"
)

// UniversalAdapterCollection is the result of an actual read-only collector
// invocation. A declared profile by itself is never evidence of execution.
type UniversalAdapterCollection struct {
	Route       string                    `json:"route"`
	Executable  bool                      `json:"executable"`
	Evidence    []IntelligenceEvidence    `json:"evidence,omitempty"`
	Limitations []radaradapter.Limitation `json:"limitations,omitempty"`
}

// ExecuteUniversalAdapter requires an explicit executable adapter instance.
// Existing live chain routes remain on their established collectors until
// individually migrated with parity fixtures. No grades are produced here.
func ExecuteUniversalAdapter(ctx context.Context, plan UniversalInvestigationPlan, adapter radaradapter.Adapter) (UniversalAdapterCollection, error) {
	result := UniversalAdapterCollection{Route: UniversalDispatchDeclaredOnly, Evidence: []IntelligenceEvidence{}}
	if adapter == nil {
		result.Limitations = []radaradapter.Limitation{{Code: "NO_EXECUTABLE_COLLECTOR", Reason: "No runtime collector is registered for this target."}}
		return result, nil
	}
	target := radaradapter.Target{Network: plan.Subject.Network, Kind: plan.Subject.Kind, Raw: plan.Subject.Raw}
	if !adapter.Supports(target) {
		result.Limitations = []radaradapter.Limitation{{Code: "UNSUPPORTED_TARGET", Reason: "Runtime collector does not support this target."}}
		return result, nil
	}
	if strings.TrimSpace(plan.Subject.ID) == "" {
		return result, errors.New("unclassified universal adapter subject")
	}
	rows, limitations, err := adapter.Collect(ctx, target)
	result.Limitations = append(result.Limitations, limitations...)
	if err != nil {
		return result, err
	}
	evidence, err := AdaptRadarEvidence(plan.Subject, adapter.Subject(), rows)
	if err != nil {
		return result, err
	}
	if len(evidence) == 0 {
		result.Limitations = append(result.Limitations, radaradapter.Limitation{Code: "NO_EVIDENCE", Reason: "Collector returned no reproducible evidence; safety cannot be inferred."})
		return result, nil
	}
	result.Route = plan.Route
	result.Executable = true
	result.Evidence = evidence
	return result, nil
}
