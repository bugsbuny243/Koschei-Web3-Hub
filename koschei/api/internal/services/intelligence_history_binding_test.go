package services

import (
	"reflect"
	"testing"
	"time"
)

func TestBindClickHouseHistoricalEvidenceAppendsEvidenceWithoutChangingLiveDecision(t *testing.T) {
	subject := ClassifyIntelligenceSubject("62tJyrfNfYJ2qZncdbwFYmeJmSFn66BhGfgj491ppump", "solana-mainnet")
	live := BuildIntelligenceInvestigation([]IntelligenceSubject{subject}, time.Date(2026, 9, 9, 3, 0, 0, 0, time.UTC))
	live.Decision = IntelligenceDecision{Status: IntelligenceEvidenceVerified, Action: "block", Summary: "fresh evidence decision", EvidenceRefs: []string{"fresh:1"}, Confidence: 1}
	live.Evidence = []IntelligenceEvidence{{
		ID: "fresh:1", SubjectID: subject.ID, ChainFamily: subject.ChainFamily, Chain: subject.Chain, Network: subject.Network,
		Source: "fresh_arvis", Status: IntelligenceEvidenceVerified, Provenance: "fresh_live_evidence", Confidence: 1,
	}}
	historical := historicalBindingFixture(subject)
	beforeDecision := live.Decision

	receipt, err := BindClickHouseHistoricalEvidence(&live, historical)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "bound" || receipt.BoundEvidence != 2 || receipt.ExactDuplicates != 0 {
		t.Fatalf("receipt=%+v", receipt)
	}
	if !reflect.DeepEqual(live.Decision, beforeDecision) {
		t.Fatalf("historical binding changed live decision: before=%#v after=%#v", beforeDecision, live.Decision)
	}
	if len(live.Evidence) != 3 {
		t.Fatalf("evidence=%#v", live.Evidence)
	}
	if len(live.Entities) != 0 || len(live.Relationships) != 0 || len(live.Behaviors) != 0 || len(live.Hypotheses) != 0 || len(live.AttackPaths) != 0 {
		t.Fatalf("binding invented higher-order intelligence: %#v", live)
	}
}

func TestBindClickHouseHistoricalEvidenceExactDuplicateIsIdempotent(t *testing.T) {
	subject := ClassifyIntelligenceSubject("62tJyrfNfYJ2qZncdbwFYmeJmSFn66BhGfgj491ppump", "solana-mainnet")
	historical := historicalBindingFixture(subject)
	live := BuildIntelligenceInvestigation([]IntelligenceSubject{subject}, time.Now().UTC())
	live.Evidence = []IntelligenceEvidence{cloneIntelligenceEvidence(historical.Evidence[0])}

	receipt, err := BindClickHouseHistoricalEvidence(&live, historical)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "bound" || receipt.BoundEvidence != 1 || receipt.ExactDuplicates != 1 {
		t.Fatalf("receipt=%+v", receipt)
	}
	if len(live.Evidence) != 2 {
		t.Fatalf("evidence=%#v", live.Evidence)
	}

	receipt, err = BindClickHouseHistoricalEvidence(&live, historical)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "no_new_evidence" || receipt.BoundEvidence != 0 || receipt.ExactDuplicates != 2 {
		t.Fatalf("second receipt=%+v", receipt)
	}
	if len(live.Evidence) != 2 {
		t.Fatalf("idempotent evidence=%#v", live.Evidence)
	}
}

func TestBindClickHouseHistoricalEvidenceRejectsSubjectMismatchAtomically(t *testing.T) {
	liveSubject := ClassifyIntelligenceSubject("62tJyrfNfYJ2qZncdbwFYmeJmSFn66BhGfgj491ppump", "solana-mainnet")
	historicalSubject := ClassifyIntelligenceSubject("7YWHMfk9JZe0LM0g1ZauHuiSxhI4Gx2G2hmZ1Ezppump", "solana-mainnet")
	live := BuildIntelligenceInvestigation([]IntelligenceSubject{liveSubject}, time.Now().UTC())
	live.Evidence = []IntelligenceEvidence{{ID: "fresh:1", SubjectID: liveSubject.ID, ChainFamily: liveSubject.ChainFamily, Chain: liveSubject.Chain, Network: liveSubject.Network, Source: "fresh", Status: IntelligenceEvidenceObserved}}
	before := cloneIntelligenceInvestigation(live)

	_, err := BindClickHouseHistoricalEvidence(&live, historicalBindingFixture(historicalSubject))
	if err == nil {
		t.Fatal("expected exact subject-boundary mismatch")
	}
	if !reflect.DeepEqual(live, before) {
		t.Fatalf("failed binding mutated live investigation: before=%#v after=%#v", before, live)
	}
}

func TestBindClickHouseHistoricalEvidenceRejectsHigherOrderClaimsAndDecisionAuthority(t *testing.T) {
	subject := ClassifyIntelligenceSubject("62tJyrfNfYJ2qZncdbwFYmeJmSFn66BhGfgj491ppump", "solana-mainnet")
	cases := []IntelligenceInvestigation{
		func() IntelligenceInvestigation {
			h := historicalBindingFixture(subject)
			h.Relationships = []IntelligenceRelationship{{SourceSubjectID: subject.ID, TargetSubjectID: subject.ID, Relation: "invented"}}
			return h
		}(),
		func() IntelligenceInvestigation {
			h := historicalBindingFixture(subject)
			h.Decision = IntelligenceDecision{Status: IntelligenceEvidenceVerified, Action: "allow", Summary: "historical authority"}
			return h
		}(),
	}
	for i, historical := range cases {
		live := BuildIntelligenceInvestigation([]IntelligenceSubject{subject}, time.Now().UTC())
		before := cloneIntelligenceInvestigation(live)
		if _, err := BindClickHouseHistoricalEvidence(&live, historical); err == nil {
			t.Fatalf("case %d unexpectedly accepted", i)
		}
		if !reflect.DeepEqual(live, before) {
			t.Fatalf("case %d mutated live investigation", i)
		}
	}
}

func TestBindClickHouseHistoricalEvidenceRejectsForgedSourceAndVerification(t *testing.T) {
	subject := ClassifyIntelligenceSubject("62tJyrfNfYJ2qZncdbwFYmeJmSFn66BhGfgj491ppump", "solana-mainnet")
	cases := []func(*IntelligenceInvestigation){
		func(h *IntelligenceInvestigation) { h.Evidence[0].Source = "untrusted_memory" },
		func(h *IntelligenceInvestigation) { h.Evidence[0].Status = IntelligenceEvidenceVerified },
		func(h *IntelligenceInvestigation) { h.Evidence[1].Attributes["memory_authority"] = "current_authority" },
		func(h *IntelligenceInvestigation) { h.Evidence[1].Attributes["signature_verification"] = "verified" },
	}
	for i, mutate := range cases {
		historical := historicalBindingFixture(subject)
		mutate(&historical)
		live := BuildIntelligenceInvestigation([]IntelligenceSubject{subject}, time.Now().UTC())
		if _, err := BindClickHouseHistoricalEvidence(&live, historical); err == nil {
			t.Fatalf("case %d unexpectedly accepted", i)
		}
	}
}

func TestBindClickHouseHistoricalEvidenceRejectsConflictingEvidenceIDAtomically(t *testing.T) {
	subject := ClassifyIntelligenceSubject("62tJyrfNfYJ2qZncdbwFYmeJmSFn66BhGfgj491ppump", "solana-mainnet")
	historical := historicalBindingFixture(subject)
	live := BuildIntelligenceInvestigation([]IntelligenceSubject{subject}, time.Now().UTC())
	conflict := cloneIntelligenceEvidence(historical.Evidence[0])
	conflict.Source = "fresh_conflicting_source"
	live.Evidence = []IntelligenceEvidence{conflict}
	before := cloneIntelligenceInvestigation(live)

	if _, err := BindClickHouseHistoricalEvidence(&live, historical); err == nil {
		t.Fatal("expected conflicting evidence id rejection")
	}
	if !reflect.DeepEqual(live, before) {
		t.Fatal("conflict failure mutated live investigation")
	}
}

func historicalBindingFixture(subject IntelligenceSubject) IntelligenceInvestigation {
	historical := BuildIntelligenceInvestigation([]IntelligenceSubject{subject}, time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC))
	historical.Evidence = []IntelligenceEvidence{
		{
			ID: "clickhouse:event:22222222-2222-2222-2222-222222222222", SubjectID: subject.ID,
			ChainFamily: subject.ChainFamily, Chain: subject.Chain, Network: subject.Network,
			Source: historicalClickHouseStreamSource, Status: IntelligenceEvidenceObserved,
			Provenance: "existing_arvis_clickhouse_stream_shadow", Confidence: 1,
			Attributes: map[string]any{"stream_payload_policy": "digests_only_not_dereferenced"},
		},
		{
			ID: "clickhouse:verdict:11111111-1111-1111-1111-111111111111", SubjectID: subject.ID,
			ChainFamily: subject.ChainFamily, Chain: subject.Chain, Network: subject.Network,
			Source: historicalClickHouseVerdictSource, Status: IntelligenceEvidenceObserved,
			Provenance: "existing_arvis_clickhouse_verdict_shadow", Confidence: 1,
			Attributes: map[string]any{
				"memory_authority":       "historical_context_only",
				"signature_verification": "not_performed_by_memory_projection",
			},
		},
	}
	return historical
}
