package services

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// The fixture models a delegated spend with resource-scoped evidence. The
// evidence belongs to the treasury, not the delegate exercising the capability.
func unifiedBindingTestInvestigation() UnifiedSecurityInvestigation {
	subjects := []IntelligenceSubject{
		ClassifyIntelligenceSubject("0x1111111111111111111111111111111111111111", "ethereum-mainnet"),
		ClassifyIntelligenceSubject("0x2222222222222222222222222222222222222222", "ethereum-mainnet"),
		ClassifyIntelligenceSubject("0x3333333333333333333333333333333333333333", "ethereum-mainnet"),
		ClassifyIntelligenceSubject("0x4444444444444444444444444444444444444444", "ethereum-mainnet"),
	}
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	base := BuildIntelligenceInvestigation(subjects, now)
	base.Evidence = []IntelligenceEvidence{{
		ID:              "fixture:evidence",
		SubjectID:       subjects[2].ID,
		ChainFamily:     subjects[2].ChainFamily,
		Chain:           subjects[2].Chain,
		Network:         subjects[2].Network,
		Source:          "explicit_test_fixture",
		Provenance:      "synthetic_binding_test",
		Status:          IntelligenceEvidenceVerified,
		TransactionHash: "fixture:transaction",
		Confidence:      1,
	}}
	refs := []string{base.Evidence[0].ID}
	capability := BuildIntelligenceCapability(subjects[0].ID, UnifiedSecurityDomainBlockchain,
		IntelligenceCapabilitySpend, "bounded_spend", subjects[2].Raw, subjects[1].ID,
		IntelligenceCapabilityLifecycleRevoked, IntelligenceEvidenceVerified, nil, refs, 1)
	action := BuildIntelligenceAction(subjects[0].ID, capability.ID, subjects[1].ID,
		UnifiedSecurityDomainBlockchain, "spend_attempt", subjects[2].Raw,
		"fixture:transaction", "attempt_only", IntelligenceEvidenceVerified, refs, 1)
	boundary := BuildTrustBoundaryTransition(subjects[0].ID, capability.ID, action.ID,
		UnifiedSecurityDomainBlockchain, UnifiedSecurityDomainProtocol, "contract_call",
		IntelligenceEvidenceVerified, refs, 1)
	consequence := BuildIntelligenceConsequence(UnifiedSecurityDomainProtocol,
		"fixture_effect", subjects[2].Raw, "Fixture consequence", "fixture_only",
		IntelligenceEvidenceVerified, refs, 1)
	path := BuildUnifiedSecurityAttackPath("Fixture path", subjects[0].ID,
		IntelligenceEvidenceVerified, []string{"explicit_test_fixture"}, []UnifiedSecurityAttackPathStep{{
			SubjectID:            subjects[0].ID,
			TargetSubjectID:      subjects[1].ID,
			CapabilityID:         capability.ID,
			ActionID:             action.ID,
			BoundaryTransitionID: boundary.ID,
			Effect:               "Fixture effect",
			EvidenceRefs:         refs,
		}}, []IntelligenceConsequence{consequence}, refs, 1)
	in := BuildUnifiedSecurityInvestigation(base, now)
	in.Capabilities = []IntelligenceCapability{capability}
	in.Actions = []IntelligenceAction{action}
	in.BoundaryTransitions = []IntelligenceTrustBoundaryTransition{boundary}
	in.AttackPaths = []UnifiedSecurityAttackPath{path}
	return in
}

func TestUnifiedBindingsPreserveConsistentEvidenceAndRevokedLifecycle(t *testing.T) {
	in := unifiedBindingTestInvestigation()
	out := FinalizeUnifiedSecurityInvestigation(in)
	if out.BindingStatus != "consistent" || len(out.BindingIssues) != 0 {
		t.Fatalf("complete graph rejected: %#v", out.BindingIssues)
	}
	want := in
	want.BindingStatus = "consistent"
	if !reflect.DeepEqual(out, want) {
		t.Fatal("consistent projection changed evidence, lifecycle, or decision")
	}
	if in.BindingStatus != "unchecked" {
		t.Fatal("finalization mutated the input")
	}
}

func TestUnifiedBindingsRejectSubstitutionAndDanglingReferences(t *testing.T) {
	tests := []struct {
		name string
		code string
		edit func(*UnifiedSecurityInvestigation)
	}{
		{"capability evidence missing", "evidence_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.Capabilities[0].EvidenceRefs = []string{"absent"}
		}},
		{"action evidence missing", "evidence_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.Actions[0].EvidenceRefs = []string{"absent"}
		}},
		{"boundary evidence missing", "evidence_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.BoundaryTransitions[0].EvidenceRefs = []string{"absent"}
		}},
		{"path evidence missing", "evidence_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].EvidenceRefs = []string{"absent"}
		}},
		{"step evidence missing", "evidence_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].Steps[0].EvidenceRefs = []string{"absent"}
		}},
		{"consequence evidence missing", "evidence_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].Consequences[0].EvidenceRefs = []string{"absent"}
		}},
		{"unknown evidence subject", "evidence_subject_missing", func(in *UnifiedSecurityInvestigation) {
			in.Base.Evidence[0].SubjectID = "absent"
		}},
		{"unrelated evidence subject", "evidence_subject_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.Base.Evidence[0].SubjectID = in.Base.Subjects[3].ID
		}},
		{"network substitution", "evidence_network_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.Base.Evidence[0].Network = "ethereum-sepolia"
		}},
		{"same address on another network", "evidence_subject_mismatch", func(in *UnifiedSecurityInvestigation) {
			other := ClassifyIntelligenceSubject(in.Base.Subjects[2].Raw, "ethereum-sepolia")
			in.Base.Subjects = append(in.Base.Subjects, other)
			in.Base.Evidence[0].SubjectID = other.ID
			in.Base.Evidence[0].Network = other.Network
			in.Base.Evidence[0].Chain = other.Chain
		}},
		{"chain substitution", "evidence_network_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.Base.Evidence[0].ChainFamily = IntelligenceChainFamilySolana
		}},
		{"ambiguous evidence", "object_id_ambiguous", func(in *UnifiedSecurityInvestigation) {
			in.Base.Evidence = append(in.Base.Evidence, in.Base.Evidence[0])
		}},
		{"ambiguous subject", "object_id_ambiguous", func(in *UnifiedSecurityInvestigation) {
			in.Base.Subjects = append(in.Base.Subjects, in.Base.Subjects[0])
		}},
		{"ambiguous action", "object_id_ambiguous", func(in *UnifiedSecurityInvestigation) {
			in.Actions = append(in.Actions, in.Actions[0])
		}},
		{"ambiguous consequence", "object_id_ambiguous", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].Consequences = append(in.AttackPaths[0].Consequences, in.AttackPaths[0].Consequences[0])
		}},
		{"capability belongs to another actor", "capability_subject_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.Capabilities[0].SubjectID = in.Base.Subjects[3].ID
		}},
		{"missing delegated authority", "subject_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.Capabilities[0].DelegatedBySubjectID = "absent"
		}},
		{"capability resource substitution", "capability_resource_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.Actions[0].ResourceRef = in.Base.Subjects[3].Raw
		}},
		{"capability domain substitution", "capability_domain_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.Actions[0].Domain = UnifiedSecurityDomainAIAgent
		}},
		{"missing capability", "capability_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.Actions[0].CapabilityID = "absent"
		}},
		{"missing action", "action_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].Steps[0].ActionID = "absent"
		}},
		{"missing boundary", "boundary_reference_missing", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].Steps[0].BoundaryTransitionID = "absent"
		}},
		{"step target substitution", "path_action_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].Steps[0].TargetSubjectID = in.Base.Subjects[3].ID
		}},
		{"boundary action substitution", "boundary_action_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.BoundaryTransitions[0].SubjectID = in.Base.Subjects[3].ID
		}},
		{"boundary omitted by step", "path_boundary_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].Steps[0].CapabilityID = ""
		}},
		{"path entry substitution", "path_entry_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].EntrySubjectID = in.Base.Subjects[3].ID
		}},
		{"step order substitution", "path_step_order_invalid", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].Steps[0].Order = 2
		}},
		{"observed evidence cannot verify claim", "evidence_strength_insufficient", func(in *UnifiedSecurityInvestigation) {
			in.Base.Evidence[0].Status = IntelligenceEvidenceObserved
		}},
		{"weak capability cannot verify action", "capability_strength_insufficient", func(in *UnifiedSecurityInvestigation) {
			in.Capabilities[0].Status = IntelligenceEvidenceObserved
		}},
		{"weak action cannot verify path", "action_strength_insufficient", func(in *UnifiedSecurityInvestigation) {
			in.Actions[0].Status = IntelligenceEvidenceObserved
		}},
		{"weak boundary cannot verify path", "boundary_strength_insufficient", func(in *UnifiedSecurityInvestigation) {
			in.BoundaryTransitions[0].Status = IntelligenceEvidenceObserved
		}},
		{"inferred consequence cannot verify path", "consequence_strength_insufficient", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].Consequences[0].Status = IntelligenceEvidenceInferred
		}},
		{"missing provenance", "evidence_provenance_missing", func(in *UnifiedSecurityInvestigation) {
			in.Base.Evidence[0].Provenance = ""
		}},
		{"transaction substitution", "action_transaction_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.Base.Evidence[0].TransactionHash = "fixture:another-transaction"
		}},
		{"transaction binding absent", "action_transaction_evidence_missing", func(in *UnifiedSecurityInvestigation) {
			in.Base.Evidence[0].TransactionHash = ""
		}},
		{"unsupported contract", "contract_version_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.ContractVersion = "unsupported"
		}},
		{"unsupported path contract", "contract_version_mismatch", func(in *UnifiedSecurityInvestigation) {
			in.AttackPaths[0].ContractVersion = "unsupported"
		}},
		{"unknown evidence status", "evidence_status_invalid", func(in *UnifiedSecurityInvestigation) {
			in.Capabilities[0].Status = "safe"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := unifiedBindingTestInvestigation()
			test.edit(&in)
			before, err := json.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}
			out := FinalizeUnifiedSecurityInvestigation(in)
			found := false
			for _, issue := range out.BindingIssues {
				if issue.Code == test.code {
					found = true
				}
			}
			if out.BindingStatus != "unverified" || !found {
				t.Fatalf("substitution was not rejected with %q: %#v", test.code, out.BindingIssues)
			}
			assertUnifiedBindingClaimsUnverified(t, out)
			if !reflect.DeepEqual(in.Base, out.Base) {
				t.Fatal("binding validation changed the authoritative base")
			}
			after, err := json.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("binding validation mutated caller-owned evidence or claims")
			}
			again := FinalizeUnifiedSecurityInvestigation(out)
			assertUnifiedBindingClaimsUnverified(t, again)
		})
	}
}

func TestUnifiedBindingsNeverUpgradeObservedInferredOrUnavailableEvidence(t *testing.T) {
	for _, status := range []string{IntelligenceEvidenceObserved, IntelligenceEvidenceInferred, IntelligenceEvidenceUnverified} {
		t.Run(status, func(t *testing.T) {
			in := unifiedBindingTestInvestigation()
			in.Base.Evidence[0].Status = status
			in.Capabilities[0].Status = status
			in.Actions[0].Status = status
			in.BoundaryTransitions[0].Status = status
			in.AttackPaths[0].Status = status
			in.AttackPaths[0].Consequences[0].Status = status
			out := FinalizeUnifiedSecurityInvestigation(in)
			if out.BindingStatus != "consistent" || len(out.BindingIssues) != 0 {
				t.Fatalf("consistent lower-strength evidence rejected: %#v", out.BindingIssues)
			}
			in.BindingStatus = "consistent"
			if !reflect.DeepEqual(in, out) {
				t.Fatal("finalization upgraded a claim or changed the source decision")
			}
		})
	}
}

func TestUnifiedBindingsEmptyEvidenceDoesNotCreateClaims(t *testing.T) {
	in := BuildUnifiedSecurityInvestigation(BuildIntelligenceInvestigation(nil, time.Now()), time.Now())
	out := FinalizeUnifiedSecurityInvestigation(in)
	if out.BindingStatus != "consistent" || out.Base.Decision.Status != IntelligenceEvidenceUnverified {
		t.Fatalf("empty projection became a decision: %#v", out)
	}
	if len(out.Capabilities)+len(out.Actions)+len(out.BoundaryTransitions)+len(out.AttackPaths) != 0 {
		t.Fatal("empty projection created claims")
	}
}

func assertUnifiedBindingClaimsUnverified(t *testing.T, out UnifiedSecurityInvestigation) {
	t.Helper()
	check := func(status string, confidence float64) {
		t.Helper()
		if status != IntelligenceEvidenceUnverified || confidence != 0 {
			t.Fatalf("broken binding retained a positive claim: status=%q confidence=%v", status, confidence)
		}
	}
	for _, c := range out.Capabilities {
		check(c.Status, c.Confidence)
	}
	for _, a := range out.Actions {
		check(a.Status, a.Confidence)
	}
	for _, b := range out.BoundaryTransitions {
		check(b.Status, b.Confidence)
	}
	for _, p := range out.AttackPaths {
		check(p.Status, p.Confidence)
		for _, c := range p.Consequences {
			check(c.Status, c.Confidence)
		}
	}
}
