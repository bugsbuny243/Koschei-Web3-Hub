package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func validReviewPolicyJSON(effectiveAt string) string {
	return fmt.Sprintf(`{"policy_id":"review-2026-v1","effective_at":%q,"timezone":"UTC","clock":"elapsed_24x7","targets_seconds":{"critical":900,"high":3600,"medium":14400,"low":43200,"info":86400}}`, effectiveAt)
}

func TestSOCReviewAttentionPolicyOptInStrictValidation(t *testing.T) {
	policy, err := parseOwnerIncidentReviewPolicy("")
	if err != nil || policy != nil {
		t.Fatalf("unset policy cannot create review promises: policy=%#v err=%v", policy, err)
	}
	policy, err = parseOwnerIncidentReviewPolicy(validReviewPolicyJSON("2026-10-29T00:00:00Z"))
	if err != nil || policy == nil || policy.PolicyID != "review-2026-v1" {
		t.Fatalf("explicit valid policy was rejected: %v", err)
	}
	if policy.TargetsSeconds["critical"] != 900 || policy.Timezone != "UTC" || policy.Clock != "elapsed_24x7" {
		t.Fatalf("explicit policy targets were modified: %#v", policy)
	}
	bad := []string{
		"null",
		"{}",
		validReviewPolicyJSON("2026-10-29T00:00:00+03:00"),
		strings.Replace(validReviewPolicyJSON("2026-10-29T00:00:00Z"), `"timezone":"UTC"`, `"timezone":"Europe/Istanbul"`, 1),
		strings.Replace(validReviewPolicyJSON("2026-10-29T00:00:00Z"), `"clock":"elapsed_24x7"`, `"clock":"business_hours"`, 1),
		strings.Replace(validReviewPolicyJSON("2026-10-29T00:00:00Z"), `"policy_id":"review-2026-v1"`, `"policy_id":"../admin"`, 1),
		strings.Replace(validReviewPolicyJSON("2026-10-29T00:00:00Z"), `"critical":900`, `"critical":0`, 1),
		strings.Replace(validReviewPolicyJSON("2026-10-29T00:00:00Z"), `"critical":900`, `"critical":100000000`, 1),
		strings.Replace(validReviewPolicyJSON("2026-10-29T00:00:00Z"), `"critical":900`, `"critical":7200`, 1),
		strings.Replace(validReviewPolicyJSON("2026-10-29T00:00:00Z"), `"medium":14400,`, "", 1),
		strings.Replace(validReviewPolicyJSON("2026-10-29T00:00:00Z"), `"info":86400`, `"info":86400,"urgent":200`, 1),
		strings.Replace(validReviewPolicyJSON("2026-10-29T00:00:00Z"), `"timezone":"UTC"`, `"timezone":"UTC","unexpected":true`, 1),
		validReviewPolicyJSON("2026-10-29T00:00:00Z") + "{}",
		strings.Repeat("x", ownerIncidentReviewPolicyMaxBytes+1),
	}
	for i, raw := range bad {
		policy, err := parseOwnerIncidentReviewPolicy(raw)
		if err == nil || policy != nil {
			t.Fatalf("invalid policy %d was accepted: %#v", i, policy)
		}
	}
}

func TestSOCReviewAttentionNeverFalsifiesSLAOrHistoricalCases(t *testing.T) {
	now := time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)
	policy, err := parseOwnerIncidentReviewPolicy(validReviewPolicyJSON("2026-10-29T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, severity, status string
		created                time.Time
		wantState              string
	}{
		{"within", "critical", "open", now.Add(-10 * time.Minute), "within_review_window_not_sla_guarantee"},
		{"over", "critical", "investigating", now.Add(-20 * time.Minute), "review_window_elapsed_not_sla_breach"},
		{"boundary", "critical", "contained", now.Add(-15 * time.Minute), "within_review_window_not_sla_guarantee"},
		{"historical", "critical", "open", now.Add(-24 * time.Hour), "case_precedes_policy_not_retroactive"},
		{"closed", "high", "closed", now.Add(-2 * time.Hour), "inactive_not_measured"},
		{"resolved", "high", "resolved", now.Add(-2 * time.Hour), "inactive_not_measured"},
		{"zero_timestamp", "high", "open", time.Time{}, "invalid_creation_time_no_review_claim"},
		{"future_timestamp", "high", "open", now.Add(time.Hour), "invalid_creation_time_no_review_claim"},
		{"unknown_severity", "urgent", "open", now.Add(time.Minute * -15), "severity_not_in_policy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := []ownerIncidentRecord{{ID: tc.name, Status: tc.status, Severity: tc.severity, CreatedAt: tc.created, EvidenceRefs: []string{"ev-1"}}}
			items, _, _ := buildOwnerIncidentTriage(input, now)
			applyOwnerIncidentReviewPolicy(items, now, policy)
			if items[0].ReviewAttention == nil || items[0].ReviewAttention.State != tc.wantState {
				t.Fatalf("wrong attention state: %#v", items[0].ReviewAttention)
			}
			if len(items[0].Incident.EvidenceRefs) != 1 || items[0].Incident.EvidenceRefs[0] != "ev-1" {
				t.Fatal("attention projection changed evidence")
			}
			a := items[0].ReviewAttention
			if tc.name == "over" {
				if a.OverSeconds == nil || *a.OverSeconds != 300 || a.DeadlineAt == nil {
					t.Fatalf("over-target seconds not deterministic: %#v", a)
				}
			}
			if tc.name == "historical" || tc.name == "closed" || tc.name == "zero_timestamp" {
				if a.DeadlineAt != nil || a.TargetSeconds != nil {
					t.Fatalf("historical/inactive/unknown case has fabricated deadline: %#v", a)
				}
			}
		})
	}
}

func TestSOCReviewAttentionFutureEffectivePolicyNotRetroactive(t *testing.T) {
	now := time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)
	policy, err := parseOwnerIncidentReviewPolicy(validReviewPolicyJSON("2026-11-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	item, _, _ := buildOwnerIncidentTriage([]ownerIncidentRecord{{Status: "open", Severity: "critical", CreatedAt: now.Add(-time.Minute)}}, now)
	a := ownerIncidentReviewAttentionForCase(item[0], now, policy)
	if a.State != "policy_not_yet_effective" || a.DeadlineAt != nil {
		t.Fatalf("future policy activated early: %#v", a)
	}
}

func TestSOCReviewAttentionMissingPolicyDoesNotCreateCaseTargets(t *testing.T) {
	now := time.Now().UTC()
	items, _, _ := buildOwnerIncidentTriage([]ownerIncidentRecord{{Status: "open", Severity: "critical", CreatedAt: now.Add(-time.Hour)}}, now)
	applyOwnerIncidentReviewPolicy(items, now, nil)
	if items[0].ReviewAttention != nil {
		t.Fatalf("absent policy invented a review window: %#v", items[0].ReviewAttention)
	}
}
