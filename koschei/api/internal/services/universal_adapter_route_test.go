package services

import (
	"context"
	"testing"
	"time"

	"koschei/api/internal/radaradapter"
)

type routeEvidenceFixture struct{}

func (routeEvidenceFixture) Supports(radaradapter.Target) bool { return true }
func (routeEvidenceFixture) Capability() radaradapter.Capability {
	return radaradapter.Capability{Networks: []string{"ethereum-mainnet"}, TargetKinds: []string{"address"}, DataSources: []string{"fixture"}, Fixture: "universal_adapter_route_test.go"}
}
func (routeEvidenceFixture) Subject() radaradapter.SubjectIdentity {
	return radaradapter.SubjectIdentity{Network: "ethereum-mainnet", IdentityKind: "chain_id", IdentityValue: "1", NativeID: "account"}
}
func (routeEvidenceFixture) Collect(_ context.Context, _ radaradapter.Target) ([]radaradapter.EvidenceRow, []radaradapter.Limitation, error) {
	return []radaradapter.EvidenceRow{{
		ID: "evidence-1", SubjectID: "subject-1", Status: "observed",
		Source: "fixture", Height: "123", RawHash: "sha256:fixture",
		ObservedAt: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC),
	}}, nil, nil
}

func TestDeclaredRouteCannotBecomeExecutableFromEvidence(t *testing.T) {
	plan := UniversalInvestigationPlan{
		Subject: IntelligenceSubject{
			ID: "subject-1", CanonicalRef: "fixture:account", Network: "ethereum-mainnet",
			Kind: "address", Raw: "account",
		},
		Route: UniversalDispatchDeclaredOnly,
	}
	result, err := ExecuteUniversalAdapter(context.Background(), plan, routeEvidenceFixture{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Executable || result.Route != UniversalDispatchDeclaredOnly || len(result.Evidence) != 1 || len(result.Limitations) == 0 {
		t.Fatalf("declared route promoted incorrectly: %+v", result)
	}
}
