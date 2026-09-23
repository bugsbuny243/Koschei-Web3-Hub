package services

import (
	"testing"
	"time"
)

func testGlobalRadarSubject() IntelligenceSubject {
	return IntelligenceSubject{
		ID:                  "kis_subject",
		Raw:                 "0x1111111111111111111111111111111111111111",
		CanonicalRef:        "evm:ethereum:ethereum-mainnet:0x1111111111111111111111111111111111111111",
		ChainFamily:         IntelligenceChainFamilyEVM,
		Chain:               "ethereum",
		Network:             "ethereum-mainnet",
		Kind:                IntelligenceSubjectAddress,
		ClassificationBasis: "evm_address_syntax",
	}
}

func testGlobalRadarEvidence(subject IntelligenceSubject) IntelligenceEvidence {
	return IntelligenceEvidence{
		ID:          "kis_evidence",
		SubjectID:   subject.ID,
		ChainFamily: subject.ChainFamily,
		Chain:       subject.Chain,
		Network:     subject.Network,
		Source:      "ethereum_jsonrpc",
		Status:      IntelligenceEvidenceObserved,
		ObservedAt:  time.Date(2026, 9, 23, 16, 30, 0, 0, time.UTC),
		Method:      "eth_getCode",
		Provenance:  "networktarget_read_only_probe",
		Confidence:  0.8,
	}
}

func TestBuildGlobalRadarObservationPreservesEvidenceWithoutCreatingVerdict(t *testing.T) {
	subject := testGlobalRadarSubject()
	evidence := testGlobalRadarEvidence(subject)

	got, err := BuildGlobalRadarObservation(GlobalRadarObservationContractProgram, subject, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != GlobalRadarObservationSchemaVersion || got.ObservationID == "" {
		t.Fatalf("unexpected observation identity: %#v", got)
	}
	if got.ObservationKind != GlobalRadarObservationContractProgram {
		t.Fatalf("observation kind=%q", got.ObservationKind)
	}
	if got.Subject.ID != subject.ID || got.Evidence.ID != evidence.ID {
		t.Fatalf("observation changed canonical identity: %#v", got)
	}
	if got.DecisionState != "evidence_only_no_verdict_created" {
		t.Fatalf("observation fabricated verdict state: %q", got.DecisionState)
	}
}

func TestBuildGlobalRadarObservationRejectsIdentityMismatch(t *testing.T) {
	subject := testGlobalRadarSubject()
	evidence := testGlobalRadarEvidence(subject)

	cases := []IntelligenceEvidence{
		func() IntelligenceEvidence { v := evidence; v.SubjectID = "kis_other"; return v }(),
		func() IntelligenceEvidence { v := evidence; v.Network = "base-mainnet"; return v }(),
		func() IntelligenceEvidence { v := evidence; v.Chain = "base"; return v }(),
		func() IntelligenceEvidence { v := evidence; v.ChainFamily = IntelligenceChainFamilyUTXO; return v }(),
	}
	for i, candidate := range cases {
		if _, err := BuildGlobalRadarObservation(GlobalRadarObservationState, subject, candidate); err == nil {
			t.Fatalf("case %d accepted mismatched evidence: %#v", i, candidate)
		}
	}
}

func TestBuildGlobalRadarObservationRejectsWeakOrMissingEvidence(t *testing.T) {
	subject := testGlobalRadarSubject()
	evidence := testGlobalRadarEvidence(subject)

	for i, mutate := range []func(*IntelligenceEvidence){
		func(v *IntelligenceEvidence) { v.ID = "" },
		func(v *IntelligenceEvidence) { v.Source = "" },
		func(v *IntelligenceEvidence) { v.ObservedAt = time.Time{} },
		func(v *IntelligenceEvidence) { v.Status = IntelligenceEvidenceInferred },
		func(v *IntelligenceEvidence) { v.Status = IntelligenceEvidenceUnverified },
	} {
		candidate := evidence
		mutate(&candidate)
		if _, err := BuildGlobalRadarObservation(GlobalRadarObservationState, subject, candidate); err == nil {
			t.Fatalf("case %d accepted weak evidence: %#v", i, candidate)
		}
	}
}

func TestBuildGlobalRadarObservationRejectsUnsupportedKind(t *testing.T) {
	subject := testGlobalRadarSubject()
	evidence := testGlobalRadarEvidence(subject)
	if _, err := BuildGlobalRadarObservation("prediction", subject, evidence); err == nil {
		t.Fatal("unsupported observation kind was accepted")
	}
}
