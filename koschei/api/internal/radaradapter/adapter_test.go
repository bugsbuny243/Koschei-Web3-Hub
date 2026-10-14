package radaradapter

import (
	"context"
	"testing"
	"time"
)

type fixtureAdapter struct{}

func (fixtureAdapter) Supports(target Target) bool {
	return target.Network == "fixture" && target.Kind == "address"
}

func (fixtureAdapter) Subject() SubjectIdentity {
	return SubjectIdentity{Network: "fixture", IdentityKind: "genesis_hash", IdentityValue: "fixture-genesis", NativeID: "fixture-account"}
}

func (fixtureAdapter) Capability() Capability {
	return Capability{Networks: []string{"fixture"}, TargetKinds: []string{"address"}, DataSources: []string{"fixture"}, Fixture: "adapter_test.go"}
}

func (fixtureAdapter) Collect(ctx context.Context, target Target) ([]EvidenceRow, []Limitation, error) {
	return []EvidenceRow{{ID: "row-1", SubjectID: "fixture-account", Status: "observed", Source: "fixture", Height: "123", RawHash: "sha256:fixture", ObservedAt: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)}}, nil, nil
}

func TestOfflineContract(t *testing.T) {
	adapter := fixtureAdapter{}
	target := Target{Network: "fixture", Kind: "address", Raw: "fixture-account"}
	if !adapter.Supports(target) {
		t.Fatal("unsupported fixture")
	}
	rows, limitations, err := adapter.Collect(context.Background(), target)
	if err != nil || len(limitations) != 0 || len(rows) != 1 {
		t.Fatalf("unexpected collection result: %v %v %v", rows, limitations, err)
	}
	if err := ValidateEvidence(adapter.Subject(), rows); err != nil {
		t.Fatal(err)
	}
}

func TestMissingProvenanceRejected(t *testing.T) {
	adapter := fixtureAdapter{}
	if err := ValidateEvidence(adapter.Subject(), []EvidenceRow{{ID: "incomplete"}}); err == nil {
		t.Fatal("incomplete evidence accepted")
	}
	if err := ValidateEvidence(SubjectIdentity{}, nil); err == nil {
		t.Fatal("incomplete subject accepted")
	}
}

func TestEvidenceIdentityAndStatusGuards(t *testing.T) {
	subject := fixtureAdapter{}.Subject()
	valid := EvidenceRow{ID: "row-1", SubjectID: subject.NativeID, Status: "observed", Source: "fixture", Height: "123", RawHash: "sha256:fixture", ObservedAt: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)}
	if err := ValidateEvidence(subject, []EvidenceRow{valid}); err != nil {
		t.Fatalf("valid evidence rejected: %v", err)
	}
	wrongSubject := valid
	wrongSubject.SubjectID = ""
	if err := ValidateEvidence(subject, []EvidenceRow{wrongSubject}); err == nil {
		t.Fatal("empty investigation subject ID accepted")
	}
	wrongSubjectIdentity := valid
	wrongSubjectIdentity.SubjectID = "another-account"
	if err := ValidateEvidence(subject, []EvidenceRow{wrongSubjectIdentity}); err == nil {
		t.Fatal("evidence from a different native subject accepted")
	}
	wrongStatus := valid
	wrongStatus.Status = "safe"
	if err := ValidateEvidence(subject, []EvidenceRow{wrongStatus}); err == nil {
		t.Fatal("unknown status accepted")
	}
	if err := ValidateEvidence(subject, []EvidenceRow{valid, valid}); err == nil {
		t.Fatal("duplicate evidence ID accepted")
	}
}
