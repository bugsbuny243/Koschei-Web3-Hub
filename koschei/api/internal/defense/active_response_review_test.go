package defense

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"
)

func activeResponseReviewFixture(t *testing.T, action, kind string) (
	DefensivePolicy, SecurityFinding, DefensiveActionRequest, ProtectedAsset, ActiveResponsePlan, ActiveResponseProviderBinding,
) {
	t.Helper()
	policy, finding, req, asset := activeResponseFixture(action, kind)
	plan, err := ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || plan.Status != ActiveResponseShadowProposed {
		t.Fatalf("invalid review fixture: %+v %v", plan, err)
	}
	provider := ActiveResponseProviderBinding{
		ProviderID: "provider-a", TenantID: req.TenantID, TargetID: req.TargetID,
		Action: action, Capability: plan.ProviderCapability, Enabled: true,
	}
	return policy, finding, req, asset, plan, provider
}

func signActiveResponseApproval(t *testing.T, seed byte, id, digest string, now time.Time) (ActiveResponseApproval, ed25519.PublicKey) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	a := ActiveResponseApproval{
		ApproverID: id, PlanDigest: digest,
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
	}
	payload, err := ActiveResponseApprovalSigningPayload(a)
	if err != nil {
		t.Fatal(err)
	}
	a.Signature = ed25519.Sign(key, payload)
	return a, key.Public().(ed25519.PublicKey)
}

func TestActiveResponseReviewNoSideEffects(t *testing.T) {
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "terminate_session", "session")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	r := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, nil, nil, now)
	if r.Status != ActiveResponseReviewReady || r.ExecutionPerformed || r.RequiredApprovals != 0 || len(r.PlanDigest) != 71 {
		t.Fatalf("expected review-ready without execution: %+v", r)
	}
}

func TestActiveResponseHighImpactRequiresTwoDistinctSignatures(t *testing.T) {
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "bgp_blackhole", "protected_prefix")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	digest, err := ActiveResponsePlanDigest(policy, finding, req, asset, plan)
	if err != nil {
		t.Fatal(err)
	}
	a1, key1 := signActiveResponseApproval(t, 1, "operator-a", digest, now)
	a2, key2 := signActiveResponseApproval(t, 2, "operator-b", digest, now)
	keys := map[string]ed25519.PublicKey{"operator-a": key1, "operator-b": key2}
	none := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, nil, keys, now)
	if none.Status != ActiveResponseReviewPending || none.RequiredApprovals != 2 {
		t.Fatalf("missing two-human gate: %+v", none)
	}
	one := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, []ActiveResponseApproval{a1}, keys, now)
	if one.Status != ActiveResponseReviewPending || one.VerifiedApprovals != 1 {
		t.Fatalf("single human approved BGP action: %+v", one)
	}
	two := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, []ActiveResponseApproval{a1, a2}, keys, now)
	if two.Status != ActiveResponseReviewReady || two.VerifiedApprovals != 2 || two.ExecutionPerformed {
		t.Fatalf("two-human review not evaluated properly: %+v", two)
	}
	dup := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, []ActiveResponseApproval{a1, a1}, keys, now)
	if dup.Status != ActiveResponseReviewDenied {
		t.Fatalf("duplicate approver accepted: %+v", dup)
	}
}

func TestActiveResponseReviewRejectsTamperedApprovalAndProvider(t *testing.T) {
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "pause_contract", "smart_contract")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	digest, err := ActiveResponsePlanDigest(policy, finding, req, asset, plan)
	if err != nil {
		t.Fatal(err)
	}
	a1, key1 := signActiveResponseApproval(t, 3, "reviewer-one", digest, now)
	a2, key2 := signActiveResponseApproval(t, 4, "reviewer-two", digest, now)
	keys := map[string]ed25519.PublicKey{"reviewer-one": key1, "reviewer-two": key2}
	valid := []ActiveResponseApproval{a1, a2}
	provider.Enabled = false
	r := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, valid, keys, now)
	if r.Status != ActiveResponseReviewDenied {
		t.Fatalf("disabled provider accepted: %+v", r)
	}
	provider.Enabled = true
	provider.TenantID = "another-tenant"
	r = ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, valid, keys, now)
	if r.Status != ActiveResponseReviewDenied {
		t.Fatalf("cross-tenant provider accepted: %+v", r)
	}
	provider.TenantID = req.TenantID
	a2.Signature[0] ^= 0x01
	r = ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, []ActiveResponseApproval{a1, a2}, keys, now)
	if r.Status != ActiveResponseReviewDenied {
		t.Fatalf("forged signature accepted: %+v", r)
	}
	r = ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, []ActiveResponseApproval{a1}, keys, now.Add(2*time.Minute))
	if r.Status != ActiveResponseReviewDenied {
		t.Fatalf("expired signature accepted: %+v", r)
	}
}

func TestActiveResponseReviewRejectsForgedPlanAndInput(t *testing.T) {
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "revoke_api_key", "api_key")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	plan.Policy.Reason = "forged-policy"
	r := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, nil, nil, now)
	if r.Status != ActiveResponseReviewDenied {
		t.Fatalf("modified plan accepted: %+v", r)
	}
	plan.Policy.Reason = "authorized defensive scope"
	req.Parameters = map[string]string{"resource": "asset-b"}
	r = ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, nil, nil, now)
	if r.Status != ActiveResponseReviewReady {
		t.Fatalf("new parameters should be included in the recomputed plan digest: %+v", r)
	}
	d1, err := ActiveResponsePlanDigest(policy, finding, req, asset, plan)
	if err != nil {
		t.Fatal(err)
	}
	req.Parameters["resource"] = "asset-c"
	d2, err := ActiveResponsePlanDigest(policy, finding, req, asset, plan)
	if err != nil || d1 == d2 {
		t.Fatalf("request parameter mutation did not change digest: %s, %s, %v", d1, d2, err)
	}
}

func TestActiveResponseApprovalSigningRejectsOversizedValidity(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	approval := ActiveResponseApproval{
		ApproverID: "person", PlanDigest: "sha256:" + string(bytes.Repeat([]byte("a"), 64)),
		IssuedAt: now, ExpiresAt: now.Add(6 * time.Minute),
	}
	if _, err := ActiveResponseApprovalSigningPayload(approval); err == nil {
		t.Fatal("long-lived approval accepted")
	}
}


func TestActiveResponseHighImpactRejectsTwoIDsSharingOneSigningKey(t *testing.T) {
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "bgp_blackhole", "protected_prefix")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	digest, err := ActiveResponsePlanDigest(policy, finding, req, asset, plan)
	if err != nil {
		t.Fatal(err)
	}
	one, keyOne := signActiveResponseApproval(t, 31, "signer-one", digest, now)
	two, keyTwo := signActiveResponseApproval(t, 31, "signer-two", digest, now)
	keys := map[string]ed25519.PublicKey{"signer-one": keyOne, "signer-two": keyTwo}
	review := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, []ActiveResponseApproval{one, two}, keys, now)
	if review.Status != ActiveResponseReviewDenied || review.ExecutionPerformed {
		t.Fatalf("reused credential accepted for dual approval: %+v", review)
	}
}
