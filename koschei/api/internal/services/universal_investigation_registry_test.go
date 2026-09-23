package services

import (
	"strings"
	"testing"
)

func TestUniversalInvestigationProfilesKeepPlannedFamiliesNonLive(t *testing.T) {
	plannedFamilies := map[string]bool{
		IntelligenceChainFamilyCosmos:    false,
		IntelligenceChainFamilySubstrate: false,
		IntelligenceChainFamilyTON:       false,
		IntelligenceChainFamilyNEAR:      false,
	}
	ids := map[string]bool{}
	for _, profile := range UniversalInvestigationAdapterProfiles() {
		if profile.ID == "" || ids[profile.ID] {
			t.Fatalf("invalid or duplicate profile id %q", profile.ID)
		}
		ids[profile.ID] = true
		if _, ok := plannedFamilies[profile.ChainFamily]; ok {
			if profile.Status != UniversalAdapterPlanned {
				t.Fatalf("family %s status=%q want planned", profile.ChainFamily, profile.Status)
			}
			plannedFamilies[profile.ChainFamily] = true
		}
		if profile.Domain == "web4" || profile.Domain == "web5" || profile.Domain == "web6" {
			if profile.Status != UniversalAdapterResearch {
				t.Fatalf("%s profile %s status=%q want research", profile.Domain, profile.ID, profile.Status)
			}
			if profile.ChainFamily != IntelligenceChainFamilyOffchain {
				t.Fatalf("%s profile %s unexpectedly claims chain family %q", profile.Domain, profile.ID, profile.ChainFamily)
			}
		}
	}
	for family, seen := range plannedFamilies {
		if !seen {
			t.Fatalf("planned family %s missing", family)
		}
	}
}

func TestUniversalInvestigationProfileForExpandedEVMNetworks(t *testing.T) {
	for _, network := range []string{
		"ethereum-mainnet", "base-mainnet", "arbitrum-mainnet", "optimism-mainnet",
		"polygon-mainnet", "bnb-mainnet", "avalanche-mainnet",
	} {
		profile, ok := UniversalInvestigationProfileForNetwork(network)
		if !ok {
			t.Fatalf("network %s missing universal profile", network)
		}
		if profile.ChainFamily != IntelligenceChainFamilyEVM || profile.Status != UniversalAdapterProbe {
			t.Fatalf("network %s profile=%#v", network, profile)
		}
	}
}

func TestClassifyUniversalInvestigationSubjectSeparatesKindsAndNetworks(t *testing.T) {
	address := "0x1111111111111111111111111111111111111111"
	ethereum, err := ClassifyUniversalInvestigationSubject(address, "ethereum-mainnet", IntelligenceSubjectContract)
	if err != nil {
		t.Fatal(err)
	}
	base, err := ClassifyUniversalInvestigationSubject(address, "base-mainnet", IntelligenceSubjectContract)
	if err != nil {
		t.Fatal(err)
	}
	if ethereum.Kind != IntelligenceSubjectContract || ethereum.ChainFamily != IntelligenceChainFamilyEVM {
		t.Fatalf("ethereum subject=%#v", ethereum)
	}
	if ethereum.ID == base.ID || ethereum.CanonicalRef == base.CanonicalRef {
		t.Fatalf("same EVM address+kind was conflated across chains: ethereum=%#v base=%#v", ethereum, base)
	}

	transaction, err := ClassifyUniversalInvestigationSubject(
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"polygon-mainnet",
		IntelligenceSubjectTransaction,
	)
	if err != nil {
		t.Fatal(err)
	}
	if transaction.Kind != IntelligenceSubjectTransaction || transaction.Chain != "polygon" {
		t.Fatalf("polygon transaction subject=%#v", transaction)
	}
}

func TestClassifyUniversalInvestigationSubjectSupportsNonChainResearchTargetsWithoutClaimingEvidence(t *testing.T) {
	did, err := ClassifyUniversalInvestigationSubject("did:web:example.com", "", IntelligenceSubjectIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if did.ChainFamily != IntelligenceChainFamilyOffchain || did.Network != "global" || did.Kind != IntelligenceSubjectIdentity {
		t.Fatalf("did subject=%#v", did)
	}
	agent, err := ClassifyUniversalInvestigationSubject("agent://research-assistant", "", IntelligenceSubjectAgent)
	if err != nil {
		t.Fatal(err)
	}
	if agent.ChainFamily != IntelligenceChainFamilyOffchain || agent.Kind != IntelligenceSubjectAgent {
		t.Fatalf("agent subject=%#v", agent)
	}
	if did.ID == agent.ID {
		t.Fatal("different non-chain target kinds were conflated")
	}
}

func TestClassifyUniversalInvestigationSubjectFailsClosedForUndeclaredChainKind(t *testing.T) {
	if _, err := ClassifyUniversalInvestigationSubject("proposal-42", "bitcoin-mainnet", IntelligenceSubjectGovernance); err == nil {
		t.Fatal("Bitcoin governance target was accepted despite no declared adapter support")
	}
	if _, err := ClassifyUniversalInvestigationSubject("0x1111111111111111111111111111111111111111", "made-up-mainnet", IntelligenceSubjectContract); err == nil {
		t.Fatal("unregistered network was accepted")
	}
	if _, err := ClassifyUniversalInvestigationSubject("anything", "", "mystery"); err == nil {
		t.Fatal("unsupported target kind was accepted")
	}
}

func TestMoveCanonicalAddressesClassifyWithEvidenceOnlyProbe(t *testing.T) {
	address := "0x" + strings.Repeat("11", 32)
	var firstID string

	for _, network := range []string{"sui-mainnet", "aptos-mainnet"} {
		profile, ok := UniversalInvestigationProfileForNetwork(network)
		if !ok || profile.ChainFamily != IntelligenceChainFamilyMove || profile.Status != UniversalAdapterProbe {
			t.Fatalf("network %s profile=%#v ok=%v", network, profile, ok)
		}

		subject, err := ClassifyUniversalInvestigationSubject(address, network, IntelligenceSubjectAddress)
		if err != nil {
			t.Fatal(err)
		}
		if subject.ChainFamily != IntelligenceChainFamilyMove || subject.Kind != IntelligenceSubjectAddress {
			t.Fatalf("network %s subject=%#v", network, subject)
		}
		if subject.ClassificationBasis != "move_32byte_canonical_address_syntax" {
			t.Fatalf("network %s basis=%q", network, subject.ClassificationBasis)
		}
		if firstID == "" {
			firstID = subject.ID
		} else if firstID == subject.ID {
			t.Fatal("same canonical Move address was conflated across Sui and Aptos")
		}

		plan, err := BuildUniversalInvestigationPlan(address, network, IntelligenceSubjectAddress)
		if err != nil {
			t.Fatal(err)
		}
		if !plan.Executable || plan.Route != UniversalDispatchMoveIdentity || !plan.EvidenceOnly || plan.VerdictAuthority != UniversalVerdictEvidenceOnly {
			t.Fatalf("network %s identity probe plan mismatch: %#v", network, plan)
		}
	}

	invalid := ClassifyIntelligenceSubject("0x1", "sui-mainnet")
	if invalid.Kind != IntelligenceSubjectUnknown {
		t.Fatalf("non-canonical Sui address received canonical classification: %#v", invalid)
	}
}
