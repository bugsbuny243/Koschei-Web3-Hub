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

func TestHolderRoleRiskBalancesAggregateTokenAccountsByOwner(t *testing.T) {
	accounts := []HolderRoleAccount{
		{TokenAccount: "A1", OwnerWallet: "OwnerA", Balance: 30, Role: "externally_owned_wallet"},
		{TokenAccount: "A2", OwnerWallet: "OwnerA", Balance: 25, Role: "externally_owned_wallet"},
		{TokenAccount: "B1", OwnerWallet: "OwnerB", Balance: 40, Role: "externally_owned_wallet"},
		{TokenAccount: "LP", OwnerWallet: "PoolPDA", Balance: 900, Role: "pump_liquidity_vault", ExcludedFromHolderRisk: true},
	}
	balances := holderRoleRiskBalancesByOwner(accounts)
	if len(balances) != 2 || balances[0] != 55 || balances[1] != 40 {
		t.Fatalf("owner aggregation failed: %#v", balances)
	}
	top1, _, top10, _ := holderRoleConcentration(balances, 95)
	if top1 <= 57.89 || top1 >= 57.90 || top10 != 100 {
		t.Fatalf("owner-level concentration = %.4f / %.4f", top1, top10)
	}
}
