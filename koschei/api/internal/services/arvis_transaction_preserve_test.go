package services

import "testing"

func TestReplaceArvisArmPreservingVerifiedSource(t *testing.T) {
	existing := SecurityRadarVerdict{
		ModuleID: ModulePumpSybilRadar,
		Signed:   true,
		Signals: map[string]any{
			"real_onchain_evidence": true,
		},
		RiskIndex: 42,
	}
	arms := []SecurityRadarVerdict{existing}

	unsigned := SecurityRadarVerdict{
		ModuleID: ModulePumpSybilRadar,
		Signed:   false,
		Signals: map[string]any{
			"real_onchain_evidence": false,
		},
		RiskIndex: 0,
	}
	replaceArvisArmPreservingVerifiedSource(arms, unsigned)
	if !arms[0].Signed || arms[0].RiskIndex != 42 {
		t.Fatalf("verified source arm was overwritten by unsigned enrichment: %#v", arms[0])
	}

	stronger := SecurityRadarVerdict{
		ModuleID: ModulePumpSybilRadar,
		Signed:   true,
		Signals: map[string]any{
			"real_onchain_evidence": true,
		},
		RiskIndex: 58,
	}
	replaceArvisArmPreservingVerifiedSource(arms, stronger)
	if !arms[0].Signed || arms[0].RiskIndex != 58 {
		t.Fatalf("signed enrichment did not replace source arm: %#v", arms[0])
	}
}
