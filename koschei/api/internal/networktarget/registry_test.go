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
	if ethereum.Network.CollectorStatus != "probe_ready" {
		t.Fatalf("EVM implementation status=%q want probe_ready", ethereum.Network.CollectorStatus)
	}
}

func TestCatalogReportsImplementedProbesWithoutClaimingLiveAvailability(t *testing.T) {
	probeReady := map[string]bool{
		"ethereum-mainnet":  false,
		"base-mainnet":      false,
		"arbitrum-mainnet":  false,
		"optimism-mainnet":  false,
		"polygon-mainnet":   false,
		"bnb-mainnet":       false,
		"avalanche-mainnet": false,
		"bitcoin-mainnet":   false,\n\t\t"sui-mainnet":       false,\n\t\t"aptos-mainnet":     false,
	}
	for _, network := range Catalog() {
		if _, ok := probeReady[network.ID]; !ok {
			continue
		}
		if network.CollectorStatus != "probe_ready" {
			t.Fatalf("%s collector status=%q want probe_ready", network.ID, network.CollectorStatus)
		}
		probeReady[network.ID] = true
	}
	for networkID, seen := range probeReady {
		if !seen {
			t.Fatalf("probe-ready network %s missing from catalog", networkID)
		}
	}

	bitcoin, err := Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	if err != nil {
		t.Fatal(err)
	}
	if bitcoin.AnalysisPerformed || bitcoin.EvidenceStatus != "unknown" || bitcoin.LiveAvailability != "not_checked" {
		t.Fatalf("offline Bitcoin resolution claimed deployment/live evidence: %#v", bitcoin)
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


func TestMoveCanonicalAddressResolutionIsNetworkScopedAndStrict(t *testing.T) {
	address := "0x" + strings.Repeat("ab", 32)
	sui, err := Resolve("sui-mainnet", address)
	if err != nil {
		t.Fatal(err)
	}
	aptos, err := Resolve("aptos-mainnet", address)
	if err != nil {
		t.Fatal(err)
	}
	if sui.SubjectID == aptos.SubjectID || sui.CanonicalRef == aptos.CanonicalRef {
		t.Fatal("same Move address was conflated across Sui and Aptos")
	}
	if sui.Classification != "move_hex_32_canonical_syntax_only" || aptos.Classification != "move_hex_32_canonical_syntax_only" {
		t.Fatalf("unexpected Move classification: sui=%#v aptos=%#v", sui, aptos)
	}
	upper, err := Resolve("sui-mainnet", "0x"+strings.Repeat("AB", 32))
	if err != nil || upper.SubjectID != sui.SubjectID {
		t.Fatal("Move hex case normalization changed Sui identity")
	}
	for _, bad := range []string{"0x1", "0x" + strings.Repeat("ab", 20), strings.Repeat("ab", 32)} {
		if _, err := Resolve("sui-mainnet", bad); err == nil {
			t.Fatalf("non-canonical Move address accepted: %q", bad)
		}
	}
}
