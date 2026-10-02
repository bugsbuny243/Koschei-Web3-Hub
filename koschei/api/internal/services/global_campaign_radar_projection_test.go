package services

import (
	"errors"
	"testing"
	"time"
)

func TestGlobalCampaignRadarProjectionVerifiedBridgeContinuity(t *testing.T) {
	base := time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC)
	source, destination, link := testGlobalCampaignRadarBridge(t, base, IntelligenceEvidenceVerified, IntelligenceEvidenceVerified)
	snapshot, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{source, destination},
		[]GlobalRadarRelationEdge{link.Relation},
		[]GlobalRadarBridgeLink{link},
		nil,
		base.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := BuildGlobalCampaignRadarProjection("KCAM1-BRIDGE", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Events) != 2 {
		t.Fatalf("events=%d", len(projection.Events))
	}
	if len(projection.CrossNetworkContinuity) != 1 {
		t.Fatalf("continuity=%d", len(projection.CrossNetworkContinuity))
	}
	continuity := projection.CrossNetworkContinuity[0]
	if continuity.BridgeLinkRef != link.LinkID || continuity.EvidenceState != IntelligenceEvidenceVerified {
		t.Fatalf("unexpected continuity: %#v", continuity)
	}
	if !continuity.TemporalAvailable || continuity.TemporalEvidenceState != IntelligenceEvidenceVerified {
		t.Fatalf("bridge continuity lost temporal verification: %#v", continuity)
	}
	if projection.Temporal.VerifiedLinkCount != 1 {
		t.Fatalf("verified temporal links=%d", projection.Temporal.VerifiedLinkCount)
	}
	if projection.VerdictAuthority || projection.GradeAuthority || projection.ContainmentAuthority ||
		projection.SameOperatorClaim || projection.RealWorldIdentityClaim || projection.WrongdoingClaim {
		t.Fatalf("projection crossed authority boundary: %#v", projection)
	}
}

func TestGlobalCampaignRadarProjectionVerifiedBridgeWithObservedEndpointStaysTemporalWatchOnly(t *testing.T) {
	base := time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC)
	source, destination, link := testGlobalCampaignRadarBridge(t, base, IntelligenceEvidenceObserved, IntelligenceEvidenceVerified)
	snapshot, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{source, destination},
		[]GlobalRadarRelationEdge{link.Relation},
		[]GlobalRadarBridgeLink{link},
		nil,
		base.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := BuildGlobalCampaignRadarProjection("KCAM1-BRIDGE", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	continuity := projection.CrossNetworkContinuity[0]
	if continuity.EvidenceState != IntelligenceEvidenceVerified {
		t.Fatalf("canonical bridge link should remain verified: %#v", continuity)
	}
	if !continuity.TemporalAvailable || continuity.TemporalEvidenceState == IntelligenceEvidenceVerified {
		t.Fatalf("observed endpoint should keep C6 temporal result non-verified: %#v", continuity)
	}
	if projection.Temporal.VerifiedLinkCount != 0 {
		t.Fatalf("observed endpoint over-promoted temporal verification: %#v", projection.Temporal)
	}
}

func TestGlobalCampaignRadarProjectionSameNetworkVerifiedRelationFeedsTemporalProof(t *testing.T) {
	base := time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC)
	sourceSubject := testGlobalCampaignRadarSubject("subject:sol:a", IntelligenceChainFamilySolana, "solana", "solana")
	targetSubject := testGlobalCampaignRadarSubject("subject:sol:b", IntelligenceChainFamilySolana, "solana", "solana")
	sourceEvidence := testGlobalCampaignRadarEvidence(
		"evidence:relation", sourceSubject, IntelligenceEvidenceVerified, "soltx-a", base,
		map[string]any{"source_subject_id": sourceSubject.ID, "target_subject_id": targetSubject.ID},
	)
	targetEvidence := testGlobalCampaignRadarEvidence("evidence:target", targetSubject, IntelligenceEvidenceVerified, "soltx-b", base.Add(20*time.Second), nil)
	sourceObservation, err := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, sourceSubject, sourceEvidence)
	if err != nil {
		t.Fatal(err)
	}
	targetObservation, err := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, targetSubject, targetEvidence)
	if err != nil {
		t.Fatal(err)
	}
	relation, err := BuildGlobalRadarRelationEdge(sourceSubject, targetSubject, "funded_by", sourceEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if relation.Status != IntelligenceEvidenceVerified {
		t.Fatalf("relation not verified: %#v", relation)
	}
	snapshot, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{sourceObservation, targetObservation},
		[]GlobalRadarRelationEdge{relation},
		nil,
		nil,
		base.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := BuildGlobalCampaignRadarProjection("KCAM1-RELATION", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Temporal.VerifiedLinkCount != 1 || len(projection.Temporal.Correlations) != 1 {
		t.Fatalf("verified relation did not feed temporal proof: %#v", projection.Temporal)
	}
	if projection.Temporal.Correlations[0].EvidenceState != IntelligenceEvidenceVerified {
		t.Fatalf("unexpected temporal relation state: %#v", projection.Temporal.Correlations[0])
	}
}

func TestGlobalCampaignRadarProjectionUnverifiedRelationRemainsContextOnly(t *testing.T) {
	base := time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC)
	sourceSubject := testGlobalCampaignRadarSubject("subject:sol:a", IntelligenceChainFamilySolana, "solana", "solana")
	targetSubject := testGlobalCampaignRadarSubject("subject:sol:b", IntelligenceChainFamilySolana, "solana", "solana")
	sourceEvidence := testGlobalCampaignRadarEvidence("evidence:a", sourceSubject, IntelligenceEvidenceVerified, "soltx-a", base, nil)
	targetEvidence := testGlobalCampaignRadarEvidence("evidence:b", targetSubject, IntelligenceEvidenceVerified, "soltx-b", base.Add(20*time.Second), nil)
	sourceObservation, _ := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, sourceSubject, sourceEvidence)
	targetObservation, _ := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, targetSubject, targetEvidence)
	relation, err := BuildGlobalRadarRelationEdge(sourceSubject, targetSubject, "funded_by", IntelligenceEvidence{})
	if err != nil {
		t.Fatal(err)
	}
	if relation.Status != IntelligenceEvidenceUnverified {
		t.Fatalf("relation should be unverified: %#v", relation)
	}
	snapshot, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{sourceObservation, targetObservation},
		[]GlobalRadarRelationEdge{relation},
		nil,
		nil,
		base.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := BuildGlobalCampaignRadarProjection("KCAM1-RELATION", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Temporal.VerifiedLinkCount != 0 {
		t.Fatalf("unverified relation was promoted: %#v", projection.Temporal)
	}
	if len(projection.Temporal.Correlations) != 1 || projection.Temporal.Correlations[0].EvidenceState == IntelligenceEvidenceVerified {
		t.Fatalf("unverified relation should remain temporal context: %#v", projection.Temporal)
	}
}

func TestGlobalCampaignRadarProjectionRejectsForgedBridgeIdentity(t *testing.T) {
	base := time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC)
	source, destination, link := testGlobalCampaignRadarBridge(t, base, IntelligenceEvidenceVerified, IntelligenceEvidenceVerified)
	snapshot, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{source, destination},
		[]GlobalRadarRelationEdge{link.Relation},
		[]GlobalRadarBridgeLink{link},
		nil,
		base.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.BridgeLinks[0].LinkID = "forged-link-id"

	_, err = BuildGlobalCampaignRadarProjection("KCAM1-FORGED", snapshot)
	if !errors.Is(err, ErrGlobalCampaignRadarEvidenceInvalid) {
		t.Fatalf("expected canonical bridge rejection, got %v", err)
	}
}

func TestGlobalCampaignRadarProjectionReplayFingerprintIgnoresSnapshotAssemblyTimeAndOrder(t *testing.T) {
	base := time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC)
	source, destination, link := testGlobalCampaignRadarBridge(t, base, IntelligenceEvidenceVerified, IntelligenceEvidenceVerified)
	firstSnapshot, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{source, destination},
		[]GlobalRadarRelationEdge{link.Relation},
		[]GlobalRadarBridgeLink{link},
		nil,
		base.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondSnapshot, err := BuildGlobalRadarSnapshot(
		[]GlobalRadarObservation{destination, source},
		[]GlobalRadarRelationEdge{link.Relation},
		[]GlobalRadarBridgeLink{link},
		nil,
		base.Add(2*time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	first, err := BuildGlobalCampaignRadarProjection("KCAM1-REPLAY", firstSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildGlobalCampaignRadarProjection("KCAM1-REPLAY", secondSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if first.FingerprintSHA256 != second.FingerprintSHA256 {
		t.Fatalf("snapshot assembly metadata changed projection fingerprint: %s != %s", first.FingerprintSHA256, second.FingerprintSHA256)
	}
	if first.Temporal.FingerprintSHA256 != second.Temporal.FingerprintSHA256 {
		t.Fatalf("snapshot order changed temporal fingerprint: %s != %s", first.Temporal.FingerprintSHA256, second.Temporal.FingerprintSHA256)
	}
}

func TestGlobalCampaignRadarProjectionRequiresCampaignRefAndGeneratedAt(t *testing.T) {
	_, err := BuildGlobalCampaignRadarProjection("", GlobalRadarSnapshot{})
	if !errors.Is(err, ErrGlobalCampaignRadarCampaignRefRequired) {
		t.Fatalf("expected campaign ref error, got %v", err)
	}
	_, err = BuildGlobalCampaignRadarProjection("KCAM1-A", GlobalRadarSnapshot{SchemaVersion: GlobalRadarSnapshotSchemaVersion})
	if !errors.Is(err, ErrGlobalCampaignRadarSnapshotInvalid) {
		t.Fatalf("expected snapshot generated_at error, got %v", err)
	}
}

func testGlobalCampaignRadarBridge(t *testing.T, base time.Time, sourceStatus, destinationStatus string) (GlobalRadarObservation, GlobalRadarObservation, GlobalRadarBridgeLink) {
	t.Helper()
	sourceSubject := testGlobalCampaignRadarSubject("subject:sol", IntelligenceChainFamilySolana, "solana", "solana")
	destinationSubject := testGlobalCampaignRadarSubject("subject:eth", IntelligenceChainFamilyEVM, "ethereum", "ethereum")
	attributes := map[string]any{"bridge_protocol": "wormhole", "bridge_transfer_id": "transfer-001"}
	sourceEvidence := testGlobalCampaignRadarEvidence("evidence:sol", sourceSubject, sourceStatus, "soltx-001", base, attributes)
	destinationEvidence := testGlobalCampaignRadarEvidence("evidence:eth", destinationSubject, destinationStatus, "0xeth001", base.Add(30*time.Second), attributes)
	source, err := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, sourceSubject, sourceEvidence)
	if err != nil {
		t.Fatal(err)
	}
	destination, err := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, destinationSubject, destinationEvidence)
	if err != nil {
		t.Fatal(err)
	}
	link, err := BuildGlobalRadarBridgeLink(source, destination)
	if err != nil {
		t.Fatal(err)
	}
	return source, destination, link
}

func testGlobalCampaignRadarSubject(id, family, chain, network string) IntelligenceSubject {
	return IntelligenceSubject{
		ID:                  id,
		Raw:                 id,
		CanonicalRef:        family + ":" + chain + ":" + network + ":" + id,
		ChainFamily:         family,
		Chain:               chain,
		Network:             network,
		Kind:                IntelligenceSubjectAddress,
		ClassificationBasis: "test_fixture",
	}
}

func testGlobalCampaignRadarEvidence(id string, subject IntelligenceSubject, status, tx string, observedAt time.Time, attributes map[string]any) IntelligenceEvidence {
	return IntelligenceEvidence{
		ID:              id,
		SubjectID:       subject.ID,
		ChainFamily:     subject.ChainFamily,
		Chain:           subject.Chain,
		Network:         subject.Network,
		Source:          "test_global_campaign_radar_projection",
		Status:          status,
		TransactionHash: tx,
		ObservedAt:      observedAt,
		Address:         subject.Raw,
		Confidence:      1,
		Attributes:      attributes,
	}
}
