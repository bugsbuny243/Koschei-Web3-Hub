package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// FinalizeUnifiedRadarVerdictContract binds the deterministic verdict state to
// its target before persistence. A withheld grade ("-") is still a signed
// deterministic decision: it means no grade-changing rule fired, never A/LOW.
func FinalizeUnifiedRadarVerdictContract(target string, verdict UnifiedRadarVerdict) UnifiedRadarVerdict {
	if strings.TrimSpace(verdict.RulesetVersion) == "" {
		verdict.RulesetVersion = UnifiedRadarRulesetVersion
	}
	if verdict.GeneratedAt.IsZero() {
		verdict.GeneratedAt = time.Now().UTC()
	} else {
		verdict.GeneratedAt = verdict.GeneratedAt.UTC()
	}
	verdict.TriggeredRules = nonNilActorRuleHits(verdict.TriggeredRules)
	verdict.WatchFlags = nonNilActorRuleHits(verdict.WatchFlags)
	verdict.DecisionPath = nonNilStrings(verdict.DecisionPath)
	verdict.Signed = true
	if strings.TrimSpace(verdict.Signature) == "" {
		verdict.Signature = signUnifiedRadarVerdict(strings.TrimSpace(target), verdict)
	}
	return verdict
}

// MarshalJSON is the public signed-verdict adapter. Internal names remain
// available for compatibility, while the canonical OSS/SDK contract receives
// rule_version, evidence, triggered_rules and decision_path. Numeric risk fields
// are deliberately absent.
func (verdict UnifiedRadarVerdict) MarshalJSON() ([]byte, error) {
	contract := verdict
	contract.RulesetVersion = strings.TrimSpace(contract.RulesetVersion)
	if contract.RulesetVersion == "" {
		contract.RulesetVersion = UnifiedRadarRulesetVersion
	}
	if contract.GeneratedAt.IsZero() {
		contract.GeneratedAt = time.Now().UTC()
	} else {
		contract.GeneratedAt = contract.GeneratedAt.UTC()
	}
	contract.TriggeredRules = nonNilActorRuleHits(contract.TriggeredRules)
	contract.WatchFlags = nonNilActorRuleHits(contract.WatchFlags)
	contract.DecisionPath = nonNilStrings(contract.DecisionPath)
	evidence := unifiedVerdictContractEvidence(contract)

	// Some callers evaluate before persistence. The serialized API contract must
	// still be self-consistent. Persistence calls Finalize... with the target and
	// therefore stores the target-bound signature; this fallback signs the
	// deterministic contract state itself when a target-bound signature is not yet
	// attached to the value.
	signature := strings.TrimSpace(contract.Signature)
	if signature == "" {
		signature = signUnifiedVerdictContractState(contract, evidence)
	}

	payload := struct {
		Grade           string                `json:"grade"`
		Verdict         string                `json:"verdict"`
		Evidence        []string              `json:"evidence"`
		RuleVersion     string                `json:"rule_version"`
		RulesetVersion  string                `json:"ruleset_version,omitempty"`
		ActorRuleset    string                `json:"actor_ruleset_version,omitempty"`
		TriggeredRules  []ActorDefenseRuleHit `json:"triggered_rules"`
		WatchFlags      []ActorDefenseRuleHit `json:"watch_flags,omitempty"`
		DecisionPath    []string              `json:"decision_path"`
		NarrativeSource string                `json:"narrative_source,omitempty"`
		Signed          bool                  `json:"signed"`
		Signature       string                `json:"signature,omitempty"`
		CreatedAt       time.Time             `json:"created_at"`
		GeneratedAt     time.Time             `json:"generated_at,omitempty"`
	}{
		Grade: normalizeUnifiedContractGrade(contract.Grade),
		Verdict: strings.TrimSpace(contract.Verdict),
		Evidence: evidence,
		RuleVersion: contract.RulesetVersion,
		RulesetVersion: contract.RulesetVersion,
		ActorRuleset: strings.TrimSpace(contract.ActorRuleset),
		TriggeredRules: contract.TriggeredRules,
		WatchFlags: contract.WatchFlags,
		DecisionPath: contract.DecisionPath,
		NarrativeSource: strings.TrimSpace(contract.NarrativeSource),
		Signed: true,
		Signature: signature,
		CreatedAt: contract.GeneratedAt,
		GeneratedAt: contract.GeneratedAt,
	}
	return json.Marshal(payload)
}

func unifiedVerdictContractEvidence(verdict UnifiedRadarVerdict) []string {
	values := []string{}
	for _, hit := range verdict.TriggeredRules {
		status := strings.ToUpper(strings.TrimSpace(hit.EvidenceStatus))
		line := strings.TrimSpace(hit.Summary)
		if line == "" {
			line = "Deterministic rule triggered."
		}
		values = append(values, fmt.Sprintf("%s [%s, %s]: %s", firstNonEmptyUnifiedContract(hit.Title, "Rule"), strings.TrimSpace(hit.RuleID), status, line))
	}
	for _, hit := range verdict.WatchFlags {
		line := strings.TrimSpace(hit.Summary)
		if line == "" {
			line = "Watch-only inference observed."
		}
		values = append(values, fmt.Sprintf("WATCH %s [%s, INFERRED]: %s", firstNonEmptyUnifiedContract(hit.Title, "Rule"), strings.TrimSpace(hit.RuleID), line))
	}
	if len(values) == 0 {
		values = append(values, "No grade-changing rule was triggered; absence of evidence is not an A grade.")
	}
	return uniqueUnifiedContractStrings(values)
}

func signUnifiedVerdictContractState(verdict UnifiedRadarVerdict, evidence []string) string {
	rules := make([]string, 0, len(verdict.TriggeredRules))
	for _, hit := range verdict.TriggeredRules {
		rules = append(rules, strings.TrimSpace(hit.RuleID)+":"+strings.TrimSpace(hit.EvidenceStatus))
	}
	sort.Strings(rules)
	payload := struct {
		Grade       string   `json:"grade"`
		RuleVersion string   `json:"rule_version"`
		Rules       []string `json:"rules"`
		Evidence    []string `json:"evidence"`
		Decision    []string `json:"decision_path"`
	}{
		Grade: normalizeUnifiedContractGrade(verdict.Grade),
		RuleVersion: strings.TrimSpace(verdict.RulesetVersion),
		Rules: rules,
		Evidence: evidence,
		Decision: nonNilStrings(verdict.DecisionPath),
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return "koschei-unified-contract:" + hex.EncodeToString(sum[:])
}

func normalizeUnifiedContractGrade(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "A", "B", "C", "D", "E", "F", "-":
		return value
	default:
		return "-"
	}
}

func uniqueUnifiedContractStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func firstNonEmptyUnifiedContract(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
