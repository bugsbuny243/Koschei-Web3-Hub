package services

import "testing"

func TestHolderRolePumpInventoryExcluded(t *testing.T) {
	role, confidence, excluded, program, _ := classifySolanaHolderOwner("pda", &SolanaAccountInfo{Owner: pumpBondingCurveProgramID})
	if role != "pump_bonding_curve_or_protocol_vault" || confidence != "high" || !excluded || program != pumpBondingCurveProgramID {
		t.Fatalf("unexpected classification: role=%s confidence=%s excluded=%t program=%s", role, confidence, excluded, program)
	}
}

func TestHolderRoleSystemWalletIncluded(t *testing.T) {
	role, confidence, excluded, _, _ := classifySolanaHolderOwner("wallet", &SolanaAccountInfo{Owner: solanaSystemProgramID})
	if role != "externally_owned_wallet" || confidence != "high" || excluded {
		t.Fatalf("unexpected classification: role=%s confidence=%s excluded=%t", role, confidence, excluded)
	}
}

func TestHolderRoleUnknownProgramNotSilentlyExcluded(t *testing.T) {
	role, _, excluded, _, _ := classifySolanaHolderOwner("pda", &SolanaAccountInfo{Owner: "UnknownProgram111111111111111111111111111"})
	if role != "program_controlled_unresolved" || excluded {
		t.Fatalf("unknown program must remain in concentration risk: role=%s excluded=%t", role, excluded)
	}
}

func TestHolderRoleConcentration(t *testing.T) {
	top1, top3, top10, top20 := holderRoleConcentration([]float64{40, 30, 20, 10}, 100)
	if top1 != 40 || top3 != 90 || top10 != 100 || top20 != 100 {
		t.Fatalf("unexpected concentration %.2f %.2f %.2f %.2f", top1, top3, top10, top20)
	}
}
