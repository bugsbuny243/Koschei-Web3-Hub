package defense

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Shadow observation verifies signed evidence formatting and consistency.
// It deliberately never asserts a real-world effect: the planner has no
// execution mechanism and no durable provider execution receipt.
const ActiveResponseWitnessSchemaVersion = "koschei-active-response-witness-v1"

const (
	ActiveResponseWitnessRejected     = "rejected"
	ActiveResponseWitnessInsufficient = "insufficient_coverage"
	ActiveResponseWitnessConsistent   = "shadow_evidence_consistent"
	ActiveResponseWitnessConflicting  = "shadow_evidence_conflicting"
)

type ActiveResponseWitness struct {
	ObserverID     string    `json:"observer_id"`
	ProviderID     string    `json:"provider_id"`
	PlanDigest     string    `json:"plan_digest"`
	ActionID       string    `json:"action_id"`
	TenantID       string    `json:"tenant_id"`
	TargetID       string    `json:"target_id"`
	ObservedState  string    `json:"observed_state"`
	EvidenceDigest string    `json:"evidence_digest"`
	ScopeComplete  bool      `json:"scope_complete"`
	CoverageGaps   []string  `json:"coverage_gaps"`
	ObservedAt     time.Time `json:"observed_at"`
	Signature      []byte    `json:"signature"`
}

type ActiveResponseWitnessAssessment struct {
	SchemaVersion      string `json:"schema_version"`
	Status             string `json:"status"`
	ActionID           string `json:"action_id"`
	ObserverID         string `json:"observer_id"`
	EvidenceConsistent bool   `json:"evidence_consistent"`
	ExecutionPerformed bool   `json:"execution_performed"`
	EffectAgreement    string `json:"effect_agreement"`
	Reason             string `json:"reason"`
}

// ActiveResponseWitnessSigningPayload is a canonical, domain-separated
// message. The signature is omitted from the message being signed.
func ActiveResponseWitnessSigningPayload(w ActiveResponseWitness) ([]byte, error) {
	if strings.TrimSpace(w.ObserverID) == "" ||
		strings.TrimSpace(w.ProviderID) == "" ||
		strings.TrimSpace(w.ActionID) == "" ||
		strings.TrimSpace(w.TenantID) == "" ||
		strings.TrimSpace(w.TargetID) == "" ||
		strings.TrimSpace(w.ObservedState) == "" ||
		w.ObservedAt.IsZero() ||
		!validActiveResponseSHA256(w.PlanDigest) ||
		!validActiveResponseSHA256(w.EvidenceDigest) {
		return nil, errors.New("witness identity, scope, state, timestamp and SHA-256 evidence are required")
	}
	return json.Marshal(struct {
		SchemaVersion   string    `json:"schema_version"`
		ObserverID      string    `json:"observer_id"`
		ProviderID      string    `json:"provider_id"`
		PlanDigest      string    `json:"plan_digest"`
		ActionID        string    `json:"action_id"`
		TenantID        string    `json:"tenant_id"`
		TargetID        string    `json:"target_id"`
		ObservedState   string    `json:"observed_state"`
		EvidenceDigest  string    `json:"evidence_digest"`
		ScopeComplete   bool      `json:"scope_complete"`
		CoverageGaps    []string  `json:"coverage_gaps"`
		ObservedAt      time.Time `json:"observed_at"`
	}{
		ActiveResponseWitnessSchemaVersion,
		w.ObserverID, w.ProviderID, w.PlanDigest, w.ActionID,
		w.TenantID, w.TargetID, w.ObservedState, w.EvidenceDigest,
		w.ScopeComplete, w.CoverageGaps, w.ObservedAt.UTC(),
	})
}

func validActiveResponseSHA256(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	raw, err := hex.DecodeString(value[len("sha256:"):])
	return err == nil && len(raw) == sha256.Size && value == "sha256:"+hex.EncodeToString(raw)
}

// AssessActiveResponseShadowWitness only authenticates an offline observer's
// assertion. Identity independence and physical-world observation quality
// require a trusted inventory and cannot be proven from signatures alone.
// This must never be interpreted as a verified post-execution receipt.
func AssessActiveResponseShadowWitness(
	plan ActiveResponsePlan,
	review ActiveResponseReview,
	provider ActiveResponseProviderBinding,
	witness ActiveResponseWitness,
	observerKeys map[string]ed25519.PublicKey,
	now time.Time,
) ActiveResponseWitnessAssessment {
	out := ActiveResponseWitnessAssessment{
		SchemaVersion:      ActiveResponseWitnessSchemaVersion,
		Status:             ActiveResponseWitnessRejected,
		ActionID:           plan.ActionID,
		ObserverID:         witness.ObserverID,
		EffectAgreement:    ControlEffectNotExercised,
		ExecutionPerformed: false,
	}
	if now.IsZero() ||
		plan.SchemaVersion != ActiveResponseSchemaVersion ||
		plan.Status != ActiveResponseShadowProposed ||
		plan.ExecutionPerformed ||
		plan.EffectAgreement != ControlEffectNotExercised ||
		review.SchemaVersion != ActiveResponseReviewSchemaVersion ||
		review.Status != ActiveResponseReviewReady ||
		review.ExecutionPerformed ||
		review.ActionID != plan.ActionID ||
		!validActiveResponseSHA256(review.PlanDigest) {
		out.Reason = "verified, non-executing shadow review is required"
		return out
	}
	if !provider.Enabled || strings.TrimSpace(provider.ProviderID) == "" ||
		provider.ProviderID != review.ProviderID ||
		provider.TenantID != plan.TenantID ||
		provider.TargetID != plan.TargetID ||
		provider.Action != plan.Action ||
		provider.Capability != plan.ProviderCapability ||
		witness.ProviderID != provider.ProviderID ||
		witness.PlanDigest != review.PlanDigest ||
		witness.ActionID != plan.ActionID ||
		witness.TenantID != plan.TenantID ||
		witness.TargetID != plan.TargetID ||
		witness.ObserverID == provider.ProviderID {
		out.Reason = "witness and provider do not match the reviewed protected scope"
		return out
	}
	if witness.ObservedAt.IsZero() || witness.ObservedAt.After(now) ||
		now.Sub(witness.ObservedAt) > 5*time.Minute {
		out.Reason = "witness is stale or future-dated"
		return out
	}
	key, ok := observerKeys[witness.ObserverID]
	if !ok || len(key) != ed25519.PublicKeySize {
		out.Reason = "observer is not in the trusted registry"
		return out
	}
	payload, err := ActiveResponseWitnessSigningPayload(witness)
	if err != nil || !ed25519.Verify(key, payload, witness.Signature) {
		out.Reason = "witness has invalid scope, evidence or signature"
		return out
	}
	if !witness.ScopeComplete || len(witness.CoverageGaps) != 0 {
		out.Status = ActiveResponseWitnessInsufficient
		out.Reason = "signed shadow witness has incomplete coverage"
		return out
	}
	if witness.ObservedState != plan.ExpectedEffect {
		out.Status = ActiveResponseWitnessConflicting
		out.Reason = "signed shadow witness differs from expected state"
		return out
	}
	out.Status = ActiveResponseWitnessConsistent
	out.EvidenceConsistent = true
	out.Reason = "offline witness assertion matches plan; no effect or live execution has been verified"
	return out
}
