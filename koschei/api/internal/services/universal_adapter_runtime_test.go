package services

import (
	"context"
	"testing"

	"koschei/api/internal/radaradapter"
)

func TestRuntimeCollectorPresence(t *testing.T) {
	registry, err := NewUniversalAdapterRegistry(registryFixtureAdapter{"fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if !registry.HasRuntimeCollector(radaradapter.Target{Network: "fixture", Kind: "address", Raw: "example"}) {
		t.Fatal("registered collector was not discovered")
	}
	if registry.HasRuntimeCollector(radaradapter.Target{Network: "missing", Kind: "address"}) {
		t.Fatal("unregistered network incorrectly marked executable")
	}
}

func TestRuntimeCollectorEmptyEvidenceStaysUnexecuted(t *testing.T) {
	registry, err := NewUniversalAdapterRegistry(registryFixtureAdapter{"fixture"})
	if err != nil {
		t.Fatal(err)
	}
	plan := UniversalInvestigationPlan{
		Subject: IntelligenceSubject{
			ID: "subject-fixture", CanonicalRef: "fixture:account",
			Network: "fixture", Kind: "address", Raw: "account",
		},
		Route: UniversalDispatchDeclaredOnly,
	}
	result, err := registry.Collect(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Executable || len(result.Evidence) != 0 || len(result.Limitations) == 0 {
		t.Fatalf("empty evidence must not imply successful scan: %+v", result)
	}
}

func TestTronExecutableRouteRejectsNonNativeAdapter(t *testing.T) {
	plan := UniversalInvestigationPlan{
		Subject: IntelligenceSubject{
			ID: "fixture-subject", CanonicalRef: "fixture:account",
			Network: "fixture", Kind: "address", Raw: "example",
		},
		Route: UniversalDispatchTronWallet,
		EvidenceOnly: true,
		VerdictAuthority: UniversalVerdictEvidenceOnly,
	}
	if _, err := ExecuteUniversalAdapter(context.Background(), plan, registryFixtureAdapter{"fixture"}); err == nil {
		t.Fatal("Tron executable route accepted non-native adapter")
	}
}
