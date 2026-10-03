package services

import (
	"reflect"
	"testing"
	"time"
)

func TestGlobalCampaignThreatFamiliesCrossChainProjectionStaysNonAuthoritative(t *testing.T) {
	base := time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)
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
	radar, err := BuildGlobalCampaignRadarProjection("KCAM1-C8", snapshot)
	if err != nil {
		t.Fatal(err)
	}

	report, err := BuildGlobalCampaignThreatFamilies(GlobalCampaignThreatFamilyInput{
		CampaignRef: "KCAM1-C8",
		Radar:       radar,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.FindingCount != 1 || report.Findings[0].RuleID != GlobalCampaignThreatFamilyCrossChain {
		t.Fatalf("unexpected findings: %#v", report.Findings)
	}
	finding := report.Findings[0]
	if finding.EvidenceState != IntelligenceEvidenceVerified || !finding.WatchOnly {
		t.Fatalf("cross-chain continuity boundary changed: %#v", finding)
	}
	if report.VerdictAuthority || report.GradeAuthority || report.ContainmentAuthority || report.SameOperatorClaim || report.RealWorldIdentityClaim || report.WrongdoingClaim {
		t.Fatalf("threat-family report crossed authority boundary: %#v", report)
	}
}

func TestGlobalCampaignThreatFamiliesRecurringTempoRemainsWatchOnly(t *testing.T) {
	tempo := CampaignTempoFingerprintReport{
		Version:           CampaignTempoFingerprintVersion,
		Network:           "solana",
		Available:         true,
		Complete:          true,
		Status:            "verified_campaign_tempo_paths_observed",
		FingerprintSHA256: "sha256:tempo",
		Paths: []CampaignTempoPath{
			{FundingSourceWallet: "funder-a", ActorWallet: "actor-a", TokenMint: "token-a", TempoProfile: "exit_event:sell|f2c=lt_5m|c2l=lt_5m|l2t=5m_30m", EvidenceRefs: []string{"ev:a"}},
			{FundingSourceWallet: "funder-a", ActorWallet: "actor-b", TokenMint: "token-b", TempoProfile: "exit_event:sell|f2c=lt_5m|c2l=lt_5m|l2t=5m_30m", EvidenceRefs: []string{"ev:b"}},
		},
	}
	report, err := BuildGlobalCampaignThreatFamilies(GlobalCampaignThreatFamilyInput{
		CampaignRef:  "KCAM1-TEMPO",
		TempoReports: []CampaignTempoFingerprintReport{tempo},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.FindingCount != 1 {
		t.Fatalf("findings=%#v", report.Findings)
	}
	finding := report.Findings[0]
	if finding.RuleID != GlobalCampaignThreatFamilyTempo || finding.Status != "observed_watch" || finding.EvidenceState != IntelligenceEvidenceObserved || !finding.WatchOnly {
		t.Fatalf("tempo recurrence over-promoted: %#v", finding)
	}
}

func TestGlobalCampaignThreatFamiliesPathwaysAggregateDeterministically(t *testing.T) {
	reports := []ThreatAnticipationReport{
		{
			Version: ThreatAnticipationVersion,
			Target:  "token-b",
			Status:  "evidence_backed_pathway_analysis",
			Pathways: []ThreatPathway{
				{ID: "freeze_abuse", Status: "open", EvidenceStatus: IntelligenceEvidenceObserved, EvidenceKeys: []string{"freeze_authority_present"}},
				{ID: "liquidity_removal", Status: "observed", EvidenceStatus: IntelligenceEvidenceVerified, EvidenceKeys: []string{"liquidity_removal_verified"}},
			},
		},
		{
			Version: ThreatAnticipationVersion,
			Target:  "token-a",
			Status:  "evidence_backed_pathway_analysis",
			Pathways: []ThreatPathway{
				{ID: "mint_inflation", Status: "open", EvidenceStatus: IntelligenceEvidenceVerified, EvidenceKeys: []string{"mint_authority_present"}},
				{ID: "freeze_abuse", Status: "closed", EvidenceStatus: IntelligenceEvidenceVerified, EvidenceKeys: []string{"freeze_authority_present"}},
			},
		},
	}
	first, err := BuildGlobalCampaignThreatFamilies(GlobalCampaignThreatFamilyInput{CampaignRef: "KCAM1-PATH", ThreatReports: reports})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildGlobalCampaignThreatFamilies(GlobalCampaignThreatFamilyInput{CampaignRef: "KCAM1-PATH", ThreatReports: []ThreatAnticipationReport{reports[1], reports[0]}})
	if err != nil {
		t.Fatal(err)
	}
	if first.FingerprintSHA256 != second.FingerprintSHA256 || !reflect.DeepEqual(first, second) {
		t.Fatalf("input order changed result:\n%#v\n%#v", first, second)
	}
	if first.FindingCount != 2 {
		t.Fatalf("findings=%#v", first.Findings)
	}
	var authority, liquidity GlobalCampaignThreatFamilyFinding
	for _, finding := range first.Findings {
		switch finding.RuleID {
		case GlobalCampaignThreatFamilyAuthority:
			authority = finding
		case GlobalCampaignThreatFamilyLiquidity:
			liquidity = finding
		}
	}
	if authority.EvidenceState != IntelligenceEvidenceVerified || !reflect.DeepEqual(authority.Targets, []string{"token-a", "token-b"}) {
		t.Fatalf("authority aggregation=%#v", authority)
	}
	if liquidity.EvidenceState != IntelligenceEvidenceVerified || !reflect.DeepEqual(liquidity.Targets, []string{"token-b"}) {
		t.Fatalf("liquidity aggregation=%#v", liquidity)
	}
}

func TestGlobalCampaignThreatFamiliesIgnoreUnknownAndUnverifiedPathways(t *testing.T) {
	report, err := BuildGlobalCampaignThreatFamilies(GlobalCampaignThreatFamilyInput{
		CampaignRef: "KCAM1-NOISE",
		ThreatReports: []ThreatAnticipationReport{{
			Version:         ThreatAnticipationVersion,
			Target:          "token-a",
			Status:          "insufficient_evidence",
			MissingEvidence: []string{"authority state"},
			Pathways: []ThreatPathway{
				{ID: "mint_inflation", Status: "unknown", EvidenceStatus: IntelligenceEvidenceUnverified},
				{ID: "liquidity_removal", Status: "open", EvidenceStatus: IntelligenceEvidenceUnverified},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.FindingCount != 0 || report.Complete {
		t.Fatalf("unverified pathway became campaign family: %#v", report)
	}
}

func TestGlobalCampaignThreatFamiliesRejectCampaignMismatch(t *testing.T) {
	_, err := BuildGlobalCampaignThreatFamilies(GlobalCampaignThreatFamilyInput{
		CampaignRef: "KCAM1-A",
		Radar: GlobalCampaignRadarProjection{
			Version:           GlobalCampaignRadarProjectionVersion,
			CampaignRef:       "KCAM1-B",
			FingerprintSHA256: "sha256:x",
		},
	})
	if err != ErrGlobalCampaignThreatCampaignMismatch {
		t.Fatalf("expected campaign mismatch, got %v", err)
	}
}

func TestGlobalCampaignThreatFamiliesNoInputsIsIncomplete(t *testing.T) {
	report, err := BuildGlobalCampaignThreatFamilies(GlobalCampaignThreatFamilyInput{CampaignRef: "KCAM1-EMPTY"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Complete || report.FindingCount != 0 || report.FingerprintSHA256 == "" {
		t.Fatalf("empty input semantics=%#v", report)
	}
}
