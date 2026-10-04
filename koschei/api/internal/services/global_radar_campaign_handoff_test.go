package services

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGlobalRadarHandoffCanonicalReceipt(t *testing.T) {
	now := time.Now().UTC()
	a, b, bridge := testGlobalCampaignRadarBridge(t, now, IntelligenceEvidenceVerified, IntelligenceEvidenceVerified)
	snapshot, err := BuildGlobalRadarSnapshot([]GlobalRadarObservation{a, b}, []GlobalRadarRelationEdge{bridge.Relation}, []GlobalRadarBridgeLink{bridge}, nil, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_, payload, source, err := canonicalGlobalRadarHandoff(snapshot)
	if err != nil || len(payload) == 0 || len(source) != 71 {
		t.Fatalf("receipt: %s %v", source, err)
	}
	_, _, again, err := canonicalGlobalRadarHandoff(snapshot)
	if err != nil || again != source {
		t.Fatalf("unstable receipt: %v", err)
	}
	snapshot.Coverage.VerifiedRelationCount++
	if _, _, _, err := canonicalGlobalRadarHandoff(snapshot); err == nil {
		t.Fatal("noncanonical coverage accepted")
	}
	if err := StageGlobalRadarCampaignSnapshot(context.Background(), nil, snapshot); !errors.Is(err, ErrGlobalCampaignStoreUnavailable) {
		t.Fatal(err)
	}
}
