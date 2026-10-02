package services

import (
	"strings"
	"testing"
	"time"
)

func TestGlobalRadarSnapshotRejectsDivergentDuplicateEvidenceID(t *testing.T) {
	base := time.Date(2026, 10, 2, 6, 0, 0, 0, time.UTC)
	leftSubject := testGlobalCampaignRadarSubject("subject:left", IntelligenceChainFamilySolana, "solana", "solana")
	rightSubject := testGlobalCampaignRadarSubject("subject:right", IntelligenceChainFamilySolana, "solana", "solana")

	leftEvidence := testGlobalCampaignRadarEvidence("evidence:shared", leftSubject, IntelligenceEvidenceVerified, "tx-left", base, nil)
	rightEvidence := testGlobalCampaignRadarEvidence("evidence:shared", rightSubject, IntelligenceEvidenceVerified, "tx-right", base.Add(time.Second), nil)

	leftObservation, err := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, leftSubject, leftEvidence)
	if err != nil {
		t.Fatal(err)
	}
	rightObservation, err := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, rightSubject, rightEvidence)
	if err != nil {
		t.Fatal(err)
	}

	_, err = BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{leftObservation, rightObservation},
		nil,
		nil,
		nil,
		base.Add(time.Minute),
	)
	if err == nil || !strings.Contains(err.Error(), "divergent content") {
		t.Fatalf("expected divergent evidence id rejection, got %v", err)
	}
}

func TestGlobalRadarSnapshotAllowsExactDuplicateEvidenceIdentity(t *testing.T) {
	base := time.Date(2026, 10, 2, 6, 0, 0, 0, time.UTC)
	subject := testGlobalCampaignRadarSubject("subject:one", IntelligenceChainFamilySolana, "solana", "solana")
	evidence := testGlobalCampaignRadarEvidence("evidence:shared", subject, IntelligenceEvidenceVerified, "tx-one", base, nil)

	transactionObservation, err := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, subject, evidence)
	if err != nil {
		t.Fatal(err)
	}
	stateObservation, err := BuildGlobalRadarObservation(GlobalRadarObservationState, subject, evidence)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{transactionObservation, stateObservation},
		nil,
		nil,
		nil,
		base.Add(time.Minute),
	); err != nil {
		t.Fatalf("exact duplicate evidence identity should be idempotent: %v", err)
	}
}
