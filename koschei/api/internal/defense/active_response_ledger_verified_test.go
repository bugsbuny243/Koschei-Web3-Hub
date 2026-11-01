package defense

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

func trustedShadowLedgerFixture(t *testing.T) (TrustedShadowLedgerInputs, time.Time) {
	t.Helper()
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "bgp_blackhole", "protected_prefix")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	digest, err := ActiveResponsePlanDigest(policy, finding, req, asset, plan)
	if err != nil {
		t.Fatal(err)
	}
	approvalA, keyA := signActiveResponseApproval(t, 21, "operator-a", digest, now)
	approvalB, keyB := signActiveResponseApproval(t, 22, "operator-b", digest, now)
	input := TrustedShadowLedgerInputs{
		Policy:           policy,
		Finding:          finding,
		Request:          req,
		KnownEvidence:    append([]string(nil), finding.EvidenceIDs...),
		Asset:            asset,
		Provider:         provider,
		Approvals:        []ActiveResponseApproval{approvalA, approvalB},
		TrustedApprovers: map[string]ed25519.PublicKey{"operator-a": keyA, "operator-b": keyB},
		Recovery: ActiveResponseRecoveryCapability{
			ProviderID:             provider.ProviderID,
			TenantID:               req.TenantID,
			TargetID:               req.TargetID,
			Action:                 req.Action,
			RunbookRef:             "operator-approved-runbook-v1",
			RecoveryOwnerID:        "recovery-operator",
			IndependentObserverID:  "independent-observer",
			ManagementPathExcluded: true,
			BreakGlassVerified:     true,
			RollbackTestAttested:   true,
			RollbackTestedAt:       now.Add(-time.Hour),
			ProviderTTLSeconds:     plan.MaxDurationSeconds,
		},
	}
	return input, now
}

func TestActiveResponseVerifiedLedgerPreflightRechecksWholeChain(t *testing.T) {
	input, now := trustedShadowLedgerFixture(t)
	preflight, err := PrepareVerifiedShadowLedger(input, now)
	if err != nil || preflight.Plan.Status != ActiveResponseShadowProposed ||
		preflight.Review.Status != ActiveResponseReviewReady ||
		preflight.Recovery.Status != ActiveResponseRecoveryReview ||
		preflight.Plan.ExecutionPerformed || preflight.Review.ExecutionPerformed ||
		preflight.Recovery.ExecutionPerformed {
		t.Fatalf("verified offline preflight failed or claimed execution: %+v %v", preflight, err)
	}
}

func TestActiveResponseVerifiedLedgerPreflightRejectsTamperedSources(t *testing.T) {
	input, now := trustedShadowLedgerFixture(t)
	input.Approvals[0].Signature[0] ^= 1
	if _, err := PrepareVerifiedShadowLedger(input, now); err == nil {
		t.Fatal("invalid human signature entered ledger preflight")
	}
	input, now = trustedShadowLedgerFixture(t)
	input.Provider.TenantID = "foreign-tenant"
	if _, err := PrepareVerifiedShadowLedger(input, now); err == nil {
		t.Fatal("foreign provider accepted")
	}
	input, now = trustedShadowLedgerFixture(t)
	input.Asset.Managed = false
	if _, err := PrepareVerifiedShadowLedger(input, now); err == nil {
		t.Fatal("unmanaged target accepted")
	}
	input, now = trustedShadowLedgerFixture(t)
	input.KnownEvidence = nil
	if _, err := PrepareVerifiedShadowLedger(input, now); err == nil {
		t.Fatal("unknown evidence accepted")
	}
}

func TestActiveResponseVerifiedLedgerPreflightRequiresRecoveryEvidence(t *testing.T) {
	input, now := trustedShadowLedgerFixture(t)
	input.Recovery.BreakGlassVerified = false
	if _, err := PrepareVerifiedShadowLedger(input, now); err == nil {
		t.Fatal("unverified recovery break-glass accepted")
	}
	input, now = trustedShadowLedgerFixture(t)
	input.Recovery.ProviderTTLSeconds = input.Recovery.ProviderTTLSeconds + 1
	if _, err := PrepareVerifiedShadowLedger(input, now); err == nil {
		t.Fatal("unbounded network action accepted")
	}
}

func TestActiveResponseVerifiedLedgerNeverWritesWithoutStorage(t *testing.T) {
	input, now := trustedShadowLedgerFixture(t)
	result, err := RecordVerifiedShadowLedger(context.Background(), nil, input, now)
	if err == nil || result.ExecutionPerformed {
		t.Fatalf("no storage must fail closed: %+v %v", result, err)
	}
}
