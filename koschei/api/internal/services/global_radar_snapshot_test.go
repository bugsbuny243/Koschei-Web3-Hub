package services

import (
	"testing"
	"time"
)

func TestBuildGlobalRadarSnapshotCountsCoverageWithoutRiskScore(t *testing.T) {
	sourceSubject := testGlobalRadarSubject()
	destinationSubject := sourceSubject
	source := bridgeObservation(sourceSubject, "ev-source", "ethereum-mainnet", "ethereum", "0xsource", "wormhole", "vaa:2/abc/7")
	destination := bridgeObservation(destinationSubject, "ev-destination", "base-mainnet", "base", "0xdestination", "wormhole", "vaa:2/abc/7")
	link, err := BuildGlobalRadarBridgeLink(source, destination)
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{source, destination},
		[]GlobalRadarRelationEdge{link.Relation},
		[]GlobalRadarBridgeLink{link},
		nil,
		time.Date(2026, 9, 23, 18, 30, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != GlobalRadarSnapshotSchemaVersion {
		t.Fatalf("schema=%q", snapshot.SchemaVersion)
	}
	if snapshot.Coverage.NetworkCount != 2 ||
		snapshot.Coverage.SubjectCount != 2 ||
		snapshot.Coverage.ObservationCount != 2 ||
		snapshot.Coverage.RelationCount != 1 ||
		snapshot.Coverage.VerifiedRelationCount != 1 ||
		snapshot.Coverage.CrossNetworkRelationCount != 1 ||
		snapshot.Coverage.BridgeLinkCount != 1 {
		t.Fatalf("unexpected snapshot coverage: %#v", snapshot.Coverage)
	}
	if snapshot.Coverage.RiskScoreProduced {
		t.Fatal("coverage snapshot produced a risk score")
	}
}

func TestBuildGlobalRadarSnapshotRejectsVerifiedRelationWithUnknownEvidence(t *testing.T) {
	source := bridgeObservation(testGlobalRadarSubject(), "ev-source", "ethereum-mainnet", "ethereum", "0xsource", "wormhole", "vaa:2/abc/7")
	target := bridgeObservation(testGlobalRadarSubject(), "ev-target", "base-mainnet", "base", "0xtarget", "wormhole", "vaa:2/abc/7")
	relation := GlobalRadarRelationEdge{
		SchemaVersion:      GlobalRadarRelationSchemaVersion,
		EdgeID:             "edge-1",
		Source:             source.Subject,
		Target:             target.Subject,
		Relation:           "bridge_transfer",
		Status:             IntelligenceEvidenceVerified,
		CrossNetwork:       true,
		EvidenceRefs:       []string{"missing-evidence"},
		VerificationPolicy: "explicit_link_evidence_required",
	}
	if _, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{source, target},
		[]GlobalRadarRelationEdge{relation},
		nil,
		nil,
		time.Now().UTC(),
	); err == nil {
		t.Fatal("snapshot accepted verified relation with evidence outside snapshot")
	}
}

func TestBuildGlobalRadarSnapshotCountsMissingTelemetryEvidence(t *testing.T) {
	subject := testGlobalRadarSubject()
	evidence := testGlobalRadarEvidence(subject)
	evidence.Attributes = map[string]any{
		"missing_evidence": []string{"client_family", "asn", "country_code"},
	}
	observation, err := BuildGlobalRadarObservation(GlobalRadarObservationNode, subject, evidence)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := BuildGlobalRadarSnapshot([]GlobalRadarObservation{observation}, nil, nil, nil, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Coverage.MissingEvidenceItemCount != 3 {
		t.Fatalf("missing evidence count=%d", snapshot.Coverage.MissingEvidenceItemCount)
	}
}


func TestBuildGlobalRadarSnapshotBindsARVISVerdictReferenceToIncludedEvidence(t *testing.T) {
	subject := ClassifyIntelligenceSubject("So11111111111111111111111111111111111111112", "solana-mainnet")
	subject.Kind = IntelligenceSubjectToken
	evidence := testGlobalRadarEvidence(subject)
	evidence.ID = "arvis-evidence-1"
	evidence.ChainFamily = subject.ChainFamily
	evidence.Chain = subject.Chain
	evidence.Network = subject.Network
	evidence.SubjectID = subject.ID
	evidence.Source = "arvis-canonical-evidence"
	observation, err := BuildGlobalRadarObservation(GlobalRadarObservationState, subject, evidence)
	if err != nil {
		t.Fatal(err)
	}
	verdict, decision := arvisVerdictReferenceFixture(subject)
	decision.EvidenceRefs = []string{evidence.ID}
	reference, err := ProjectARVISSignedVerdictToGlobalRadar(subject, verdict, decision)
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{observation},
		nil,
		nil,
		[]GlobalRadarVerdictReference{reference},
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Coverage.VerdictReferenceCount != 1 || len(snapshot.VerdictRefs) != 1 {
		t.Fatalf("verdict reference missing from snapshot: %#v", snapshot)
	}

	bad := reference
	bad.EvidenceRefs = []string{"outside-evidence"}
	if _, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{observation},
		nil,
		nil,
		[]GlobalRadarVerdictReference{bad},
		time.Now().UTC(),
	); err == nil {
		t.Fatal("snapshot accepted verdict reference to evidence outside snapshot")
	}
}
