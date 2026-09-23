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
	snapshot, err := BuildGlobalRadarSnapshot([]GlobalRadarObservation{observation}, nil, nil, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Coverage.MissingEvidenceItemCount != 3 {
		t.Fatalf("missing evidence count=%d", snapshot.Coverage.MissingEvidenceItemCount)
	}
}
