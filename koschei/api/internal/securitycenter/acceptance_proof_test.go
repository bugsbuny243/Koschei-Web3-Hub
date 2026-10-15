package securitycenter

import (
	"strings"
	"testing"
)

func fixtureAcceptanceProof() AcceptanceProof {
	return AcceptanceProof{
		CheckID:          "WSC-16.4",
		Target:           "isolated-fixture-asset",
		NetworkOrScope:   "solana-mainnet",
		TargetVersion:    "commit:fixture",
		ObservationPoint: "slot:100",
		TimeWindow:       "fixture-run:1",
		CollectorID:      "transaction-preflight-v1",
		LiveRoute:        "/api/v1/transaction/preflight",
		RunID:            "ci-run:fixture",
		RawSourceRef:     "fixture/source.json",
		ArtifactSHA256:   strings.Repeat("a", 64),
		RuleVersion:      "fixture-rule-v1",
		ExpectedResult:   "unknown authority should withhold",
		ObservedResult:   "withhold, no submission",
		EvidenceState:    "sufficient",
		ExecutionState:   "completed",
		FindingState:     "verified",
		Limitations:      "isolated deterministic fixture, not a live production verdict",
		CustomerOutput:   "fixture/report.json",
		PositiveFixture:  "fixture/known-untrusted.json",
		NormalFixture:    "fixture/benign.json",
		MonitoringRef:    "fixture/runtime-health.json",
		EnforcementState: "shadow",
		Reviewer:         "fixture-reviewer",
	}
}

func TestAcceptanceProofNeverPromotesIncompleteEvidence(t *testing.T) {
	good := fixtureAcceptanceProof()
	if err := ValidateAcceptanceProof(good); err != nil {
		t.Fatalf("well-formed candidate should be eligible for human review: %v", err)
	}
	cases := []struct {
		name string
		mutate func(*AcceptanceProof)
	}{
		{"unknown_control", func(p *AcceptanceProof) { p.CheckID = "WSC-99.9" }},
		{"missing_slot", func(p *AcceptanceProof) { p.ObservationPoint = "" }},
		{"missing_source", func(p *AcceptanceProof) { p.RawSourceRef = "" }},
		{"missing_route", func(p *AcceptanceProof) { p.LiveRoute = "" }},
		{"missing_rule", func(p *AcceptanceProof) { p.RuleVersion = "" }},
		{"missing_fixture", func(p *AcceptanceProof) { p.NormalFixture = "" }},
		{"missing_monitoring", func(p *AcceptanceProof) { p.MonitoringRef = "" }},
		{"unreviewed", func(p *AcceptanceProof) { p.Reviewer = "" }},
		{"incomplete_run", func(p *AcceptanceProof) { p.ExecutionState = "partial" }},
		{"weak_evidence", func(p *AcceptanceProof) { p.EvidenceState = "unavailable" }},
		{"hash_not_sha256", func(p *AcceptanceProof) { p.ArtifactSHA256 = "not-a-hash" }},
		{"unknown_finding", func(p *AcceptanceProof) { p.FindingState = "suspected" }},
		{"no_receipt", func(p *AcceptanceProof) { p.EnforcementState = "enforced" }},
		{"failed_enforcement", func(p *AcceptanceProof) { p.EnforcementState = "failed" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			record := fixtureAcceptanceProof()
			test.mutate(&record)
			if err := ValidateAcceptanceProof(record); err == nil {
				t.Fatal("invalid evidence promoted to acceptance review")
			}
		})
	}
}

func TestAcceptanceProofNoFindingsNeedSearchBoundary(t *testing.T) {
	record := fixtureAcceptanceProof()
	record.FindingState = "none_observed"
	if err := ValidateAcceptanceProof(record); err == nil {
		t.Fatal("no observed finding cannot imply an unbounded absence")
	}
	record.NegativeScope = "only 10 observed token holders at a single slot"
	if err := ValidateAcceptanceProof(record); err != nil {
		t.Fatalf("bounded none_observed should be eligible for review: %v", err)
	}
	record.EnforcementState = "enforced"
	record.EnforcementProof = "fixture/block-receipt.json"
	if err := ValidateAcceptanceProof(record); err != nil {
		t.Fatalf("receipt-anchored enforcement should be eligible for review: %v", err)
	}
}
