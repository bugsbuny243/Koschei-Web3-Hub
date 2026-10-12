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
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if adapter == nil {
		result.Limitations = []radaradapter.Limitation{{Code: "NO_EXECUTABLE_COLLECTOR", Reason: "No runtime collector is registered for this target."}}
		return result, nil
	}
	target := radaradapter.Target{Network: plan.Subject.Network, Kind: plan.Subject.Kind, Raw: plan.Subject.Raw}
	if !adapter.Supports(target) {
		result.Limitations = []radaradapter.Limitation{{Code: "UNSUPPORTED_TARGET", Reason: "Runtime collector does not support this target."}}
		return result, nil
	}
	if strings.TrimSpace(plan.Subject.ID) == "" || strings.TrimSpace(plan.Subject.CanonicalRef) == "" || strings.TrimSpace(plan.Subject.Network) == "" || strings.TrimSpace(plan.Subject.Raw) == "" {
		return result, errors.New("unclassified universal adapter subject")
	}
	rows, limitations, err := adapter.Collect(ctx, target)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}
	result.Limitations = append(result.Limitations, limitations...)
	if err != nil {
		return result, err
	}
	if len(rows) == 0 {
		result.Limitations = append(result.Limitations, radaradapter.Limitation{Code: "NO_EVIDENCE", Reason: "Collector returned no reproducible evidence; safety cannot be inferred."})
		return result, nil
	}
	// Tron collectors use the native account address as their row subject.
	// Rebind only after confirming it matches the installed adapter identity;
	// never trust an arbitrary collector-supplied subject ID.
	if plan.Subject.Network == "tron-mainnet" {
		identity := adapter.Subject()
		for i := range rows {
			if rows[i].SubjectID != identity.NativeID {
				return result, errors.New("tron evidence native account identity mismatch")
			}
			rows[i].SubjectID = plan.Subject.ID
		}
	}
	evidence, err := AdaptRadarEvidence(plan.Subject, adapter.Subject(), rows)
	if err != nil {
		return result, err
	}

	if plan.Route == UniversalDispatchTronWallet {
		if plan.Subject.Network != "tron-mainnet" || (plan.Subject.Kind != "account" && plan.Subject.Kind != "wallet") ||
			adapter.Subject().Network != "tron-mainnet" || adapter.Subject().IdentityKind != "account" ||
			plan.VerdictAuthority != UniversalVerdictEvidenceOnly || !plan.EvidenceOnly {
			return result, errors.New("tron executable route requires evidence-only wallet plan")
		}
		for _, row := range rows {
			if row.Status != "observed" || row.Attributes["account_state_height_verified"] != false {
				return result, errors.New("tron wallet evidence must remain unanchored and observed")
			}
		}
		result.Route = UniversalDispatchTronWallet
		result.Executable = true
		result.Evidence = evidence
		return result, nil
	}
	if plan.Route != UniversalDispatchSolanaCore && plan.Route != UniversalDispatchSolanaTx && plan.Route != UniversalDispatchEVMProbe && plan.Route != UniversalDispatchEVMTransaction && plan.Route != UniversalDispatchBitcoinProbe && plan.Route != UniversalDispatchMoveIdentity {
		result.Limitations = append(result.Limitations, radaradapter.Limitation{Code: "NO_EXECUTABLE_ROUTE", Reason: "Collected evidence cannot promote a declared or unsupported route into an executable scan."})
		result.Evidence = evidence
		return result, nil
	}
	result.Route = plan.Route
	result.Executable = true
	result.Evidence = evidence
	return result, nil
}
