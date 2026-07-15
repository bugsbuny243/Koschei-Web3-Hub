package handlers

import (
	"strings"
	"testing"

	"koschei/api/internal/services"
	"koschei/api/internal/web3"
)

func TestCustomerTokenMappingKeepsRepeatActorEvidenceOutOfLegacyScore(t *testing.T) {
	baselineCore := fixtureHolderCore()
	baseline := applyHolderCoreToTokenRisk(web3.TokenRiskResult{Token: web3.NormalizedTokenData{}}, baselineCore)

	core := fixtureHolderCore()
	core.RepeatDominantHolders = []services.RepeatDominantHolderEvidence{{
		OwnerWallet:       "OwnerA",
		CurrentPercentage: 60,
		TokenCount:        2,
		RiskWeight:        80,
		ObservationWindow: "son 30 gün Koschei gözlemi",
		EvidenceLine:      "REPEAT DOMINANT HOLDER: OwnerA iki tokenda gözlendi.",
	}}
	got := applyHolderCoreToTokenRisk(web3.TokenRiskResult{Token: web3.NormalizedTokenData{}}, core)
	if got.Score != baseline.Score || got.RiskLevel != baseline.RiskLevel {
		t.Fatalf("repeat actor evidence changed legacy score: baseline=%d/%s got=%d/%s", baseline.Score, baseline.RiskLevel, got.Score, got.RiskLevel)
	}
	if score := applyRepeatDominantRiskToLegacyScore(100, core); score != 100 {
		t.Fatalf("deprecated legacy bridge changed score: %d", score)
	}
	joined := strings.Join(got.VerifiedEvidence, " ")
	if !strings.Contains(joined, "REPEAT DOMINANT HOLDER") {
		t.Fatalf("repeat actor evidence was hidden: %v", got.VerifiedEvidence)
	}
}

func TestRepeatDominantLegacyScoreBridgeIsNoOp(t *testing.T) {
	core := fixtureHolderCore()
	core.RepeatDominantHolders = []services.RepeatDominantHolderEvidence{{
		OwnerWallet:       "OwnerA",
		CurrentPercentage: 19.9,
		TokenCount:        1,
		RiskWeight:        0,
	}}
	for _, original := range []int{0, 10, 75, 100} {
		if score := applyRepeatDominantRiskToLegacyScore(original, core); score != original {
			t.Fatalf("legacy score bridge changed %d to %d", original, score)
		}
	}
}
