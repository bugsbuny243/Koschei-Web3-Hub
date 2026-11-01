package defense

import (
	"testing"
	"time"
)

func activeResponseRecoveryFixture(t *testing.T, action, kind string) (
	ActiveResponsePlan, ActiveResponseReview, ActiveResponseRecoveryCapability, time.Time,
) {
	t.Helper()
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, action, kind)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	// Recovery tests evaluate an offline review snapshot. The high-impact
	// approval tests separately prove the verifier rejects missing signatures.
	review := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, nil, nil, now)
	if review.Status == ActiveResponseReviewDenied {
		t.Fatalf("invalid review setup: %+v", review)
	}
	if plan.RequiresHumanApproval {
		review.Status = ActiveResponseReviewReady
		review.RequiredApprovals = 2
		review.VerifiedApprovals = 2
	}
	capability := ActiveResponseRecoveryCapability{
		ProviderID:             provider.ProviderID,
		TenantID:               req.TenantID,
		TargetID:               req.TargetID,
		Action:                 req.Action,
		RunbookRef:             "approved-runbook-v1",
		RecoveryOwnerID:        "operator-a",
		IndependentObserverID:  "observer-b",
		ManagementPathExcluded: true,
		BreakGlassVerified:     true,
		RollbackTestAttested:   true,
		RollbackTestedAt:       now.Add(-time.Hour),
		ProviderTTLSeconds:     plan.MaxDurationSeconds,
	}
	return plan, review, capability, now
}

func TestActiveResponseRecoveryModesCoverCatalog(t *testing.T) {
	for _, spec := range ActiveResponseCatalog() {
		mode, err := ActiveResponseRecoveryMode(spec.Action)
		if err != nil || mode == "" {
			t.Errorf("missing recovery requirement for %s: %v", spec.Action, err)
		}
	}
	if _, err := ActiveResponseRecoveryMode("arbitrary_provider_command"); err == nil {
		t.Fatal("unknown recovery action accepted")
	}
}

func TestActiveResponseRecoveryRequiresIndependentRecoveryEvidence(t *testing.T) {
	plan, review, capability, now := activeResponseRecoveryFixture(t, "bgp_blackhole", "protected_prefix")
	good := AssessActiveResponseRecovery(plan, review, capability, now)
	if good.Status != ActiveResponseRecoveryReview || good.ExecutionPerformed || good.RollbackPerformed {
		t.Fatalf("recovery readiness should remain non-executing: %+v", good)
	}
	capability.ProviderTTLSeconds = plan.MaxDurationSeconds + 1
	bad := AssessActiveResponseRecovery(plan, review, capability, now)
	if bad.Status != ActiveResponseRecoveryBlocked {
		t.Fatalf("unbounded blackhole TTL passed: %+v", bad)
	}
	capability.ProviderTTLSeconds = plan.MaxDurationSeconds
	capability.RollbackTestedAt = now.Add(-31 * 24 * time.Hour)
	bad = AssessActiveResponseRecovery(plan, review, capability, now)
	if bad.Status != ActiveResponseRecoveryBlocked {
		t.Fatalf("stale rollback test passed: %+v", bad)
	}
	capability.RollbackTestedAt = now.Add(-time.Hour)
	capability.BreakGlassVerified = false
	bad = AssessActiveResponseRecovery(plan, review, capability, now)
	if bad.Status != ActiveResponseRecoveryBlocked {
		t.Fatalf("unverified break-glass path passed: %+v", bad)
	}
}

func TestActiveResponseRecoveryRejectsOperatorObserverCollision(t *testing.T) {
	plan, review, capability, now := activeResponseRecoveryFixture(t, "isolate_segment", "network_segment")
	capability.IndependentObserverID = capability.RecoveryOwnerID
	bad := AssessActiveResponseRecovery(plan, review, capability, now)
	if bad.Status != ActiveResponseRecoveryBlocked {
		t.Fatalf("same operator and observer accepted: %+v", bad)
	}
	capability.IndependentObserverID = "independent"
	capability.ManagementPathExcluded = false
	bad = AssessActiveResponseRecovery(plan, review, capability, now)
	if bad.Status != ActiveResponseRecoveryBlocked {
		t.Fatalf("management path not protected: %+v", bad)
	}
	capability.ManagementPathExcluded = true
	capability.TenantID = "foreign-tenant"
	bad = AssessActiveResponseRecovery(plan, review, capability, now)
	if bad.Status != ActiveResponseRecoveryBlocked {
		t.Fatalf("cross-tenant recovery binding accepted: %+v", bad)
	}
}

func TestActiveResponseRecoveryManualOnlyForNonAutomaticActions(t *testing.T) {
	for _, item := range []struct {
		action string
		kind   string
	}{
		{"pause_contract", "smart_contract"},
		{"terminate_session", "session"},
		{"revoke_api_key", "api_key"},
		{"air_gap", "hardware_link"},
		{"lock_hsm", "hsm_key"},
		{"block_transaction", "transaction_policy"},
	} {
		t.Run(item.action, func(t *testing.T) {
			plan, review, capability, now := activeResponseRecoveryFixture(t, item.action, item.kind)
			capability.ProviderTTLSeconds = 0
			assessment := AssessActiveResponseRecovery(plan, review, capability, now)
			if assessment.Status != ActiveResponseRecoveryManual || assessment.ExecutionPerformed || assessment.RollbackPerformed {
				t.Fatalf("expected documented manual recovery only: %+v", assessment)
			}
		})
	}
}

func TestActiveResponseRecoveryRejectsUnreviewedAndExecutedClaims(t *testing.T) {
	plan, review, capability, now := activeResponseRecoveryFixture(t, "isolate_workload", "workload")
	review.Status = ActiveResponseReviewPending
	r := AssessActiveResponseRecovery(plan, review, capability, now)
	if r.Status != ActiveResponseRecoveryBlocked {
		t.Fatalf("pending review accepted: %+v", r)
	}
	review.Status = ActiveResponseReviewReady
	plan.ExecutionPerformed = true
	r = AssessActiveResponseRecovery(plan, review, capability, now)
	if r.Status != ActiveResponseRecoveryBlocked {
		t.Fatalf("executed plan accepted by shadow preflight: %+v", r)
	}
}
