package services

import (
	"errors"
	"testing"
	"time"
)

func campaignRuntimeFixture(t *testing.T) GlobalRadarSnapshot {
	t.Helper()
	base := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	a, b, bridge := testGlobalCampaignRadarBridge(t, base, IntelligenceEvidenceVerified, IntelligenceEvidenceVerified)
	snapshot, err := BuildGlobalRadarSnapshot([]GlobalRadarObservation{a, b}, []GlobalRadarRelationEdge{bridge.Relation}, []GlobalRadarBridgeLink{bridge}, nil, base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestGlobalCampaignRuntimeOnlyVerifiedLinkedEvidence(t *testing.T) {
	snapshot := campaignRuntimeFixture(t)
	jobs, err := buildGlobalCampaignRuntimeJobs(snapshot)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("verified graph: jobs=%d error=%v", len(jobs), err)
	}
	job := jobs[0]
	if len(job.Input.BridgeLinkRefs) != 1 || job.Radar.Temporal.VerifiedLinkCount != 1 {
		t.Fatal("bridge/temporal provenance lost")
	}
	c := MaterializeGlobalCampaign(job.Input)
	if c.State != GlobalCampaignEmerging || len(c.MissingEvidence) == 0 || job.Radar.WrongdoingClaim || job.Radar.ContainmentAuthority {
		t.Fatal("evidence group promoted to authority")
	}
	replay := snapshot
	replay.GeneratedAt = snapshot.GeneratedAt.Add(time.Hour)
	replay.Observations = []GlobalRadarObservation{snapshot.Observations[1], snapshot.Observations[0]}
	again, err := buildGlobalCampaignRuntimeJobs(replay)
	if err != nil || len(again) != 1 || globalCampaignRuntimeSourceRef(again[0]) != globalCampaignRuntimeSourceRef(job) {
		t.Fatalf("replay not deterministic: %v", err)
	}
	snapshot.BridgeLinks = nil
	snapshot.Relations = nil
	// Both independently verified observations are insufficient to group.
	unrelated, err := buildGlobalCampaignRuntimeJobs(snapshot)
	if err != nil || len(unrelated) != 0 {
		t.Fatalf("unrelated evidence became a campaign: %v %+v", err, unrelated)
	}
	edge, err := BuildGlobalRadarRelationEdge(snapshot.Observations[0].Subject, snapshot.Observations[1].Subject, "funded_by", IntelligenceEvidence{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Relations = []GlobalRadarRelationEdge{edge}
	unrelated, err = buildGlobalCampaignRuntimeJobs(snapshot)
	if err != nil || len(unrelated) != 0 {
		t.Fatalf("unverified edge became a campaign: %v", err)
	}
}

func TestGlobalCampaignRuntimeRejectsForgeryAndOversizedGraphs(t *testing.T) {
	snapshot := campaignRuntimeFixture(t)
	forged := campaignRuntimeFixture(t)
	forged.BridgeLinks[0].LinkID = "forged"
	if _, err := buildGlobalCampaignRuntimeJobs(forged); !errors.Is(err, ErrGlobalCampaignRadarEvidenceInvalid) {
		t.Fatalf("forged bridge accepted: %v", err)
	}
	snapshot.Observations = make([]GlobalRadarObservation, 129)
	if _, err := buildGlobalCampaignRuntimeJobs(snapshot); !errors.Is(err, ErrGlobalCampaignRuntimeInput) {
		t.Fatalf("unbounded graph accepted: %v", err)
	}
	jobs, err := buildGlobalCampaignRuntimeJobs(campaignRuntimeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	jobs[0].Radar.Events[0].Ref = "tampered"
	if _, err := validateGlobalCampaignRuntimeJob(jobs[0]); !errors.Is(err, ErrGlobalCampaignRuntimeInput) {
		t.Fatalf("tampered projection accepted: %v", err)
	}
}
