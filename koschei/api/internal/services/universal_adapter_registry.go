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
