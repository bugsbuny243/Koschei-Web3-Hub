package services

import "testing"

func TestUnifiedSignedEvidenceChainFamilyRecognizesUTXONamespaces(t *testing.T) {
	cases := []string{
		"utxo",
		"bip122:000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
	}
	for _, chain := range cases {
		if got := unifiedSignedEvidenceChainFamily(chain); got != IntelligenceChainFamilyUTXO {
			t.Fatalf("chain %q family=%q want=%q", chain, got, IntelligenceChainFamilyUTXO)
		}
	}
}
