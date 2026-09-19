
package services

import "testing"

func TestUniversalDispatcherRoutesLiveAndProbeTargetsWithoutOverclaiming(t *testing.T) {
	solana, err := BuildUniversalInvestigationPlan(
		"11111111111111111111111111111111",
		"solana-mainnet",
		IntelligenceSubjectAddress,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !solana.Executable || solana.Route != UniversalDispatchSolanaCore || solana.EvidenceOnly ||
		solana.VerdictAuthority != UniversalVerdictExistingRulesOnly {
		t.Fatalf("unexpected Solana plan: %#v", solana)
	}

	evm, err := BuildUniversalInvestigationPlan(
		"0x1111111111111111111111111111111111111111",
		"polygon-mainnet",
		IntelligenceSubjectContract,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !evm.Executable || evm.Route != UniversalDispatchEVMProbe || !evm.EvidenceOnly ||
		evm.VerdictAuthority != UniversalVerdictEvidenceOnly {
		t.Fatalf("unexpected EVM plan: %#v", evm)
	}

	bitcoin, err := BuildUniversalInvestigationPlan(
		"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kygt080",
		"bitcoin-mainnet",
		IntelligenceSubjectAddress,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !bitcoin.Executable || bitcoin.Route != UniversalDispatchBitcoinProbe || !bitcoin.EvidenceOnly ||
		bitcoin.VerdictAuthority != UniversalVerdictEvidenceOnly {
		t.Fatalf("unexpected Bitcoin plan: %#v", bitcoin)
	}
}

func TestUniversalDispatcherKeepsDeclaredButUnconnectedKindsNonExecutable(t *testing.T) {
	evmTx, err := BuildUniversalInvestigationPlan(
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"ethereum-mainnet",
		IntelligenceSubjectTransaction,
	)
	if err != nil {
		t.Fatal(err)
	}
	if evmTx.Executable || evmTx.Route != UniversalDispatchDeclaredOnly || evmTx.VerdictAuthority != UniversalVerdictNone {
		t.Fatalf("EVM transaction was overclaimed: %#v", evmTx)
	}

	btcTx, err := BuildUniversalInvestigationPlan(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bitcoin-mainnet",
		IntelligenceSubjectTransaction,
	)
	if err != nil {
		t.Fatal(err)
	}
	if btcTx.Executable || btcTx.Route != UniversalDispatchDeclaredOnly || btcTx.VerdictAuthority != UniversalVerdictNone {
		t.Fatalf("Bitcoin transaction was overclaimed: %#v", btcTx)
	}
}

func TestUniversalDispatcherKeepsPlannedFamiliesFailClosed(t *testing.T) {
	cases := []struct {
		target  string
		network string
		kind    string
	}{
		{"0x1", "sui-mainnet", IntelligenceSubjectContract},
		{"0x1", "aptos-mainnet", IntelligenceSubjectContract},
		{"cosmos1placeholder", "cosmoshub-mainnet", IntelligenceSubjectAddress},
		{"1:placeholder", "ton-mainnet", IntelligenceSubjectContract},
		{"example.near", "near-mainnet", IntelligenceSubjectContract},
	}
	for _, tc := range cases {
		plan, err := BuildUniversalInvestigationPlan(tc.target, tc.network, tc.kind)
		if err != nil {
			t.Fatalf("%s/%s: %v", tc.network, tc.kind, err)
		}
		if plan.Executable || plan.Route != UniversalDispatchPlannedAdapter || plan.VerdictAuthority != UniversalVerdictNone {
			t.Fatalf("%s overclaimed planned adapter: %#v", tc.network, plan)
		}
	}
}

func TestUniversalDispatcherKeepsWeb4Web5Web6ResearchOnly(t *testing.T) {
	cases := []struct {
		target string
		kind   string
		domain string
	}{
		{"did:web:example.com", IntelligenceSubjectIdentity, "web5"},
		{"agent://research-assistant", IntelligenceSubjectAgent, "web4"},
		{"artifact://signed-build", IntelligenceSubjectArtifact, ""},
	}
	for _, tc := range cases {
		plan, err := BuildUniversalInvestigationPlan(tc.target, "", tc.kind)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Executable || plan.Route != UniversalDispatchResearchAdapter || plan.VerdictAuthority != UniversalVerdictNone {
			t.Fatalf("research target overclaimed: %#v", plan)
		}
		if tc.domain != "" && plan.Adapter.Domain != tc.domain {
			t.Fatalf("target %s domain=%q want=%q", tc.target, plan.Adapter.Domain, tc.domain)
		}
	}
}

func TestUniversalDispatcherRejectsExecutableAddressKindWithInvalidSyntax(t *testing.T) {
	if _, err := BuildUniversalInvestigationPlan("not-an-evm-address", "ethereum-mainnet", IntelligenceSubjectContract); err == nil {
		t.Fatal("invalid EVM contract target was accepted")
	}
	if _, err := BuildUniversalInvestigationPlan("not-bitcoin", "bitcoin-mainnet", IntelligenceSubjectAddress); err == nil {
		t.Fatal("invalid Bitcoin address target was accepted")
	}
}
