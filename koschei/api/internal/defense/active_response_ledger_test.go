package defense

import (
	"context"
	"strings"
	"testing"
	"time"
)

func activeResponseLedgerFixture(t *testing.T) (
	ActiveResponsePlan, ActiveResponseReview, ActiveResponseRecoveryAssessment, time.Time,
) {
	t.Helper()
	plan, review, capability, now := activeResponseRecoveryFixture(t, "bgp_blackhole", "protected_prefix")
	recovery := AssessActiveResponseRecovery(plan, review, capability, now)
	if recovery.Status != ActiveResponseRecoveryReview {
		t.Fatalf("unexpected recovery fixture: %+v", recovery)
	}
	return plan, review, recovery, now
}

func TestActiveResponseLedgerOfflineHashChainIsDeterministic(t *testing.T) {
	plan, review, recovery, now := activeResponseLedgerFixture(t)
	a, err := BuildShadowLedgerEntry(plan, review, recovery, "", now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildShadowLedgerEntry(plan, review, recovery, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if a.ChainHash != b.ChainHash ||
		a.ChainHash == a.PreviousHash ||
		!validActiveResponseSHA256(a.ChainHash) ||
		a.ExecutionPerformed {
		t.Fatalf("invalid or non-deterministic shadow chain entry: %+v vs %+v", a, b)
	}
	plan2 := plan
	plan2.ActionID = "another-action"
	review2 := review
	review2.ActionID = plan2.ActionID
	recovery2 := recovery
	recovery2.ActionID = plan2.ActionID
	next, err := BuildShadowLedgerEntry(plan2, review2, recovery2, a.ChainHash, now.Add(time.Second))
	if err != nil || next.PreviousHash != a.ChainHash || next.ChainHash == a.ChainHash {
		t.Fatalf("shadow chain linkage failed: %+v %v", next, err)
	}
}

func TestActiveResponseLedgerRejectsNonShadowAndMismatchedReview(t *testing.T) {
	plan, review, recovery, now := activeResponseLedgerFixture(t)
	plan.ExecutionPerformed = true
	if _, err := BuildShadowLedgerEntry(plan, review, recovery, "", now); err == nil {
		t.Fatal("executed plan entered shadow ledger")
	}
	plan.ExecutionPerformed = false
	review.Status = ActiveResponseReviewPending
	if _, err := BuildShadowLedgerEntry(plan, review, recovery, "", now); err == nil {
		t.Fatal("unapproved review entered shadow ledger")
	}
	review.Status = ActiveResponseReviewReady
	recovery.ProviderID = "foreign-provider"
	if _, err := BuildShadowLedgerEntry(plan, review, recovery, "", now); err == nil {
		t.Fatal("provider mismatch entered shadow ledger")
	}
	recovery.ProviderID = review.ProviderID
	recovery.Requirements = []string{"missing break glass"}
	if _, err := BuildShadowLedgerEntry(plan, review, recovery, "", now); err == nil {
		t.Fatal("incomplete recovery entered shadow ledger")
	}
}

func TestActiveResponseLedgerRequiresValidChainAndTrustedTime(t *testing.T) {
	plan, review, recovery, now := activeResponseLedgerFixture(t)
	if _, err := BuildShadowLedgerEntry(plan, review, recovery, "sha256:broken", now); err == nil {
		t.Fatal("invalid chain hash accepted")
	}
	if _, err := BuildShadowLedgerEntry(plan, review, recovery, "", time.Time{}); err == nil {
		t.Fatal("missing recording timestamp accepted")
	}
	plan.TenantID = "foreign\ntenant"
	if _, err := BuildShadowLedgerEntry(plan, review, recovery, "", now); err == nil {
		t.Fatal("untrusted tenant identifier accepted")
	}
	plan.TenantID = strings.Repeat("x", 129)
	if _, err := BuildShadowLedgerEntry(plan, review, recovery, "", now); err == nil {
		t.Fatal("unbounded tenant identifier accepted")
	}
}

func TestActiveResponseLedgerDoesNotWriteWithoutStorage(t *testing.T) {
	plan, review, recovery, now := activeResponseLedgerFixture(t)
	result, err := recordShadowLedger(context.Background(), nil, plan, review, recovery, now)
	if err == nil || result.ExecutionPerformed {
		t.Fatalf("nil DB must fail safely: %+v %v", result, err)
	}
}
