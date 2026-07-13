package handlers

import (
	"strings"
	"testing"

	"koschei/api/internal/services"
	"koschei/api/internal/web3"
)

func fixtureHolderCore() holderIntelligenceCoreResult {
	return holderIntelligenceCoreResult{
		Roles: services.HolderRoleAnalysis{
			Available: true, Status: "verified_role_resolution", Supply: 1000, CirculatingSupply: 100,
			ProtocolControlledPercentage: 90,
		},
		Intelligence: services.HolderIntelligence{
			Available: true, Status: "verified_owner_aggregated_holdings", OwnerAggregationApplied: true,
			OwnerCount: 2, CirculatingSupply: 100, Top1Percentage: 60, Top10Percentage: 100,
			Rows: []services.HolderIntelligenceRow{{
				OwnerWallet: "OwnerA", OwnerResolved: true, TokenAccounts: []string{"A1", "A2"},
				TokenAccountCount: 2, Balance: 60, RiskBearing: true,
				ObservationTier: "deep", ObservationStatus: "verified_bounded_observation",
				SignaturesFetched: 100, ParsedTransactions: 10, HistoryExhausted: false,
			}},
		},
		Cluster: services.HolderClusterAnalysis{
			WalletsRequested: 2, DeepOwnersScanned: 1, ShallowOwnersScanned: 1,
			RPCBudget: 600, RPCCallsUsed: 14,
		},
	}
}

func TestCustomerTokenMappingUsesOwnerNormalizedConcentration(t *testing.T) {
	base := web3.TokenRiskResult{Token: web3.NormalizedTokenData{
		Mint: "Mint", Network: "solana-mainnet", LargestHolderPercent: 99, TopTenPercent: 99,
	}}
	got := applyHolderCoreToTokenRisk(base, fixtureHolderCore())
	if got.Token.LargestHolderPercent != 60 || got.Token.TopTenPercent != 100 {
		t.Fatalf("legacy fields were not populated from normalized owners: %#v", got.Token)
	}
	if got.Score != 45 || got.RiskLevel != "medium" {
		t.Fatalf("legacy thresholds changed or used raw concentration: score=%d risk=%s", got.Score, got.RiskLevel)
	}
	if got.HolderIntelligence.Rows[0].TokenAccountCount != 2 || got.FinalPolicy != "evidence_backed" || got.VerdictWithheld {
		t.Fatalf("normalized holder result missing: %#v", got)
	}
}

func TestCustomerAndOwnerSurfacesShareNormalizedConcentration(t *testing.T) {
	core := fixtureHolderCore()
	ownerTop1, ownerTop10, ok := holderIntelligenceCoreConcentration(core)
	if !ok {
		t.Fatal("shared holder core did not expose concentration")
	}
	customer := applyHolderCoreToTokenRisk(web3.TokenRiskResult{Token: web3.NormalizedTokenData{}}, core)
	if customer.Token.LargestHolderPercent != roundPercent(ownerTop1) || customer.Token.TopTenPercent != roundPercent(ownerTop10) {
		t.Fatalf("customer concentration diverged from OwnerRadar core: customer=%#v owner=%.4f/%.4f", customer.Token, ownerTop1, ownerTop10)
	}
}

func TestCustomerMappingPreservesDeepAndShallowObservationMetadata(t *testing.T) {
	core := fixtureHolderCore()
	core.Intelligence.Rows = append(core.Intelligence.Rows, services.HolderIntelligenceRow{
		OwnerWallet: "OwnerB", OwnerResolved: true, RiskBearing: true,
		ObservationTier: "shallow", ObservationStatus: "signature_only_observation",
		SignaturesFetched: 20, ParsedTransactions: 2, ObservationWindowExhausted: true,
	})
	got := applyHolderCoreToTokenRisk(web3.TokenRiskResult{Token: web3.NormalizedTokenData{}}, core)
	if got.HolderIntelligence.Rows[0].ObservationTier != "deep" || got.HolderIntelligence.Rows[0].SignaturesFetched != 100 || got.HolderIntelligence.Rows[0].ParsedTransactions != 10 {
		t.Fatalf("deep observation metadata was lost: %#v", got.HolderIntelligence.Rows[0])
	}
	if got.HolderIntelligence.Rows[1].ObservationTier != "shallow" || got.HolderIntelligence.Rows[1].SignaturesFetched != 20 || got.HolderIntelligence.Rows[1].ParsedTransactions != 2 || !got.HolderIntelligence.Rows[1].ObservationWindowExhausted {
		t.Fatalf("shallow observation metadata was lost: %#v", got.HolderIntelligence.Rows[1])
	}
}

func TestCustomerTokenMappingCarriesLaunchAndCreatorEvidence(t *testing.T) {
	core := fixtureHolderCore()
	core.LaunchForensics = services.LaunchForensicsAnalysis{
		Available: true, Status: "verified_launch_forensics", Summary: "Verified launch window.",
		Profiles: []services.LaunchActorProfile{{
			OwnerWallet: "OwnerA", Label: "SNIPER", CreatorLinked: true,
			FundingStatus: "creator_linked", Evidence: []string{"CREATOR_LINKED"},
		}},
		Findings: []string{"OwnerA entered first and is creator-linked by bounded funding evidence."},
	}
	core.Intelligence.LaunchForensicsAvailable = true
	core.Intelligence.Rows[0].LaunchBehaviorLabel = "SNIPER"
	core.Intelligence.Rows[0].LaunchEntryRank = 1
	core.Intelligence.Rows[0].LaunchCreatorLinked = true
	core.Intelligence.Rows[0].LaunchFundingStatus = "creator_linked"
	got := applyHolderCoreToTokenRisk(web3.TokenRiskResult{Token: web3.NormalizedTokenData{}}, core)
	if !got.LaunchForensics.Available || !got.HolderIntelligence.Rows[0].LaunchCreatorLinked {
		t.Fatalf("launch forensics did not reach customer result: %#v", got)
	}
	joined := strings.Join(got.VerifiedEvidence, " ")
	if !strings.Contains(joined, "creator-linked") || !strings.Contains(joined, "Verified launch window") {
		t.Fatalf("creator/launch evidence missing: %s", joined)
	}
}

func TestHolderCoreWithholdsUnavailableOrBlockingEvidence(t *testing.T) {
	core := fixtureHolderCore()
	core.Intelligence.FinalVerdictBlocked = true
	core.Roles.BlockingEvidenceGap = true
	core.Final = services.SecurityRadarFinalVerdict{Signed: true, RiskLevel: "low", RiskIndex: 5}
	got := applyHolderCoreToTokenRisk(web3.TokenRiskResult{Token: web3.NormalizedTokenData{}}, core)
	if got.FinalPolicy != "withhold" || !got.VerdictWithheld || holderIntelligenceCoreShieldAction(core) != "withhold" {
		t.Fatalf("blocking evidence was converted into a reassuring verdict: %#v", got)
	}
}

func TestPreflightKeepsHolderSectionWhenRPCBudgetDegrades(t *testing.T) {
	core := fixtureHolderCore()
	core.Final = services.SecurityRadarFinalVerdict{Signed: true, RiskLevel: "high", RiskIndex: 70}
	core.Cluster.RPCCallsUsed = core.Cluster.RPCBudget
	core.Intelligence.Rows = append(core.Intelligence.Rows, services.HolderIntelligenceRow{
		OwnerWallet: "OwnerB", OwnerResolved: true, RiskBearing: true,
		ObservationTier: "shallow", ObservationStatus: "rpc_budget_exhausted",
		ObservationBudgetDegraded: true,
	})
	if action := holderIntelligenceCoreShieldAction(core); action != "warn" {
		t.Fatalf("preflight did not use the shared evidence-backed final: %s", action)
	}
	if !core.Intelligence.Available || len(core.Intelligence.Rows) != 2 || !core.Intelligence.Rows[1].ObservationBudgetDegraded {
		t.Fatalf("budget degradation removed holder intelligence: %#v", core.Intelligence)
	}
	if explanation := holderIntelligenceCoreExplanation(core); !strings.Contains(explanation, "budget-degraded") || !strings.Contains(explanation, "not classified as safe or organic") {
		t.Fatalf("budget limitation was not explained honestly: %s", explanation)
	}
}

func TestHolderCoreExplanationStatesBoundedObservation(t *testing.T) {
	core := fixtureHolderCore()
	core.Intelligence.Rows = append(core.Intelligence.Rows, services.HolderIntelligenceRow{
		OwnerWallet: "OwnerB", ObservationTier: "shallow", ObservationStatus: "no_observed_signatures",
		ObservationWindowExhausted: true,
	})
	text := holderIntelligenceCoreExplanation(core)
	for _, expected := range []string{"aggregated", "reported separately", "Bounded behavior observation", "not classified as safe or organic"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in explanation: %s", expected, text)
		}
	}
}
