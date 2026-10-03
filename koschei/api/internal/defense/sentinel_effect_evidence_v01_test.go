package defense

import (
	"strings"
	"testing"
	"time"
)

func TestRequireVerifiedDefensiveEffectAcceptsIndependentAgreement(t *testing.T) {
	effect := ObservedDefensiveEffect{
		ActionID: "action-1", TargetID: "workload-1", ObserverID: "observer-1",
		ExpectedState: "isolated", ObservedState: "isolated",
		EvidenceDigest: "sha256:abc", ObservedAt: time.Unix(1, 0).UTC(),
	}
	if err := RequireVerifiedDefensiveEffect(effect); err != nil {
		t.Fatalf("expected verified effect, got %v", err)
	}
	if !strings.HasPrefix(effect.Digest(), "sha256:") {
		t.Fatalf("expected deterministic digest")
	}
}

func TestRequireVerifiedDefensiveEffectRejectsDisagreement(t *testing.T) {
	effect := ObservedDefensiveEffect{
		ActionID: "action-1", TargetID: "workload-1", ObserverID: "observer-1",
		ExpectedState: "isolated", ObservedState: "reachable",
		EvidenceDigest: "sha256:def",
	}
	if err := RequireVerifiedDefensiveEffect(effect); err == nil {
		t.Fatal("expected disagreement to fail closed")
	}
}

func TestRequireVerifiedDefensiveEffectRejectsUnknownObservation(t *testing.T) {
	effect := ObservedDefensiveEffect{
		ActionID: "action-1", TargetID: "workload-1", ObserverID: "observer-1",
		ExpectedState: "isolated", EvidenceDigest: "sha256:ghi",
	}
	if err := RequireVerifiedDefensiveEffect(effect); err == nil {
		t.Fatal("expected unknown observation to fail closed")
	}
}
