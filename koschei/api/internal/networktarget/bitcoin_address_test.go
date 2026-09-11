package networktarget

import (
	"strings"
	"testing"
)

func TestResolveBitcoinLegacyMainnetAddresses(t *testing.T) {
	addresses := []string{
		"1BoatSLRHtKNngkdXEeobR76b53LETtpyT",
		"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
	}
	for _, address := range addresses {
		resolution, err := Resolve("bitcoin-mainnet", address)
		if err != nil {
			t.Fatalf("Resolve(%q) error: %v", address, err)
		}
		if resolution.CanonicalRef != "bitcoin-mainnet:"+address {
			t.Fatalf("canonical_ref=%q", resolution.CanonicalRef)
		}
		if resolution.Classification != "bitcoin_base58check_syntax_only" {
			t.Fatalf("classification=%q", resolution.Classification)
		}
		if resolution.AnalysisPerformed || resolution.EvidenceStatus != "unknown" {
			t.Fatalf("syntax parse fabricated evidence: %+v", resolution)
		}
	}
}

func TestResolveBitcoinRejectsWrongNetworkAndChecksum(t *testing.T) {
	invalid := []string{
		"mipcBbFg9gMiCh81Kj8tqqdgoZub1ZJRfn",         // testnet P2PKH
		"1BoatSLRHtKNngkdXEeobR76b53LETtpyU",         // bad checksum
		"bc1Qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq", // mixed case
	}
	for _, address := range invalid {
		if _, err := Resolve("bitcoin-mainnet", address); err == nil {
			t.Fatalf("Resolve accepted invalid mainnet address %q", address)
		}
	}
}

func TestResolveBitcoinSegWitV0AndV1(t *testing.T) {
	v0 := makeWitnessAddressForTest(0, make([]byte, 20), bech32ChecksumConst)
	v1Program := make([]byte, 32)
	v1Program[0] = 1
	v1 := makeWitnessAddressForTest(1, v1Program, bech32mChecksumConst)

	resolutionV0, err := Resolve("bitcoin-mainnet", strings.ToUpper(v0))
	if err != nil {
		t.Fatalf("v0 resolve: %v", err)
	}
	if resolutionV0.CanonicalRef != "bitcoin-mainnet:"+v0 {
		t.Fatalf("v0 canonical_ref=%q", resolutionV0.CanonicalRef)
	}
	if resolutionV0.Classification != "bitcoin_segwit_v0_bech32_syntax_only" {
		t.Fatalf("v0 classification=%q", resolutionV0.Classification)
	}

	resolutionV1, err := Resolve("bitcoin-mainnet", v1)
	if err != nil {
		t.Fatalf("v1 resolve: %v", err)
	}
	if resolutionV1.Classification != "bitcoin_segwit_v1_bech32m_syntax_only" {
		t.Fatalf("v1 classification=%q", resolutionV1.Classification)
	}
}

func TestResolveBitcoinRejectsChecksumVariantMismatch(t *testing.T) {
	v0Wrong := makeWitnessAddressForTest(0, make([]byte, 20), bech32mChecksumConst)
	v1Wrong := makeWitnessAddressForTest(1, make([]byte, 32), bech32ChecksumConst)
	for _, address := range []string{v0Wrong, v1Wrong} {
		if _, err := Resolve("bitcoin-mainnet", address); err == nil {
			t.Fatalf("accepted wrong checksum variant %q", address)
		}
	}
}

func makeWitnessAddressForTest(version byte, program []byte, checksumConstant uint32) string {
	converted, ok := convertBits(program, 8, 5, true)
	if !ok {
		panic("test witness conversion failed")
	}
	data := append([]byte{version}, converted...)
	values := append(bech32HRPExpand("bc"), data...)
	values = append(values, make([]byte, 6)...)
	polymod := bech32Polymod(values) ^ checksumConstant
	for index := 0; index < 6; index++ {
		shift := uint(5 * (5 - index))
		data = append(data, byte((polymod>>shift)&31))
	}
	var builder strings.Builder
	builder.WriteString("bc1")
	for _, value := range data {
		builder.WriteByte(bech32Charset[value])
	}
	return builder.String()
}
