package handlers

import (
	"testing"

	"koschei/api/internal/services"
	"koschei/api/internal/web3"
)

func TestCustomerTokenMappingAppliesRepeatDominantRiskWeight(t *testing.T) {
	core := fixtureHolderCore()
	core.RepeatDominantHolders = []services.RepeatDominantHolderEvidence{{
		OwnerWallet:       "OwnerA",
		CurrentPercentage: 60,
		TokenCount:        2,
		RiskWeight:        80,
		ObservationWindow: "son 30 gün Koschei gözlemi",
	}}

	got := applyHolderCoreToTokenRisk(web3.TokenRiskResult{Token: web3.NormalizedTokenData{}}, core)
	if got.Score != 20 || got.RiskLevel != "high" {
		t.Fatalf("repeat-holder risk did not reach legacy customer score: score=%d risk=%s", got.Score, got.RiskLevel)
	}
	if score := applyRepeatDominantRiskToLegacyScore(100, core); score != 20 {
		t.Fatalf("public legacy score bridge = %d", score)
	}
	if score := applyRepeatDominantRiskToLegacyScore(10, core); score != 10 {
		t.Fatalf("repeat risk must not make an already-worse score reassuring: %d", score)
	}
}

func TestRepeatDominantLegacyScoreIgnoresUnqualifiedEvidence(t *testing.T) {
	core := fixtureHolderCore()
	core.RepeatDominantHolders = []services.RepeatDominantHolderEvidence{{
		OwnerWallet:       "OwnerA",
		CurrentPercentage: 19.9,
		TokenCount:        1,
		RiskWeight:        0,
	}}
	if score := applyRepeatDominantRiskToLegacyScore(75, core); score != 75 {
		t.Fatalf("unqualified repeat evidence changed score: %d", score)
	}
}
