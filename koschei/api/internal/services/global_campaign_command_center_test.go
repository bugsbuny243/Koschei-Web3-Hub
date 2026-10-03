package services

import (
	"errors"
	"testing"
)

func TestGlobalCampaignCommandCenterMonitoringDoesNotClaimContainment(t *testing.T) {
	campaign, fabric, _, now := responseAuthorizationFixture(t)
	snapshot, err := BuildGlobalCampaignCommandCenterSnapshot(GlobalCampaignCommandCenterInput{
		Campaign:   campaign,
		Fabric:     fabric,
		ObservedAt: now,
	})
	if err != nil {
		t.Fatalf("build monitoring snapshot: %v", err)
	}
	if snapshot.ResponseState != GlobalCampaignCommandCenterMonitoring || snapshot.ContainmentVerified {
		t.Fatalf("monitoring must not claim containment: state=%s verified=%v", snapshot.ResponseState, snapshot.ContainmentVerified)
	}
}

func TestGlobalCampaignCommandCenterAuthorizationIsNotExecution(t *testing.T) {
	campaign, fabric, proposal, auth, _, now := effectProofFixture(t)
	snapshot, err := BuildGlobalCampaignCommandCenterSnapshot(GlobalCampaignCommandCenterInput{
		Campaign:      campaign,
		Fabric:        fabric,
		Proposal:      proposal,
		Authorization: auth,
		ObservedAt:    now,
	})
	if err != nil {
		t.Fatalf("build authorized snapshot: %v", err)
	}
	if snapshot.ResponseState != GlobalCampaignCommandCenterAuthorized || snapshot.ContainmentVerified || snapshot.EffectProofHashSHA256 != "" {
		t.Fatalf("authorization must not imply execution/containment: %#v", snapshot)
	}
}

func TestGlobalCampaignCommandCenterPartialEffectIsExecutedOnly(t *testing.T) {
	campaign, fabric, proposal, auth, preflight, now := effectProofFixture(t)
	input := validEffectProofInput(now)
	input.EffectVerified = false
	proof, err := NewGlobalCampaignResponseEffectProof(campaign, fabric, proposal, auth, preflight, input, now)
	if err != nil {
		t.Fatalf("build partial effect proof: %v", err)
	}
	snapshot, err := BuildGlobalCampaignCommandCenterSnapshot(GlobalCampaignCommandCenterInput{
		Campaign:      campaign,
		Fabric:        fabric,
		Proposal:      proposal,
		Authorization: auth,
		Preflight:     preflight,
		EffectProof:   proof,
		ObservedAt:    now,
	})
	if err != nil {
		t.Fatalf("build executed snapshot: %v", err)
	}
	if snapshot.ResponseState != GlobalCampaignCommandCenterExecuted || snapshot.ContainmentVerified {
		t.Fatalf("partial effect must remain executed/not verified: state=%s verified=%v", snapshot.ResponseState, snapshot.ContainmentVerified)
	}
}

func TestGlobalCampaignCommandCenterVerifiedEffectIsOnlyContainmentTruth(t *testing.T) {
	campaign, fabric, proposal, auth, preflight, now := effectProofFixture(t)
	proof, err := NewGlobalCampaignResponseEffectProof(campaign, fabric, proposal, auth, preflight, validEffectProofInput(now), now)
	if err != nil {
		t.Fatalf("build effect proof: %v", err)
	}
	links := []GlobalCampaignIncidentLink{{
		IncidentRef:        "INC-42",
		CampaignRef:        campaign.CampaignRef,
		CampaignRevision:   int64(campaign.Revision),
		EvidenceHashSHA256: campaign.EvidenceHashSHA256,
	}}
	snapshot, err := BuildGlobalCampaignCommandCenterSnapshot(GlobalCampaignCommandCenterInput{
		Campaign:      campaign,
		IncidentLinks: links,
		Fabric:        fabric,
		Proposal:      proposal,
		Authorization: auth,
		Preflight:     preflight,
		EffectProof:   proof,
		ObservedAt:    now,
	})
	if err != nil {
		t.Fatalf("build verified snapshot: %v", err)
	}
	if snapshot.ResponseState != GlobalCampaignCommandCenterContainmentVerified || !snapshot.ContainmentVerified || len(snapshot.Incidents) != 1 {
		t.Fatalf("expected verified containment truth, got state=%s verified=%v incidents=%d", snapshot.ResponseState, snapshot.ContainmentVerified, len(snapshot.Incidents))
	}
}

func TestGlobalCampaignCommandCenterRejectsCrossRevisionBinding(t *testing.T) {
	campaign, fabric, proposal, auth, _, now := effectProofFixture(t)
	campaign.Revision++
	if _, err := BuildGlobalCampaignCommandCenterSnapshot(GlobalCampaignCommandCenterInput{
		Campaign:      campaign,
		Fabric:        fabric,
		Proposal:      proposal,
		Authorization: auth,
		ObservedAt:    now,
	}); !errors.Is(err, ErrGlobalCampaignCommandCenterInvalid) && !errors.Is(err, ErrGlobalCampaignCommandCenterMismatch) {
		t.Fatalf("expected cross-revision rejection, got %v", err)
	}
}

func TestGlobalCampaignCommandCenterRejectsSnapshotTamper(t *testing.T) {
	campaign, fabric, _, now := responseAuthorizationFixture(t)
	snapshot, err := BuildGlobalCampaignCommandCenterSnapshot(GlobalCampaignCommandCenterInput{
		Campaign:   campaign,
		Fabric:     fabric,
		ObservedAt: now,
	})
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	snapshot.CampaignState = GlobalCampaignClosed
	if err := ValidateGlobalCampaignCommandCenterSnapshot(snapshot); !errors.Is(err, ErrGlobalCampaignCommandCenterInvalid) {
		t.Fatalf("expected tamper rejection, got %v", err)
	}
}
