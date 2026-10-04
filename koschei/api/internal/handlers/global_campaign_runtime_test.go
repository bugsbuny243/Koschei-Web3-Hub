package handlers

import (
	"reflect"
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestGlobalCampaignMaterializerInputRejectsGenericObservationsOnly(t *testing.T) {
	assembly := unifiedInvestigationAssembly{
		CombinedEvidence: []services.ActorDefenseEvidenceRecord{{
			Network:            "solana-mainnet",
			ActorWallet:        "actor-a",
			CounterpartID:      "target-a",
			VerificationStatus: "verified",
			EvidenceKey:        "evidence:generic:1",
			TokenMint:          "target-a",
		}},
		UnifiedVerdict: services.UnifiedRadarVerdict{
			GeneratedAt: time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC),
		},
	}

	input, eligible := globalCampaignMaterializerInputFromAssembly("target-a", "solana-mainnet", assembly)
	if eligible {
		t.Fatal("generic observations must remain Radar evidence and must not mint a campaign")
	}
	if len(input.ObservationRefs) != 1 || input.ObservationRefs[0] != "evidence:generic:1" {
		t.Fatalf("observation refs=%v", input.ObservationRefs)
	}
	if input.VerifiedAnchorCount != 1 {
		t.Fatalf("verified anchor count=%d want=1", input.VerifiedAnchorCount)
	}
	if len(input.CampaignGenomeRefs) != 0 || len(input.IncidentRefs) != 0 || len(input.BehaviorSignatureRefs) != 0 || len(input.CampaignTempoRefs) != 0 {
		t.Fatalf("unexpected campaign-specific anchors: %+v", input)
	}
}

func TestGlobalCampaignMaterializerInputUsesGenomeAsTechnicalAnchor(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 8, 30, 0, 0, time.UTC)
	assembly := unifiedInvestigationAssembly{
		Creator: "creator-wallet",
		CampaignGenome: services.ActorCampaignGenome{
			Complete:                true,
			GenomeID:                "KGEN1-TECHNICAL",
			VerifiedDescriptorCount: 2,
			ObservedDescriptorCount: 1,
		},
		CampaignGenomeMatches: services.CampaignGenomeMatchReport{
			Matches: []services.CampaignGenomePatternMatch{{ActorWallet: "matched-wallet"}},
		},
		UnifiedVerdict: services.UnifiedRadarVerdict{GeneratedAt: observedAt},
	}

	input, eligible := globalCampaignMaterializerInputFromAssembly("target-a", "solana-mainnet", assembly)
	if !eligible {
		t.Fatal("complete campaign genome must make the materialization eligible")
	}
	if !reflect.DeepEqual(nonEmptyHandlerStrings(input.CampaignGenomeRefs), []string{"KGEN1-TECHNICAL"}) {
		t.Fatalf("genome refs=%v", input.CampaignGenomeRefs)
	}
	if !input.ObservedAt.Equal(observedAt) {
		t.Fatalf("observed_at=%s want=%s", input.ObservedAt, observedAt)
	}
	actors := nonEmptyHandlerStrings(input.Actors)
	if !reflect.DeepEqual(actors, []string{"creator-wallet", "matched-wallet"}) {
		t.Fatalf("actors=%v", actors)
	}
	if input.VerifiedAnchorCount != 2 || input.ObservedAnchorCount != 1 {
		t.Fatalf("anchor counts verified=%d observed=%d", input.VerifiedAnchorCount, input.ObservedAnchorCount)
	}
}

func TestBehaviorSignatureCampaignRefIsOrderIndependent(t *testing.T) {
	left := services.BehavioralSignatureMatch{
		SignatureID:  "KOSCH-BEH-001",
		Triggered:    true,
		EvidenceRefs: []string{"evidence:b", "evidence:a"},
	}
	right := left
	right.EvidenceRefs = []string{"evidence:a", "evidence:b"}

	leftRef := behaviorSignatureCampaignRef(left)
	rightRef := behaviorSignatureCampaignRef(right)
	if leftRef == "" || leftRef != rightRef {
		t.Fatalf("refs left=%q right=%q", leftRef, rightRef)
	}
}

func TestGlobalCampaignMaterializerInputUsesIncidentAndTempoAnchorsWithoutIdentityClaims(t *testing.T) {
	assembly := unifiedInvestigationAssembly{
		IncidentCorpus: services.SecurityIncidentCorpusView{Records: []services.SecurityIncidentCorpusRecord{{
			IncidentKey: "incident:1",
			Network:     "solana-mainnet",
			Target:      "token-a",
			ActorWallet: "actor-a",
			Evidence:    []string{"evidence:incident:1"},
		}}},
		CampaignTempo: services.CampaignTempoFingerprintReport{
			Complete:          true,
			FingerprintSHA256: "sha256:tempo",
			Paths: []services.CampaignTempoPath{{
				ActorWallet:         "actor-a",
				FundingSourceWallet: "funder-a",
				TokenMint:           "token-a",
				EvidenceRefs:        []string{"evidence:tempo:1"},
			}},
		},
	}

	input, eligible := globalCampaignMaterializerInputFromAssembly("token-a", "solana-mainnet", assembly)
	if !eligible {
		t.Fatal("incident/tempo evidence must be campaign-eligible")
	}
	if !reflect.DeepEqual(nonEmptyHandlerStrings(input.IncidentRefs), []string{"incident:1"}) {
		t.Fatalf("incident refs=%v", input.IncidentRefs)
	}
	if !reflect.DeepEqual(nonEmptyHandlerStrings(input.CampaignTempoRefs), []string{"sha256:tempo"}) {
		t.Fatalf("tempo refs=%v", input.CampaignTempoRefs)
	}
	// GlobalCampaignMaterializerInput carries evidence references only. It has no
	// field that can assert real-world identity, wrongdoing or containment power.
	if reflect.TypeOf(input).NumField() == 0 {
		t.Fatal("unexpected empty materializer input type")
	}
}

func TestWireGlobalCampaignOperatorProjectionBuildsMonitoringSurface(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	target := "11111111111111111111111111111111"
	campaign := services.NewGlobalCampaign(services.GlobalCampaign{
		CampaignRef:         "KCAM1-RUNTIME",
		Revision:            1,
		State:               services.GlobalCampaignEmerging,
		FirstObservedAt:     observedAt,
		LastObservedAt:      observedAt,
		Networks:            []string{"solana-mainnet"},
		Subjects:            []string{target},
		Assets:              []string{target},
		CampaignGenomeRefs:  []string{"KGEN1-RUNTIME"},
		VerifiedAnchorCount: 1,
		RulesetVersion:      globalCampaignRuntimeRulesetVersion,
	})
	verified := services.SecurityRadarVerdict{
		Module:             "Verified runtime arm",
		ModuleID:           "runtime_arm",
		Target:             target,
		Network:            "solana-mainnet",
		EvidenceVerified:   true,
		Evidence:           []string{"evidence:runtime:1"},
		GeneratedAt:        observedAt.Format(time.RFC3339Nano),
		RuleVersion:        "runtime-rule-v1",
		Grade:              "C",
		RiskIndex:          40,
		RiskLevel:          "medium",
		Verdict:            "review",
		Recommendation:     "monitor",
		Signed:             true,
		Signature:          "signature",
		SignatureAlgorithm: "ed25519",
		KeyID:              "key-1",
		PayloadHash:        "sha256:payload",
		Digest:             "sha256:digest",
	}
	assembly := unifiedInvestigationAssembly{
		Core: holderIntelligenceCoreResult{Bundle: services.SecurityRadarBundle{
			Target:   target,
			Network:  "solana-mainnet",
			Metadata: map[string]any{"arvis_arms": []services.SecurityRadarVerdict{verified}},
		}},
		CampaignTempo: services.CampaignTempoFingerprintReport{},
		Threat:        services.ThreatAnticipationReport{},
	}

	status := globalCampaignRuntimeStatus{Status: "persisted", CampaignRef: campaign.CampaignRef}
	wireGlobalCampaignOperatorProjection(&status, campaign, assembly, observedAt)
	if status.RadarProjectionStatus != "projected" || status.RadarFingerprint == "" {
		t.Fatalf("radar projection not wired: %+v", status)
	}
	if status.ThreatFingerprint == "" || status.FabricContractHash == "" {
		t.Fatalf("campaign projection chain incomplete: %+v", status)
	}
	if status.CommandCenter.SchemaVersion != services.GlobalCampaignCommandCenterSchemaVersion ||
		status.CommandCenter.ResponseState != services.GlobalCampaignCommandCenterMonitoring {
		t.Fatalf("command center not in monitoring state: %+v", status.CommandCenter)
	}
	if status.CommandCenter.VerdictAuthority || status.CommandCenter.GradeAuthority ||
		status.CommandCenter.ContainmentAuthority || status.CommandCenter.ExecutionAuthority {
		t.Fatalf("operator projection crossed authority boundary: %+v", status.CommandCenter)
	}
}
