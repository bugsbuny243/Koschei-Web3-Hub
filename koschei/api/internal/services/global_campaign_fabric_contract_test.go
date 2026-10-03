package services

import (
	"reflect"
	"testing"
	"time"
)

func TestFabricCampaignEvidenceContractDeterministicAndObserveOnly(t *testing.T) {
	base := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	campaign := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:         "KCAM1-FABRIC",
		Revision:            3,
		State:               GlobalCampaignActive,
		FirstObservedAt:     base,
		LastObservedAt:      base.Add(time.Hour),
		Networks:            []string{"solana", "ethereum"},
		Subjects:            []string{"subject:b", "subject:a"},
		ObservationRefs:     []string{"obs:2", "obs:1"},
		RelationRefs:        []string{"rel:1"},
		BridgeLinkRefs:      []string{"bridge:1"},
		VerdictRefs:         []string{"verdict:1"},
		AttackPathRefs:      []string{"attack:1"},
		MissingEvidence:     []string{"destination_owner"},
		ObservedAnchorCount: 2,
		VerifiedAnchorCount: 1,
		RulesetVersion:      "campaign-rules.v1",
	})
	links := []GlobalCampaignIncidentLink{
		{IncidentRef: "KSI-B", CampaignRef: campaign.CampaignRef, CampaignRevision: 3, EvidenceHashSHA256: campaign.EvidenceHashSHA256},
		{IncidentRef: "KSI-A", CampaignRef: campaign.CampaignRef, CampaignRevision: 2, EvidenceHashSHA256: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	first, err := BuildFabricCampaignEvidenceContract(campaign, GlobalCampaignTemporalCorrelationReport{}, GlobalCampaignRadarProjection{}, GlobalCampaignThreatFamilyReport{}, links)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildFabricCampaignEvidenceContract(campaign, GlobalCampaignTemporalCorrelationReport{}, GlobalCampaignRadarProjection{}, GlobalCampaignThreatFamilyReport{}, []GlobalCampaignIncidentLink{links[1], links[0]})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("input order changed Fabric contract:\n%#v\n%#v", first, second)
	}
	if first.Mode != FabricCampaignEvidenceMode || first.ContractHashSHA256 == "" {
		t.Fatalf("unexpected contract: %#v", first)
	}
	if first.VerdictAuthority || first.GradeAuthority || first.ContainmentAuthority || first.ResponseExecutionAuthority || first.SameOperatorClaim || first.RealWorldIdentityClaim || first.WrongdoingClaim {
		t.Fatalf("Fabric contract crossed authority boundary: %#v", first)
	}
	if err := ValidateFabricCampaignEvidenceContract(first); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestFabricCampaignEvidenceContractBindsC6C7C8Fingerprints(t *testing.T) {
	base := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	campaign := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:         "KCAM1-FABRIC",
		Revision:            1,
		State:               GlobalCampaignEmerging,
		FirstObservedAt:     base,
		LastObservedAt:      base,
		Networks:            []string{"solana"},
		Subjects:            []string{"subject:a"},
		ObservationRefs:     []string{"obs:1"},
		ObservedAnchorCount: 1,
	})
	temporal := GlobalCampaignTemporalCorrelationReport{Version: GlobalCampaignTemporalCorrelatorVersion, CampaignRef: campaign.CampaignRef, FingerprintSHA256: "sha256:temporal"}
	radar := GlobalCampaignRadarProjection{Version: GlobalCampaignRadarProjectionVersion, CampaignRef: campaign.CampaignRef, FingerprintSHA256: "sha256:radar"}
	threat := GlobalCampaignThreatFamilyReport{Version: GlobalCampaignThreatFamiliesVersion, CampaignRef: campaign.CampaignRef, FingerprintSHA256: "sha256:threat"}

	contract, err := BuildFabricCampaignEvidenceContract(campaign, temporal, radar, threat, nil)
	if err != nil {
		t.Fatal(err)
	}
	if contract.TemporalFingerprintSHA256 != temporal.FingerprintSHA256 || contract.RadarFingerprintSHA256 != radar.FingerprintSHA256 || contract.ThreatFingerprintSHA256 != threat.FingerprintSHA256 {
		t.Fatalf("fingerprint binding lost: %#v", contract)
	}
}

func TestFabricCampaignEvidenceContractRejectsAuthorityOrCampaignMismatch(t *testing.T) {
	base := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	campaign := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:         "KCAM1-A",
		Revision:            1,
		State:               GlobalCampaignEmerging,
		FirstObservedAt:     base,
		LastObservedAt:      base,
		Networks:            []string{"solana"},
		Subjects:            []string{"subject:a"},
		ObservationRefs:     []string{"obs:1"},
		ObservedAnchorCount: 1,
	})
	_, err := BuildFabricCampaignEvidenceContract(campaign, GlobalCampaignTemporalCorrelationReport{
		Version:           GlobalCampaignTemporalCorrelatorVersion,
		CampaignRef:       "KCAM1-B",
		FingerprintSHA256: "sha256:x",
	}, GlobalCampaignRadarProjection{}, GlobalCampaignThreatFamilyReport{}, nil)
	if err != ErrFabricCampaignEvidenceMismatch {
		t.Fatalf("expected campaign mismatch, got %v", err)
	}

	contract, err := BuildFabricCampaignEvidenceContract(campaign, GlobalCampaignTemporalCorrelationReport{}, GlobalCampaignRadarProjection{}, GlobalCampaignThreatFamilyReport{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	contract.ResponseExecutionAuthority = true
	if err := ValidateFabricCampaignEvidenceContract(contract); err != ErrFabricCampaignEvidenceInvalid {
		t.Fatalf("expected authority rejection, got %v", err)
	}
}

func TestFabricCampaignEvidenceContractTamperFailsClosed(t *testing.T) {
	base := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	campaign := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:         "KCAM1-TAMPER",
		Revision:            1,
		State:               GlobalCampaignEmerging,
		FirstObservedAt:     base,
		LastObservedAt:      base,
		Networks:            []string{"solana"},
		Subjects:            []string{"subject:a"},
		ObservationRefs:     []string{"obs:1"},
		ObservedAnchorCount: 1,
	})
	contract, err := BuildFabricCampaignEvidenceContract(campaign, GlobalCampaignTemporalCorrelationReport{}, GlobalCampaignRadarProjection{}, GlobalCampaignThreatFamilyReport{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	contract.EvidenceRefs = append(contract.EvidenceRefs, "forged:evidence")
	if err := ValidateFabricCampaignEvidenceContract(contract); err != ErrFabricCampaignEvidenceInvalid {
		t.Fatalf("expected hash mismatch rejection, got %v", err)
	}
}
