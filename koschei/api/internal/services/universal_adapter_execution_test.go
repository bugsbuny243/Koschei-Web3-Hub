package services

import (
	"context"
	"testing"

	"koschei/api/internal/radaradapter"
)

func TestUniversalAdapterWithoutCollectorIsDeclaredOnly(t *testing.T) {
	plan := UniversalInvestigationPlan{Subject: IntelligenceSubject{ID: "test-subject", Raw: "test", Network: "testnet", Kind: "address"}}
	result, err := ExecuteUniversalAdapter(context.Background(), plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Executable || result.Route != UniversalDispatchDeclaredOnly || len(result.Evidence) != 0 || len(result.Limitations) == 0 {
		t.Fatalf("missing collector must not be executable: %+v", result)
	}
}

type unsupportedFixtureAdapter struct{}

func (unsupportedFixtureAdapter) Supports(radaradapter.Target) bool { return false }
func (unsupportedFixtureAdapter) Collect(context.Context, radaradapter.Target) ([]radaradapter.EvidenceRow, []radaradapter.Limitation, error) {
	panic("unsupported collector must never execute")
}
func (unsupportedFixtureAdapter) Subject() radaradapter.SubjectIdentity { return radaradapter.SubjectIdentity{} }
func (unsupportedFixtureAdapter) Capability() radaradapter.Capability { return radaradapter.Capability{} }

func TestUniversalAdapterRejectsUnsupportedTarget(t *testing.T) {
	plan := UniversalInvestigationPlan{Subject: IntelligenceSubject{ID: "test-subject", Raw: "test", Network: "testnet", Kind: "address"}}
	result, err := ExecuteUniversalAdapter(context.Background(), plan, unsupportedFixtureAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Executable || result.Route != UniversalDispatchDeclaredOnly || len(result.Evidence) != 0 {
		t.Fatalf("unsupported collector incorrectly executed: %+v", result)
	}
}
