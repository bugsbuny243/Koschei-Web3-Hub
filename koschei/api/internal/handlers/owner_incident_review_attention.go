package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Review windows are operational attention hints, NOT SLA guarantees.
// No analyst acknowledgment, response, notification, or on-chain enforcement
// is inferred from the age of an incident.
const ownerIncidentReviewPolicyEnv = "KOSCHEI_SOC_REVIEW_POLICY_JSON"
const ownerIncidentReviewPolicyClock = "elapsed_24x7"
const ownerIncidentReviewPolicyMaxBytes = 4096
const ownerIncidentReviewTargetMaxSeconds int64 = 7 * 24 * 60 * 60

type ownerIncidentReviewPolicy struct {
	PolicyID       string           `json:"policy_id"`
	EffectiveAt    time.Time        `json:"effective_at"`
	Timezone       string           `json:"timezone"`
	Clock          string           `json:"clock"`
	TargetsSeconds map[string]int64 `json:"targets_seconds"`
}

type ownerIncidentReviewAttention struct {
	PolicyID       string     `json:"policy_id"`
	State          string     `json:"state"`
	TargetSeconds  *int64     `json:"target_seconds,omitempty"`
	DeadlineAt     *time.Time `json:"review_at,omitempty"`
	ElapsedSeconds *int64     `json:"elapsed_seconds,omitempty"`
	OverSeconds    *int64     `json:"over_target_seconds,omitempty"`
}

func validOwnerReviewPolicyID(s string) bool {
	if len(s) < 3 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		allowed := b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
		if i == 0 && !allowed {
			return false
		}
		if !allowed && b != '-' && b != '_' && b != '.' {
			return false
		}
	}
	return true
}

// A policy is deliberately absent unless an operator sets all five explicit
// targets and an effective timestamp. Unexpected keys and malformed data
// fail closed so an invalid policy cannot silently produce a healthy queue.
func parseOwnerIncidentReviewPolicy(raw string) (*ownerIncidentReviewPolicy, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if len(raw) > ownerIncidentReviewPolicyMaxBytes {
		return nil, errors.New("SOC review policy is too large")
	}
	var body struct {
		PolicyID       string           `json:"policy_id"`
		EffectiveAt    string           `json:"effective_at"`
		Timezone       string           `json:"timezone"`
		Clock          string           `json:"clock"`
		TargetsSeconds map[string]int64 `json:"targets_seconds"`
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return nil, fmt.Errorf("invalid SOC review policy JSON: %w", err)
	}
	var additional any
	if err := decoder.Decode(&additional); err != io.EOF {
		return nil, errors.New("SOC review policy must be a single JSON object")
	}
	if !validOwnerReviewPolicyID(body.PolicyID) ||
		body.Timezone != "UTC" || body.Clock != ownerIncidentReviewPolicyClock ||
		!strings.HasSuffix(body.EffectiveAt, "Z") {
		return nil, errors.New("SOC review policy identity, UTC timezone, clock or effective time is invalid")
	}
	effectiveAt, err := time.Parse(time.RFC3339, body.EffectiveAt)
	if err != nil || effectiveAt.IsZero() {
		return nil, errors.New("SOC review policy effective_at must be an explicit UTC instant")
	}
	// Explicit policy completeness is required: no unspecified severity gets
	// an invented default and no extra keys can mask misspelled severities.
	if len(body.TargetsSeconds) != 5 {
		return nil, errors.New("SOC review policy must contain exactly five severity targets")
	}
	for _, severity := range []string{"critical", "high", "medium", "low", "info"} {
		seconds, ok := body.TargetsSeconds[severity]
		if !ok || seconds < 60 || seconds > ownerIncidentReviewTargetMaxSeconds {
			return nil, fmt.Errorf("SOC review policy target for %s is missing or out of range", severity)
		}
	}
	// A less severe case must not have a stricter attention window by accident.
	ranks := []string{"critical", "high", "medium", "low", "info"}
	for i := 1; i < len(ranks); i++ {
		if body.TargetsSeconds[ranks[i]] < body.TargetsSeconds[ranks[i-1]] {
			return nil, errors.New("SOC review policy severity windows must be monotonic")
		}
	}
	return &ownerIncidentReviewPolicy{
		PolicyID: body.PolicyID, EffectiveAt: effectiveAt.UTC(),
		Timezone: "UTC", Clock: ownerIncidentReviewPolicyClock,
		TargetsSeconds: body.TargetsSeconds,
	}, nil
}

func configuredOwnerIncidentReviewPolicy() (*ownerIncidentReviewPolicy, error) {
	return parseOwnerIncidentReviewPolicy(os.Getenv(ownerIncidentReviewPolicyEnv))
}

func ownerIncidentReviewAttentionForCase(item ownerIncidentTriageItem, now time.Time, policy *ownerIncidentReviewPolicy) *ownerIncidentReviewAttention {
	if policy == nil {
		return nil
	}
	result := &ownerIncidentReviewAttention{PolicyID: policy.PolicyID}
	created := item.Incident.CreatedAt
	if !item.Active {
		result.State = "inactive_not_measured"
		return result
	}
	if created.IsZero() || created.After(now) {
		result.State = "invalid_creation_time_no_review_claim"
		return result
	}
	if now.Before(policy.EffectiveAt) {
		result.State = "policy_not_yet_effective"
		return result
	}
	if created.Before(policy.EffectiveAt) {
		result.State = "case_precedes_policy_not_retroactive"
		return result
	}
	target, ok := policy.TargetsSeconds[item.Incident.Severity]
	if !ok {
		result.State = "severity_not_in_policy"
		return result
	}
	deadline := created.Add(time.Duration(target) * time.Second)
	elapsed := incidentElapsedSeconds(created, now)
	result.TargetSeconds = &target
	result.DeadlineAt = &deadline
	result.ElapsedSeconds = &elapsed
	if now.After(deadline) {
		over := incidentElapsedSeconds(deadline, now)
		result.State = "review_window_elapsed_not_sla_breach"
		result.OverSeconds = &over
	} else {
		result.State = "within_review_window_not_sla_guarantee"
	}
	return result
}

func applyOwnerIncidentReviewPolicy(items []ownerIncidentTriageItem, now time.Time, policy *ownerIncidentReviewPolicy) {
	if policy == nil {
		return
	}
	for i := range items {
		items[i].ReviewAttention = ownerIncidentReviewAttentionForCase(items[i], now, policy)
	}
}
