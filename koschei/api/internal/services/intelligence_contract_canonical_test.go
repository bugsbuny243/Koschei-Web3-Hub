package services

import "testing"

func TestClassifyIntelligenceSubjectCanonicalizesEVMCase(t *testing.T) {
	mixed := ClassifyIntelligenceSubject("0xAbCdEf0000000000000000000000000000001234", "ethereum-mainnet")
	lower := ClassifyIntelligenceSubject("0xabcdef0000000000000000000000000000001234", "ethereum-mainnet")
	if mixed.ID != lower.ID {
		t.Fatalf("EVM subject IDs differ by address case: %q != %q", mixed.ID, lower.ID)
	}
	if mixed.CanonicalRef != lower.CanonicalRef {
		t.Fatalf("EVM canonical refs differ by address case: %q != %q", mixed.CanonicalRef, lower.CanonicalRef)
	}
	if mixed.Raw == lower.Raw {
		t.Fatal("raw input should preserve caller-provided EVM casing")
	}
}

func TestClassifyIntelligenceSubjectPreservesSolanaCase(t *testing.T) {
	address := "11111111111111111111111111111111"
	subject := ClassifyIntelligenceSubject(address, "solana-mainnet")
	if subject.Raw != address {
		t.Fatalf("raw Solana address changed: %q", subject.Raw)
	}
	if subject.CanonicalRef == "" {
		t.Fatal("Solana canonical ref is empty")
	}
}

func TestClassifyIntelligenceSubjectRecognizesBitcoinBeforeGenericBase58(t *testing.T) {
	address := "1BoatSLRHtKNngkdXEeobR76b53LETtpyT"
	subject := ClassifyIntelligenceSubject(address, "bitcoin-mainnet")
	if subject.ChainFamily != IntelligenceChainFamilyUTXO || subject.Chain != "bitcoin" {
		t.Fatalf("Bitcoin subject misclassified: %+v", subject)
	}
	if subject.Kind != IntelligenceSubjectAddress {
		t.Fatalf("Bitcoin subject kind=%q", subject.Kind)
	}
	if subject.ClassificationBasis != "bitcoin_mainnet_address_syntax" {
		t.Fatalf("Bitcoin classification basis=%q", subject.ClassificationBasis)
	}
	if subject.CanonicalRef != "utxo:bitcoin:bitcoin-mainnet:"+address {
		t.Fatalf("Bitcoin canonical ref=%q", subject.CanonicalRef)
	}
}

func TestClassifyIntelligenceSubjectCanonicalizesBitcoinBech32Case(t *testing.T) {
	lower := "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
	upper := "BC1QXY2KGDYGJRSQTZQ2N0YRF2493P83KKFJHX0WLH"
	lowerSubject := ClassifyIntelligenceSubject(lower, "bitcoin-mainnet")
	upperSubject := ClassifyIntelligenceSubject(upper, "bitcoin-mainnet")
	if lowerSubject.ChainFamily != IntelligenceChainFamilyUTXO || upperSubject.ChainFamily != IntelligenceChainFamilyUTXO {
		t.Fatalf("Bech32 Bitcoin subject not classified as UTXO: lower=%+v upper=%+v", lowerSubject, upperSubject)
	}
	if lowerSubject.ID != upperSubject.ID || lowerSubject.CanonicalRef != upperSubject.CanonicalRef {
		t.Fatalf("Bech32 case changed Bitcoin identity: lower=%+v upper=%+v", lowerSubject, upperSubject)
	}
	if upperSubject.Raw != upper {
		t.Fatalf("raw Bitcoin address casing was not preserved: %q", upperSubject.Raw)
	}
}

func TestClassifyIntelligenceSubjectDoesNotTreatBitcoinAddressAsBitcoinOnSolanaNetwork(t *testing.T) {
	address := "1BoatSLRHtKNngkdXEeobR76b53LETtpyT"
	subject := ClassifyIntelligenceSubject(address, "solana-mainnet")
	if subject.ChainFamily == IntelligenceChainFamilyUTXO {
		t.Fatalf("Bitcoin classification ignored explicit network: %+v", subject)
	}
}
