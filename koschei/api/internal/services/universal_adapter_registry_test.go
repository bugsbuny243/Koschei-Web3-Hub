package services

import (
	"context"
	"testing"

	"koschei/api/internal/radaradapter"
)

type registryFixtureAdapter struct {
	network string
}

func (a registryFixtureAdapter) Supports(target radaradapter.Target) bool {
	return target.Network == a.network
}
func (a registryFixtureAdapter) Collect(context.Context, radaradapter.Target) ([]radaradapter.EvidenceRow, []radaradapter.Limitation, error) {
	return nil, nil, nil
}
func (a registryFixtureAdapter) Subject() radaradapter.SubjectIdentity {
	return radaradapter.SubjectIdentity{Network: a.network, IdentityKind: "fixture", IdentityValue: "fixture", NativeID: "fixture"}
}
func (a registryFixtureAdapter) Capability() radaradapter.Capability {
	return radaradapter.Capability{Networks: []string{a.network}, TargetKinds: []string{"address"}, DataSources: []string{"fixture"}, Fixture: "universal_adapter_registry_test.go"}
}

func TestRegistryNoRuntimeAdapter(t *testing.T) {
	registry, err := NewUniversalAdapterRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.ExecutableCapabilities()) != 0 {
		t.Fatal("empty registry must have no executable capabilities")
	}
	result, err := registry.Collect(context.Background(), UniversalInvestigationPlan{Subject: IntelligenceSubject{Network: "tron-mainnet", Kind: "address"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Executable || result.Route != UniversalDispatchDeclaredOnly {
		t.Fatalf("declared network incorrectly executable: %+v", result)
	}
}

func TestRegistryRejectsAmbiguousCollectors(t *testing.T) {
	registry, err := NewUniversalAdapterRegistry(registryFixtureAdapter{"fixture"}, registryFixtureAdapter{"fixture"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Collect(context.Background(), UniversalInvestigationPlan{Subject: IntelligenceSubject{Network: "fixture", Kind: "address"}})
	if err == nil {
		t.Fatal("ambiguous collectors must fail closed")
	}
}

func TestRegistryRejectsNilAdapter(t *testing.T) {
	if _, err := NewUniversalAdapterRegistry(nil); err == nil {
		t.Fatal("nil adapter must be rejected")
	}
}

type incompleteRegistryAdapter struct {
	registryFixtureAdapter
}

func (a incompleteRegistryAdapter) Capability() radaradapter.Capability {
	return radaradapter.Capability{
		Networks: []string{"fixture"},
		TargetKinds: []string{""},
		DataSources: []string{"fixture"},
		Fixture: "universal_adapter_registry_test.go",
	}
}

func TestRegistryRejectsEmptyTargetKind(t *testing.T) {
	if _, err := NewUniversalAdapterRegistry(incompleteRegistryAdapter{}); err == nil {
		t.Fatal("empty declared target kind must be rejected")
	}
}

func TestRegistryHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	registry, err := NewUniversalAdapterRegistry(registryFixtureAdapter{"fixture"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Collect(ctx, UniversalInvestigationPlan{
		Subject: IntelligenceSubject{Network: "fixture", Kind: "address", Raw: "account"},
	})
	if err == nil {
		t.Fatal("cancelled context must stop registry dispatch")
	}
}
