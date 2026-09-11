package services

import "testing"

func TestUnifiedSignedEvidenceChainFamilyRecognizesBitcoinUTXO(t *testing.T) {
	tests := map[string]string{
		"bitcoin": IntelligenceChainFamilyUTXO,
		"utxo": IntelligenceChainFamilyUTXO,
		"bip122:000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f": IntelligenceChainFamilyUTXO,
		"solana": IntelligenceChainFamilySolana,
		"eip155:1": IntelligenceChainFamilyEVM,
	}
	for chain, want := range tests {
		if got := unifiedSignedEvidenceChainFamily(chain); got != want {
			t.Fatalf("chain %q family=%q want=%q", chain, got, want)
		}
	}
}

func TestUnifiedSignedEvidenceBitcoinSubjectUsesUTXOFamily(t *testing.T) {
	subject := buildUnifiedSignedEvidenceSubject(
		UnifiedSecurityDomainBlockchain,
		IntelligenceSubjectAddress,
		"bitcoin-mainnet",
		securityEvidenceSubject("bitcoin", "address", "bc1qexample"),
	)
	if subject.ChainFamily != IntelligenceChainFamilyUTXO {
		t.Fatalf("chain family=%q", subject.ChainFamily)
	}
	if subject.Chain != "bitcoin" || subject.Network != "bitcoin-mainnet" {
		t.Fatalf("unexpected subject chain/network: %#v", subject)
	}
}

func securityEvidenceSubject(chain, kind, id string) securityevidence.Subject {
	return securityevidence.Subject{Chain: chain, Type: kind, ID: id}
}
