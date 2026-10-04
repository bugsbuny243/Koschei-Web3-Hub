package services

import (
	"testing"
	"time"
)

func TestProjectSecurityRadarVerdictToGlobalCampaignPreservesAuthorityBoundary(t *testing.T) {
	item := globalCampaignRadarRuntimeVerdict{
		SecurityRadarVerdictRecord: SecurityRadarVerdictRecord{
			ID:               "11111111-1111-1111-1111-111111111111",
			ModuleID:         "final_verdict_engine",
			Target:           "9cRCn9rGT8V2imeM2BaKs13yhMEais3ruM3rPvTGpump",
			TargetType:       "mint",
			Network:          "solana-mainnet",
			Grade:            "D",
			RiskIndex:        82,
			RiskLevel:        "high",
			Verdict:          "review",
			RuleVersion:      "test-rules-v1",
			EvidenceVerified: true,
			PayloadHash:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Source:           "arvis_stream",
			CreatedAt:        time.Date(2026, 10, 4, 4, 0, 0, 0, time.UTC),
			Signals:          map[string]any{"verified_evidence": true},
		},
		EvidenceBacked: true,
	}

	observation, snapshot, material, err := projectSecurityRadarVerdictToGlobalCampaign(item)
	if err != nil {
		t.Fatal(err)
	}
	if observation.DecisionState != "evidence_only_no_verdict_created" {
		t.Fatalf("global radar observation gained decision authority: %q", observation.DecisionState)
	}
	if observation.Evidence.Status != IntelligenceEvidenceVerified {
		t.Fatalf("verified evidence status lost: %q", observation.Evidence.Status)
	}
	if len(snapshot.Observations) != 1 || snapshot.Coverage.RiskScoreProduced {
		t.Fatalf("unexpected radar snapshot boundary: %+v", snapshot.Coverage)
	}
	if len(material.ObservationRefs) != 1 || material.ObservationRefs[0] != observation.ObservationID {
		t.Fatalf("campaign is not anchored to canonical observation: %+v", material.ObservationRefs)
	}
	if len(material.VerdictRefs) != 1 || material.VerifiedAnchorCount != 1 {
		t.Fatalf("expected verdict context + verified anchor: %+v", material)
	}
	campaign := MaterializeGlobalCampaign(material)
	if campaign.CampaignRef == "" || campaign.VerdictAuthority || campaign.GradeAuthority || campaign.ContainmentAuthority || campaign.SameOperatorClaim || campaign.RealWorldIdentityClaim || campaign.WrongdoingClaim {
		t.Fatalf("campaign crossed non-authority boundary: %+v", campaign)
	}
	projection, err := BuildGlobalCampaignRadarProjection(campaign.CampaignRef, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if projection.FingerprintSHA256 == "" || projection.VerdictAuthority || projection.GradeAuthority || projection.ContainmentAuthority {
		t.Fatalf("unexpected campaign projection authority: %+v", projection)
	}
}

func TestProjectSecurityRadarVerdictDoesNotUpgradeObservedEvidence(t *testing.T) {
	item := globalCampaignRadarRuntimeVerdict{
		SecurityRadarVerdictRecord: SecurityRadarVerdictRecord{
			ID:          "22222222-2222-2222-2222-222222222222",
			ModuleID:    "final_verdict_engine",
			Target:      "6QPvGr1L7aXGybpGKvvG8LtFDV9dRzK6QbSpRNJJonYM",
			TargetType:  "mint",
			Network:     "solana-mainnet",
			RuleVersion: "test-rules-v1",
			Source:      "security_radar",
			CreatedAt:   time.Date(2026, 10, 4, 4, 5, 0, 0, time.UTC),
			Signals:     map[string]any{"real_onchain_evidence": true},
		},
		EvidenceBacked: true,
	}
	observation, _, material, err := projectSecurityRadarVerdictToGlobalCampaign(item)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Evidence.Status != IntelligenceEvidenceObserved {
		t.Fatalf("observed evidence was upgraded to %q", observation.Evidence.Status)
	}
	if material.VerifiedAnchorCount != 0 || material.ObservedAnchorCount != 1 {
		t.Fatalf("anchor accounting upgraded observed evidence: %+v", material)
	}
}

func TestProjectSecurityRadarVerdictRejectsUnknownSubject(t *testing.T) {
	item := globalCampaignRadarRuntimeVerdict{
		SecurityRadarVerdictRecord: SecurityRadarVerdictRecord{
			ID:        "33333333-3333-3333-3333-333333333333",
			Target:    "not-a-chain-address",
			Network:   "unknown-mainnet",
			CreatedAt: time.Date(2026, 10, 4, 4, 10, 0, 0, time.UTC),
		},
	}
	if _, _, _, err := projectSecurityRadarVerdictToGlobalCampaign(item); err == nil {
		t.Fatal("expected unsupported canonical subject error")
	}
}
