package services

import "testing"

func TestActorCoordinationIntelligenceNoEvidenceMakesNoClaim(t *testing.T) {
	report := BuildActorCoordinationIntelligence(
		ActorDefenseDossier{Wallet: "wallet-a", Network: "solana-mainnet"},
		PersistentFundingClusterReport{SubjectWallet: "wallet-a", Network: "solana-mainnet", Complete: true},
		ActorCampaignGenome{},
	)
	if report.Available {
		t.Fatal("empty evidence must not become an available coordination finding")
	}
	if report.SignalCount != 0 {
		t.Fatalf("expected zero signals, got %d", report.SignalCount)
	}
	if report.SameOperatorClaim || report.RealWorldIdentityClaim || report.CriminalGroupClaim || report.MarketManipulationClaim || report.VerdictAuthority {
		t.Fatal("coordination intelligence must not manufacture identity, wrongdoing, manipulation or verdict authority")
	}
}

func TestActorCoordinationIntelligenceComposesRetainedEvidence(t *testing.T) {
	dossier := ActorDefenseDossier{
		Wallet:  "wallet-a",
		Network: "solana-mainnet",
		Track: ActorDefenseTrack{
			State:                    "correlated",
			CreatedTokenCount:        3,
			DominantHolderTokenCount: 2,
			RelatedActorCount:        4,
			Dossier: map[string]any{
				"creator_reuse_evidence_status": "verified",
				"holder_reuse_evidence_status":  "observed",
				"related_actor_evidence_status": "observed",
			},
		},
	}
	funding := PersistentFundingClusterReport{
		SubjectWallet: "wallet-a",
		Network:       "solana-mainnet",
		Complete:      true,
		Sources: []PersistentFundingSourceHistory{
			{
				Wallet:                 "funder-1",
				EvidenceStatus:         "verified_supported",
				FundedActorCount:       3,
				CreatedTokenCount:      4,
				LiquidityRemovalActors: 1,
				Members: []PersistentFundingClusterMember{
					{Wallet: "wallet-a"},
					{Wallet: "wallet-b"},
					{Wallet: "wallet-c"},
				},
			},
		},
	}
	genome := ActorCampaignGenome{
		Complete:        true,
		GenomeID:        "KCG1-1234567890ABCDEF",
		DescriptorCount: 5,
	}
	report := BuildActorCoordinationIntelligence(dossier, funding, genome)
	if !report.Available || report.Status != "coordination_patterns_observed" {
		t.Fatalf("expected coordination patterns, got status=%q available=%v", report.Status, report.Available)
	}
	if report.SignalCount != 6 {
		t.Fatalf("expected six composed signals, got %d", report.SignalCount)
	}
	if report.VerifiedSignalCount < 3 {
		t.Fatalf("expected verified-backed signals, got %d", report.VerifiedSignalCount)
	}
	if report.SameOperatorClaim || report.RealWorldIdentityClaim || report.CriminalGroupClaim || report.MarketManipulationClaim || report.VerdictAuthority {
		t.Fatal("technical correlations must remain non-authoritative")
	}
	foundCluster := false
	for _, signal := range report.Signals {
		if signal.Kind != "shared_funding_cluster" {
			continue
		}
		foundCluster = true
		if len(signal.RelatedWallets) != 2 || signal.RelatedWallets[0] != "wallet-b" || signal.RelatedWallets[1] != "wallet-c" {
			t.Fatalf("unexpected related wallets: %#v", signal.RelatedWallets)
		}
	}
	if !foundCluster {
		t.Fatal("expected shared-funding cluster signal")
	}
}
