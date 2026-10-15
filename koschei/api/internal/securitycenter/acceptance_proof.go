package securitycenter

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var acceptanceSHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// AcceptanceProof is a candidate evidence packet for one WSC/G control.
// It is NOT a receipt from an enforcement engine and cannot set live verdicts.
type AcceptanceProof struct {
	CheckID          string `json:"check_id"`
	Target           string `json:"target"`
	NetworkOrScope   string `json:"network_or_scope"`
	TargetVersion    string `json:"target_version"`
	ObservationPoint string `json:"observation_point"`
	TimeWindow       string `json:"time_window"`
	CollectorID      string `json:"collector_id"`
	LiveRoute        string `json:"live_route"`
	RunID            string `json:"run_id"`
	RawSourceRef     string `json:"raw_source_ref"`
	ArtifactSHA256   string `json:"artifact_sha256"`
	RuleVersion      string `json:"rule_version"`
	ExpectedResult   string `json:"expected_result"`
	ObservedResult   string `json:"observed_result"`
	EvidenceState    string `json:"evidence_state"`
	ExecutionState   string `json:"execution_state"`
	FindingState     string `json:"finding_state"`
	NegativeScope    string `json:"negative_scope,omitempty"`
	Limitations      string `json:"limitations"`
	CustomerOutput   string `json:"customer_output"`
	PositiveFixture  string `json:"positive_fixture"`
	NormalFixture    string `json:"normal_fixture"`
	MonitoringRef    string `json:"monitoring_ref"`
	EnforcementState string `json:"enforcement_state"`
	EnforcementProof string `json:"enforcement_proof,omitempty"`
	Reviewer         string `json:"reviewer"`
}

// ValidateAcceptanceProof enforces G01–G08 metadata sufficiency for promotion
// to manual review. It does NOT verify that a URL, signature or on-chain
// observation is truthful; a reviewer must retrieve and reproduce the source.
// A well-formed packet is not an accepted/verified capability by itself.
func ValidateAcceptanceProof(p AcceptanceProof) error {
	found := false
	for _, requirement := range acceptanceRequirements {
		if p.CheckID == requirement.id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown acceptance control %q", p.CheckID)
	}
	fields := map[string]string{
		"target": p.Target,
		"network_or_scope": p.NetworkOrScope,
		"target_version": p.TargetVersion,
		"observation_point": p.ObservationPoint,
		"time_window": p.TimeWindow,
		"collector_id": p.CollectorID,
		"live_route": p.LiveRoute,
		"run_id": p.RunID,
		"raw_source_ref": p.RawSourceRef,
		"artifact_sha256": p.ArtifactSHA256,
		"rule_version": p.RuleVersion,
		"expected_result": p.ExpectedResult,
		"observed_result": p.ObservedResult,
		"limitations": p.Limitations,
		"customer_output": p.CustomerOutput,
		"positive_fixture": p.PositiveFixture,
		"normal_fixture": p.NormalFixture,
		"monitoring_ref": p.MonitoringRef,
		"reviewer": p.Reviewer,
	}
	for field, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required to review %s", field, p.CheckID)
		}
	}
	if !acceptanceSHA256Pattern.MatchString(p.ArtifactSHA256) {
		return errors.New("artifact_sha256 must be a lowercase 64-character hex digest")
	}
	if p.ExecutionState != "completed" || p.EvidenceState != "sufficient" {
		return errors.New("not-completed or insufficient-evidence work cannot be promoted")
	}
	if p.FindingState != "verified" && p.FindingState != "none_observed" {
		return errors.New("finding_state must be verified or none_observed for a completed acceptance case")
	}
	if p.FindingState == "none_observed" && strings.TrimSpace(p.NegativeScope) == "" {
		return errors.New("none_observed requires an explicit negative-scope boundary")
	}
	switch p.EnforcementState {
	case "not_connected", "shadow", "enforced":
		if p.EnforcementState == "enforced" && strings.TrimSpace(p.EnforcementProof) == "" {
			return errors.New("enforced control requires an actual enforcement receipt")
		}
	default:
		return errors.New("unknown or failed enforcement state cannot be promoted")
	}
	return nil
}
