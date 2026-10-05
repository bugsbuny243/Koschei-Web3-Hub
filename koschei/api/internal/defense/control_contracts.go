package defense

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Defensive control contracts are deliberately model- and vendor-neutral.
// They describe what Sentinel may propose and what deterministic code must
// verify before a defensive action can ever leave shadow mode.

type SecurityFinding struct {
	FindingID      string   `json:"finding_id"`
	Severity       string   `json:"severity"`
	Confidence     float64  `json:"confidence"`
	EvidenceIDs    []string `json:"evidence_ids"`
	DefensiveAction string  `json:"defensive_action"`
}

type DefensiveActionRequest struct {
	ActionID    string            `json:"action_id"`
	TenantID    string            `json:"tenant_id"`
	TargetID    string            `json:"target_id"`
	Action      string            `json:"action"`
	FindingID   string            `json:"finding_id"`
	EvidenceIDs []string          `json:"evidence_ids"`
	Reason      string            `json:"reason"`
	Parameters  map[string]string `json:"parameters"`
}

type DefensivePolicy struct {
	TenantID       string   `json:"tenant_id"`
	AllowedActions []string `json:"allowed_actions"`
	AllowedTargets []string `json:"allowed_targets"`
	HumanGateActions []string `json:"human_gate_actions"`
}

type PolicyDecision struct {
	Allowed               bool   `json:"allowed"`
	RequiresHumanApproval bool   `json:"requires_human_approval"`
	Reason                string `json:"reason"`
}

const (
	EffectAgree        = "agree"
	EffectDisagree     = "disagree"
	EffectIndeterminate = "indeterminate"
	EffectNotExercised = "not_exercised"
)

type EffectObservation struct {
	ActionID       string    `json:"action_id"`
	TenantID       string    `json:"tenant_id"`
	TargetID       string    `json:"target_id"`
	ObserverID     string    `json:"observer_id"`
	ExpectedState  string    `json:"expected_state"`
	ObservedState  string    `json:"observed_state"`
	EvidenceDigest string    `json:"evidence_digest"`
	ScopeComplete  bool      `json:"scope_complete"`
	CoverageGaps   []string  `json:"coverage_gaps"`
	ObservedAt     time.Time `json:"observed_at"`
	Agreement      string    `json:"agreement"`
}

type DefensiveReceipt struct {
	SchemaVersion  string             `json:"schema_version"`
	Action         DefensiveActionRequest `json:"action"`
	Policy         PolicyDecision     `json:"policy"`
	Observation    EffectObservation  `json:"observation"`
	IssuedAt       time.Time          `json:"issued_at"`
	ReceiptHash    string             `json:"receipt_hash"`
}

func ValidateFinding(f SecurityFinding, knownEvidence []string) error {
	if strings.TrimSpace(f.FindingID) == "" {
		return errors.New("finding_id is required")
	}
	if f.Confidence < 0 || f.Confidence > 1 {
		return errors.New("confidence must be between 0 and 1")
	}
	if len(f.EvidenceIDs) == 0 {
		return errors.New("finding must be evidence-bound")
	}
	known := stringSet(knownEvidence)
	seen := map[string]struct{}{}
	for _, id := range f.EvidenceIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return errors.New("empty evidence id")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate evidence id %q", id)
		}
		seen[id] = struct{}{}
		if _, ok := known[id]; !ok {
			return fmt.Errorf("unknown evidence id %q", id)
		}
	}
	return nil
}

func EvaluateDefensivePolicy(policy DefensivePolicy, req DefensiveActionRequest) PolicyDecision {
	if strings.TrimSpace(req.TenantID) == "" || req.TenantID != strings.TrimSpace(policy.TenantID) {
		return PolicyDecision{Reason: "tenant boundary mismatch"}
	}
	if !contains(policy.AllowedActions, req.Action) {
		return PolicyDecision{Reason: "action is outside the defensive policy"}
	}
	if !contains(policy.AllowedTargets, req.TargetID) {
		return PolicyDecision{Reason: "target is outside the authorized protected boundary"}
	}
	if strings.TrimSpace(req.FindingID) == "" || len(req.EvidenceIDs) == 0 {
		return PolicyDecision{Reason: "action is not bound to a finding and evidence"}
	}
	return PolicyDecision{
		Allowed: true,
		RequiresHumanApproval: contains(policy.HumanGateActions, req.Action),
		Reason: "authorized defensive scope",
	}
}

func DeriveEffectAgreement(observation EffectObservation) string {
	if !observation.ScopeComplete || len(observation.CoverageGaps) > 0 || strings.TrimSpace(observation.ObservedState) == "" {
		return EffectIndeterminate
	}
	if strings.TrimSpace(observation.ExpectedState) == "" {
		return EffectNotExercised
	}
	if observation.ExpectedState == observation.ObservedState {
		return EffectAgree
	}
	return EffectDisagree
}

func BuildDefensiveReceipt(req DefensiveActionRequest, decision PolicyDecision, observation EffectObservation, now time.Time) (DefensiveReceipt, error) {
	if !decision.Allowed {
		return DefensiveReceipt{}, errors.New("cannot issue receipt for denied action")
	}
	if observation.ActionID != req.ActionID || observation.TenantID != req.TenantID || observation.TargetID != req.TargetID {
		return DefensiveReceipt{}, errors.New("effect observation does not bind to action boundary")
	}
	observation.Agreement = DeriveEffectAgreement(observation)
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	receipt := DefensiveReceipt{
		SchemaVersion: "koschei-sentinel-defensive-receipt-v1",
		Action: req,
		Policy: decision,
		Observation: observation,
		IssuedAt: now,
	}
	receipt.ReceiptHash = hashValue(struct {
		SchemaVersion string `json:"schema_version"`
		Action DefensiveActionRequest `json:"action"`
		Policy PolicyDecision `json:"policy"`
		Observation EffectObservation `json:"observation"`
		IssuedAt time.Time `json:"issued_at"`
	}{receipt.SchemaVersion, receipt.Action, receipt.Policy, receipt.Observation, receipt.IssuedAt})
	return receipt, nil
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func contains(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func SortedEvidenceIDs(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}
