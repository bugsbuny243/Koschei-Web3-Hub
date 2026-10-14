package radaradapter

import (
	"context"
	"testing"
	"time"
)

type testAdapter struct {
	rows   []EvidenceRow
	limits []Limitation
}

func (a testAdapter) Supports(Target) bool { return true }
func (a testAdapter) Collect(context.Context, Target) ([]EvidenceRow, []Limitation, error) {
	return a.rows, a.limits, nil
}
func (a testAdapter) Subject() SubjectIdentity {
	return SubjectIdentity{Network: "fixture", NativeID: "account", ChainIdentity: "genesis"}
}
func (a testAdapter) Capability() Capability {
	return Capability{Network: "fixture", TargetClasses: []string{"address"}, Fixture: "contract_test.go", EvidenceOnly: true}
}
func TestConformanceRejectsMissingEvidenceAndForgedProvenance(t *testing.T) {
	target := Target{"fixture", "address", "account"}
	if Conform(context.Background(), testAdapter{}, target) == nil {
		t.Fatal("empty scan accepted")
	}
	a := testAdapter{rows: []EvidenceRow{{Network: "fixture", SubjectID: "account", ObservedAt: time.Now()}}}
	if Conform(context.Background(), a, target) == nil {
		t.Fatal("unanchored evidence accepted")
	}
	a = testAdapter{limits: []Limitation{{"unknown", "fixture source unavailable"}}}
	if e := Conform(context.Background(), a, target); e == nil {
		t.Fatal("limitation-only fixture must not enable execution")
	}
}
func TestRegistryEvidenceOnlyCannotPromote(t *testing.T) {
	row := NewRow(SubjectIdentity{Network: "fixture", NativeID: "account", ChainIdentity: "genesis"}, "authority", Response{Raw: []byte(`{"fixture":true}`), Host: "fixture.example"}, "1", nil)
	row.Status = "verified"
	factory := func(Target, Transport) (Adapter, error) { return testAdapter{rows: []EvidenceRow{row}}, nil }
	r := NewRegistry()
	target := Target{"fixture", "address", "account"}
	if e := r.Register(factory, target, nil); e != nil {
		t.Fatal(e)
	}
	out := r.Run(context.Background(), target, nil)
	if len(out.Evidence) != 1 || out.Evidence[0].Status != "observed" {
		t.Fatal("evidence promotion")
	}
	target.Kind = "bridge"
	if r.Run(context.Background(), target, nil).Status != "declared_not_executable" {
		t.Fatal("unsupported marked executable")
	}
	row.Attributes["critical"] = true
	if ValidateRow(row, SubjectIdentity{Network: "fixture", NativeID: "account", ChainIdentity: "genesis"}) == nil {
		t.Fatal("unconfirmed critical evidence accepted")
	}
}
