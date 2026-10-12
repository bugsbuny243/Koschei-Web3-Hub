package services

import (
	"context"
	"errors"
	"strings"

	"koschei/api/internal/radaradapter"
)

// UniversalAdapterRegistry contains explicitly installed runtime collectors.
// Declared network profiles are not automatically registered as collectors.
type UniversalAdapterRegistry struct {
	adapters []radaradapter.Adapter
}

func NewUniversalAdapterRegistry(adapters ...radaradapter.Adapter) (*UniversalAdapterRegistry, error) {
	registry := &UniversalAdapterRegistry{adapters: make([]radaradapter.Adapter, 0, len(adapters))}
	for _, adapter := range adapters {
		if adapter == nil {
			return nil, errors.New("nil runtime adapter")
		}
		capability := adapter.Capability()
		if len(capability.Networks) == 0 || len(capability.TargetKinds) == 0 || len(capability.DataSources) == 0 || strings.TrimSpace(capability.Fixture) == "" {
			return nil, errors.New("runtime adapter has incomplete capability declaration")
		}
		for _, network := range capability.Networks {
			if strings.TrimSpace(network) == "" {
				return nil, errors.New("runtime adapter declares empty network")
			}
		}
		for _, kind := range capability.TargetKinds {
			if strings.TrimSpace(kind) == "" {
				return nil, errors.New("runtime adapter declares empty target kind")
			}
		}
		for _, source := range capability.DataSources {
			if strings.TrimSpace(source) == "" {
				return nil, errors.New("runtime adapter declares empty data source")
			}
		}
		registry.adapters = append(registry.adapters, adapter)
	}
	return registry, nil
}

// Collect resolves a collector by Supports, not by a static adapter profile.
// Ambiguous matches fail closed instead of silently selecting a provider.
func (registry *UniversalAdapterRegistry) Collect(ctx context.Context, plan UniversalInvestigationPlan) (UniversalAdapterCollection, error) {
	if err := ctx.Err(); err != nil {
		return UniversalAdapterCollection{Route: UniversalDispatchDeclaredOnly}, err
	}
	if registry == nil {
		return ExecuteUniversalAdapter(ctx, plan, nil)
	}
	target := radaradapter.Target{Network: plan.Subject.Network, Kind: plan.Subject.Kind, Raw: plan.Subject.Raw}
	var selected radaradapter.Adapter
	for _, adapter := range registry.adapters {
		if !adapter.Supports(target) {
			continue
		}
		if selected != nil {
			return UniversalAdapterCollection{Route: UniversalDispatchDeclaredOnly}, errors.New("ambiguous runtime adapter selection")
		}
		selected = adapter
	}
	return ExecuteUniversalAdapter(ctx, plan, selected)
}

// ExecutableCapabilities returns only installed adapter capabilities. This
// does not imply successful evidence collection or grant verdict authority.
func (registry *UniversalAdapterRegistry) ExecutableCapabilities() []radaradapter.Capability {
	out := []radaradapter.Capability{}
	if registry == nil {
		return out
	}
	for _, adapter := range registry.adapters {
		capability := adapter.Capability()
		if len(capability.Networks) == 0 || len(capability.TargetKinds) == 0 {
			continue
		}
		copyCapability := radaradapter.Capability{
			Networks:         append([]string(nil), capability.Networks...),
			TargetKinds:      append([]string(nil), capability.TargetKinds...),
			DataSources:      append([]string(nil), capability.DataSources...),
			RequiresArchival: capability.RequiresArchival,
			Fixture:          strings.TrimSpace(capability.Fixture),
		}
		out = append(out, copyCapability)
	}
	return out
}

// HasRuntimeCollector reports whether exactly one registered adapter supports
// the target. This is a runtime claim, not proof of a successful scan.
func (registry *UniversalAdapterRegistry) HasRuntimeCollector(target radaradapter.Target) bool {
	if registry == nil {
		return false
	}
	matches := 0
	for _, adapter := range registry.adapters {
		if adapter.Supports(target) {
			matches++
		}
	}
	return matches == 1
}

// PrepareTronWalletPlan enables a Tron permission probe only when exactly one
// explicitly installed native-account collector matches the classified target.
// This does not register an adapter or enable any customer-facing endpoint.
func (registry *UniversalAdapterRegistry) PrepareTronWalletPlan(plan UniversalInvestigationPlan) (UniversalInvestigationPlan, error) {
	target := radaradapter.Target{Network: plan.Subject.Network, Kind: plan.Subject.Kind, Raw: plan.Subject.Raw}
	if target.Network != "tron-mainnet" || (target.Kind != "account" && target.Kind != "wallet") ||
		strings.TrimSpace(plan.Subject.ID) == "" || strings.TrimSpace(plan.Subject.CanonicalRef) == "" ||
		strings.TrimSpace(target.Raw) == "" || !registry.HasRuntimeCollector(target) {
		return plan, errors.New("tron wallet requires a classified subject and exactly one installed collector")
	}
	for _, adapter := range registry.adapters {
		if !adapter.Supports(target) {
			continue
		}
		identity := adapter.Subject()
		if _, native := adapter.(*radaradapter.TronWalletAdapter); !native {
			return plan, errors.New("tron plan requires native wallet collector")
		}
		if identity.Network != "tron-mainnet" || identity.IdentityKind != "account" ||
			strings.TrimSpace(identity.NativeID) == "" {
			return plan, errors.New("tron collector native account identity unavailable")
		}
		plan.Route = UniversalDispatchTronWallet
		plan.Executable = false // collection, not declaration, determines execution
		plan.EvidenceOnly = true
		plan.VerdictAuthority = UniversalVerdictEvidenceOnly
		return plan, nil
	}
	return plan, errors.New("tron wallet collector unavailable")
}

// CollectTronWalletPermissions runs a deliberately bounded, read-only Tron
// permission probe through an explicitly registered native collector. It does
// not register a public route or produce a safety verdict.
func (registry *UniversalAdapterRegistry) CollectTronWalletPermissions(ctx context.Context, plan UniversalInvestigationPlan) (UniversalAdapterCollection, error) {
	if err := ctx.Err(); err != nil {
		return UniversalAdapterCollection{Route: UniversalDispatchDeclaredOnly}, err
	}
	prepared, err := registry.PrepareTronWalletPlan(plan)
	if err != nil {
		return UniversalAdapterCollection{Route: UniversalDispatchDeclaredOnly}, err
	}
	collected, err := registry.Collect(ctx, prepared)
	if err != nil {
		return collected, err
	}
	if !collected.Executable || collected.Route != UniversalDispatchTronWallet || len(collected.Evidence) == 0 {
		return collected, errors.New("tron permissions evidence unavailable")
	}
	for _, evidence := range collected.Evidence {
		if evidence.Status != IntelligenceEvidenceObserved || evidence.SubjectID != prepared.Subject.ID ||
			evidence.Network != "tron-mainnet" || evidence.Provenance != "radaradapter_read_only" ||
			evidence.Attributes["account_state_height_verified"] != false {
			return UniversalAdapterCollection{Route: UniversalDispatchDeclaredOnly}, errors.New("tron permission evidence lacks observed-only provenance")
		}
	}
	return collected, nil
}
