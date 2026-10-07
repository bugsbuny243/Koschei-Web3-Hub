package defense

import (
	"testing"
	"time"
)

func TestValidateFindingRequiresKnownEvidence(t *testing.T) {
	finding := SecurityFinding{FindingID: "finding-1", Severity: "high", Confidence: 0.9, EvidenceIDs: []string{"evidence-1"}}
	if err := ValidateFinding(finding, []string{"evidence-1"}); err != nil {
		t.Fatalf("expected valid finding: %v", err)
	}
	finding.EvidenceIDs = []string{"unknown"}
	if err := ValidateFinding(finding, []string{"evidence-1"}); err == nil {
		t.Fatal("expected unknown evidence to fail closed")
	}
}

func TestPolicyRejectsCrossTenantAndUnauthorizedTarget(t *testing.T) {
	policy := DefensivePolicy{TenantID: "tenant-a", AllowedActions: []string{"isolate_workload"}, AllowedTargets: []string{"workload-1"}}
	base := DefensiveActionRequest{ActionID: "action-1", TenantID: "tenant-a", TargetID: "workload-1", Action: "isolate_workload", FindingID: "finding-1", EvidenceIDs: []string{"evidence-1"}}
	if got := EvaluateDefensivePolicy(policy, base); !got.Allowed {
		t.Fatalf("expected authorized request, got %q", got.Reason)
	}
	crossTenant := base
	crossTenant.TenantID = "tenant-b"
	if got := EvaluateDefensivePolicy(policy, crossTenant); got.Allowed {
		t.Fatal("cross-tenant action must fail closed")
	}
	outside := base
	outside.TargetID = "external-target"
	if got := EvaluateDefensivePolicy(policy, outside); got.Allowed {
		t.Fatal("target outside protected boundary must fail closed")
	}
}

func TestEffectAgreementPreservesIndeterminateState(t *testing.T) {
	observation := EffectObservation{ExpectedState: "isolated", ObservedState: "isolated", ScopeComplete: false, CoverageGaps: []string{"network-observer-unavailable"}}
	if got := DeriveEffectAgreement(observation); got != ControlEffectIndeterminate {
		t.Fatalf("expected indeterminate, got %q", got)
	}
	observation.ScopeComplete = true
	observation.CoverageGaps = nil
	if got := DeriveEffectAgreement(observation); got != ControlEffectAgree {
		t.Fatalf("expected agree, got %q", got)
	}
}

func TestReceiptBindsPolicyActionAndObservedEffect(t *testing.T) {
	req := DefensiveActionRequest{ActionID: "action-1", TenantID: "tenant-a", TargetID: "workload-1", Action: "isolate_workload", FindingID: "finding-1", EvidenceIDs: []string{"evidence-1"}}
	decision := PolicyDecision{Allowed: true, Reason: "authorized defensive scope"}
	observedAt := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	observation := EffectObservation{ActionID: req.ActionID, TenantID: req.TenantID, TargetID: req.TargetID, ObserverID: "observer-1", ExpectedState: "isolated", ObservedState: "isolated", EvidenceDigest: "sha256:test", ScopeComplete: true, ObservedAt: observedAt}
	receipt, err := BuildDefensiveReceipt(req, decision, observation, observedAt)
	if err != nil {
		t.Fatalf("unexpected receipt error: %v", err)
	}
	if receipt.Observation.Agreement != ControlEffectAgree || receipt.ReceiptHash == "" {
		t.Fatalf("receipt missing verified agreement/hash: %+v", receipt)
	}

	bad := observation
	bad.TargetID = "other-workload"
	if _, err := BuildDefensiveReceipt(req, decision, bad, observedAt); err == nil {
		t.Fatal("mismatched effect boundary must fail closed")
	}
}
