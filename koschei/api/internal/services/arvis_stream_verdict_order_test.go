package services

import "testing"

func TestPrioritizeFinalArvisArm(t *testing.T) {
	arms := []SecurityRadarVerdict{
		{ModuleID: ModulePumpSybilRadar},
		{ModuleID: ModuleHolderConcentration},
		{ModuleID: ModuleFinalVerdictEngine},
		{ModuleID: ModuleMEVShield},
	}
	ordered := prioritizeFinalArvisArm(arms)
	if len(ordered) != len(arms) {
		t.Fatalf("expected %d arms, got %d", len(arms), len(ordered))
	}
	if ordered[0].ModuleID != ModuleFinalVerdictEngine {
		t.Fatalf("final verdict must be first, got %s", ordered[0].ModuleID)
	}
	seen := map[string]int{}
	for _, arm := range ordered {
		seen[arm.ModuleID]++
	}
	for _, arm := range arms {
		if seen[arm.ModuleID] != 1 {
			t.Fatalf("arm %s was lost or duplicated: %#v", arm.ModuleID, ordered)
		}
	}
}
