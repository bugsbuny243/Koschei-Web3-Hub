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
