package services

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MarshalJSON keeps the public signed-verdict contract aligned with
// ACTOR_INVESTIGATION_ENGINE.md: deterministic letter grade, evidence, rule
// version and decision path; never a numeric risk score. A dash grade is still
// a signed deterministic result: no grade-changing rule fired, and that must
// not be confused with an A grade.
func (v UnifiedRadarVerdict) MarshalJSON() ([]byte, error) {
	evidence := unifiedVerdictEvidence(v)
	signed := v.Signed
	if v.Grade == "-" {
		signed = true
	}
	return json.Marshal(struct {
		Grade           string                `json:"grade"`
		Verdict         string                `json:"verdict"`
		Evidence        []string              `json:"evidence"`
		RuleVersion     string                `json:"rule_version"`
		ActorRuleVersion string               `json:"actor_ruleset_version,omitempty"`
		TriggeredRules  []ActorDefenseRuleHit `json:"triggered_rules"`
		WatchFlags      []ActorDefenseRuleHit `json:"watch_flags,omitempty"`
		DecisionPath    []string              `json:"decision_path"`
		NarrativeSource string                `json:"narrative_source,omitempty"`
		Signed          bool                  `json:"signed"`
		Signature       string                `json:"signature,omitempty"`
		CreatedAt       any                   `json:"created_at"`
	}{
		Grade: v.Grade,
		Verdict: v.Verdict,
		Evidence: evidence,
		RuleVersion: v.RulesetVersion,
		ActorRuleVersion: v.ActorRuleset,
		TriggeredRules: nonNilRuleHits(v.TriggeredRules),
		WatchFlags: nonNilRuleHits(v.WatchFlags),
		DecisionPath: nonNilDecisionPath(v.DecisionPath),
		NarrativeSource: v.NarrativeSource,
		Signed: signed,
		Signature: v.Signature,
		CreatedAt: v.GeneratedAt,
	})
}

func unifiedVerdictEvidence(v UnifiedRadarVerdict) []string {
	out := []string{}
	for _, hit := range v.TriggeredRules {
		line := fmt.Sprintf("%s [%s, %s]", strings.TrimSpace(hit.Title), strings.TrimSpace(hit.RuleID), strings.ToUpper(strings.TrimSpace(hit.EvidenceStatus)))
		if summary := strings.TrimSpace(hit.Summary); summary != "" {
			line += ": " + summary
		}
		out = appendUniqueContractEvidence(out, line)
	}
	for _, hit := range v.WatchFlags {
		line := fmt.Sprintf("WATCH: %s [%s, %s]", strings.TrimSpace(hit.Title), strings.TrimSpace(hit.RuleID), strings.ToUpper(strings.TrimSpace(hit.EvidenceStatus)))
		if summary := strings.TrimSpace(hit.Summary); summary != "" {
			line += ": " + summary
		}
		out = appendUniqueContractEvidence(out, line)
	}
	for _, step := range v.DecisionPath {
		out = appendUniqueContractEvidence(out, strings.TrimSpace(step))
	}
	if len(out) == 0 {
		out = append(out, "No grade-changing rule was satisfied; absence of evidence is not an A grade.")
	}
	return out
}

func appendUniqueContractEvidence(dst []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return dst
	}
	for _, existing := range dst {
		if existing == value {
			return dst
		}
	}
	return append(dst, value)
}

func nonNilRuleHits(in []ActorDefenseRuleHit) []ActorDefenseRuleHit {
	if in == nil {
		return []ActorDefenseRuleHit{}
	}
	return in
}

func nonNilDecisionPath(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
