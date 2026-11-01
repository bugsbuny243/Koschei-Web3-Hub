package defense

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func signedActiveResponseWitness(t *testing.T, plan ActiveResponsePlan, review ActiveResponseReview, provider ActiveResponseProviderBinding, now time.Time) (ActiveResponseWitness, ed25519.PublicKey) {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	hash := sha256.Sum256([]byte("independent observer fixture"))
	w := ActiveResponseWitness{
		ObserverID:     "independent-operator",
		ProviderID:     provider.ProviderID,
		PlanDigest:     review.PlanDigest,
		ActionID:       plan.ActionID,
		TenantID:       plan.TenantID,
		TargetID:       plan.TargetID,
		ObservedState:  plan.ExpectedEffect,
		EvidenceDigest: "sha256:" + hex.EncodeToString(hash[:]),
		ScopeComplete:  true,
		ObservedAt:     now.Add(-time.Second),
	}
	payload, err := ActiveResponseWitnessSigningPayload(w)
	if err != nil {
		t.Fatal(err)
	}
	w.Signature = ed25519.Sign(private, payload)
	return w, private.Public().(ed25519.PublicKey)
}

func TestActiveResponseWitnessRemainsNonExecutingDespiteMatchingEvidence(t *testing.T) {
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "terminate_session", "session")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	review := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, nil, nil, now)
	if review.Status != ActiveResponseReviewReady {
		t.Fatalf("unexpected review: %+v", review)
	}
	witness, key := signedActiveResponseWitness(t, plan, review, provider, now)
	result := AssessActiveResponseShadowWitness(plan, review, provider, witness, map[string]ed25519.PublicKey{witness.ObserverID: key}, now)
	if result.Status != ActiveResponseWitnessConsistent || !result.EvidenceConsistent ||
		result.EffectAgreement != ControlEffectNotExercised || result.ExecutionPerformed {
		t.Fatalf("witness improperly claimed execution or effect: %+v", result)
	}
}

func TestActiveResponseWitnessRejectsForgedOrMisboundEvidence(t *testing.T) {
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "revoke_api_key", "api_key")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	review := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, nil, nil, now)
	witness, key := signedActiveResponseWitness(t, plan, review, provider, now)
	keys := map[string]ed25519.PublicKey{witness.ObserverID: key}
	witness.ObservedState = "tampered"
	result := AssessActiveResponseShadowWitness(plan, review, provider, witness, keys, now)
	if result.Status != ActiveResponseWitnessRejected {
		t.Fatalf("tampered signature passed: %+v", result)
	}
	witness, _ = signedActiveResponseWitness(t, plan, review, provider, now)
	witness.TenantID = "foreign"
	result = AssessActiveResponseShadowWitness(plan, review, provider, witness, keys, now)
	if result.Status != ActiveResponseWitnessRejected {
		t.Fatalf("cross-tenant witness passed: %+v", result)
	}
	witness, _ = signedActiveResponseWitness(t, plan, review, provider, now)
	result = AssessActiveResponseShadowWitness(plan, review, provider, witness, nil, now)
	if result.Status != ActiveResponseWitnessRejected {
		t.Fatalf("untrusted observer passed: %+v", result)
	}
}

func TestActiveResponseWitnessDistinguishesCoverageAndDisagreement(t *testing.T) {
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "block_transaction", "transaction_policy")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	review := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, nil, nil, now)
	witness, key := signedActiveResponseWitness(t, plan, review, provider, now)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	keys := map[string]ed25519.PublicKey{witness.ObserverID: key}

	witness.ScopeComplete = false
	witness.CoverageGaps = []string{"provider-propagation-unknown"}
	payload, err := ActiveResponseWitnessSigningPayload(witness)
	if err != nil {
		t.Fatal(err)
	}
	witness.Signature = ed25519.Sign(private, payload)
	result := AssessActiveResponseShadowWitness(plan, review, provider, witness, keys, now)
	if result.Status != ActiveResponseWitnessInsufficient || result.EvidenceConsistent ||
		result.EffectAgreement != ControlEffectNotExercised {
		t.Fatalf("coverage gap reported as verified effect: %+v", result)
	}

	witness.ScopeComplete = true
	witness.CoverageGaps = nil
	witness.ObservedState = "different-state"
	payload, err = ActiveResponseWitnessSigningPayload(witness)
	if err != nil {
		t.Fatal(err)
	}
	witness.Signature = ed25519.Sign(private, payload)
	result = AssessActiveResponseShadowWitness(plan, review, provider, witness, keys, now)
	if result.Status != ActiveResponseWitnessConflicting || result.EvidenceConsistent {
		t.Fatalf("conflicting shadow evidence accepted: %+v", result)
	}
}

func TestActiveResponseWitnessRejectsStaleOrWrongProvider(t *testing.T) {
	policy, finding, req, asset, plan, provider := activeResponseReviewFixture(t, "terminate_session", "session")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	review := ReviewActiveResponse(policy, finding, req, finding.EvidenceIDs, asset, plan, provider, nil, nil, now)
	witness, key := signedActiveResponseWitness(t, plan, review, provider, now)
	keys := map[string]ed25519.PublicKey{witness.ObserverID: key}
	result := AssessActiveResponseShadowWitness(plan, review, provider, witness, keys, now.Add(6*time.Minute))
	if result.Status != ActiveResponseWitnessRejected {
		t.Fatalf("stale witness accepted: %+v", result)
	}
	provider.ProviderID = "unrelated-provider"
	result = AssessActiveResponseShadowWitness(plan, review, provider, witness, keys, now)
	if result.Status != ActiveResponseWitnessRejected {
		t.Fatalf("provider-bound witness accepted through unrelated provider: %+v", result)
	}
}
