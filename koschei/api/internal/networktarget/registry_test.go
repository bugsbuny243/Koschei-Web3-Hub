package networktarget

import (
	"strings"
	"testing"
)

func TestAddressIdentityIsNetworkScoped(t *testing.T) {
	address := "0x" + strings.Repeat("ab", 20)
	ethereum, err := Resolve("ethereum-mainnet", address)
	if err != nil {
		t.Fatal(err)
	}
	base, err := Resolve("base-mainnet", address)
	if err != nil {
		t.Fatal(err)
	}
	if ethereum.SubjectID == base.SubjectID || ethereum.CanonicalRef == base.CanonicalRef {
		t.Fatal("same address on different networks was conflated")
	}
	upper, err := Resolve("ethereum-mainnet", "0x"+strings.Repeat("AB", 20))
	if err != nil || upper.SubjectID != ethereum.SubjectID {
		t.Fatal("EVM case normalization changed identity")
	}
	if ethereum.AnalysisPerformed || ethereum.EvidenceStatus != "unknown" || ethereum.LiveAvailability != "not_checked" {
		t.Fatal("syntax resolution claimed analysis or live evidence")
	}
	if ethereum.Network.CollectorStatus != "not_connected" {
		t.Fatal("EVM collector was falsely promoted")
	}
}

func TestUnknownAndMismatchedNetworksNeverFallBack(t *testing.T) {
	address := "0x" + strings.Repeat("ab", 20)
	for _, network := range []string{"", "ethereum", "made-up-mainnet", "solana-mainnet", "bitcoin-mainnet"} {
		if _, err := Resolve(network, address); err == nil {
			t.Fatalf("unexpectedly resolved %q", network)
		}
	}
	if _, err := Resolve("ethereum-mainnet", strings.Repeat("1", 32)); err == nil {
		t.Fatal("Solana address accepted as EVM")
	}
}

func TestSolanaRequiresExactly32DecodedBytes(t *testing.T) {
	for _, address := range []string{
		strings.Repeat("1", 32),
		"So11111111111111111111111111111111111111112",
	} {
		result, err := Resolve("solana-mainnet", address)
		if err != nil || !result.SyntaxValid || result.Address != address {
			t.Fatalf("existing Solana address rejected: %q %v", address, err)
		}
	}
	for _, address := range []string{
		strings.Repeat("1", 33), strings.Repeat("z", 44),
		strings.Repeat("0", 32), strings.Repeat("1", 31), "https://example.invalid",
	} {
		if _, err := Resolve("solana-mainnet", address); err == nil {
			t.Fatalf("invalid decoded address accepted: %q", address)
		}
	}
}

func TestCatalogCannotBeMutatedByConsumer(t *testing.T) {
	catalog := Catalog()
	catalog[0].CollectorStatus = "promoted"
	if Catalog()[0].CollectorStatus != "existing" {
		t.Fatal("caller mutated shared registry")
	}
}
