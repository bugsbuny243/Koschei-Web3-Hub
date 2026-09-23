package radarevent

import (
	"strings"
	"testing"

	"koschei/api/internal/securityevidence"
)

func TestEventDigestIsDeterministicAcrossInputOrder(t *testing.T) {
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	factDigest := strings.Repeat("c", 64)

	first := Event{
		Producer:         "evm-adapter/mainnet",
		Kind:             KindTransaction,
		NetworkID:        "ethereum-mainnet",
		SubjectKind:      "transaction",
		SubjectID:        "0xabc",
		ObservedAtUnixMS: 1780000000000,
		State:            securityevidence.StateVerified,
		NativeRefs: []NativeReference{
			{Kind: "block", Value: "24500000"},
			{Kind: "tx_hash", Value: "0xabc"},
		},
		SourceDigests: []string{digestB, digestA, digestA},
		Facts: []Fact{
			{Key: "status", Value: "success", EvidenceSHA256: factDigest},
			{Key: "value_wei", Value: "1000000000000000000", Unit: "wei"},
		},
	}

	second := first
	second.NativeRefs = []NativeReference{
		{Kind: "tx_hash", Value: "0xabc"},
		{Kind: "block", Value: "24500000"},
	}
	second.SourceDigests = []string{digestA, digestB}
	second.Facts = []Fact{
		{Key: "value_wei", Value: "1000000000000000000", Unit: "wei"},
		{Key: "status", Value: "success", EvidenceSHA256: factDigest},
	}

	sealedFirst, err := first.Seal()
	if err != nil {
		t.Fatal(err)
	}
	sealedSecond, err := second.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if sealedFirst.EventSHA256 != sealedSecond.EventSHA256 {
		t.Fatalf("digest mismatch: %s != %s", sealedFirst.EventSHA256, sealedSecond.EventSHA256)
	}
	if err := sealedFirst.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestObservedAndVerifiedEventsRequireSourceProvenance(t *testing.T) {
	for _, state := range []securityevidence.EvidenceState{
		securityevidence.StateObserved,
		securityevidence.StateVerified,
	} {
		_, err := (Event{
			Producer:         "collector",
			Kind:             KindNetworkHealth,
			NetworkID:        "bitcoin-mainnet",
			SubjectKind:      "network",
			SubjectID:        "bitcoin-mainnet",
			ObservedAtUnixMS: 1780000000000,
			State:            state,
			Facts:            []Fact{{Key: "tip_height", Value: "900000"}},
		}).Seal()
		if err == nil || !strings.Contains(err.Error(), "requires source digest") {
			t.Fatalf("state=%s err=%v", state, err)
		}
	}
}

func TestEventRejectsUnknownNetworkAndUnsupportedKind(t *testing.T) {
	sourceDigest := strings.Repeat("d", 64)
	base := Event{
		Producer:         "collector",
		Kind:             KindAsset,
		NetworkID:        "solana-mainnet",
		SubjectKind:      "asset",
		SubjectID:        "mint",
		ObservedAtUnixMS: 1780000000000,
		State:            securityevidence.StateObserved,
		SourceDigests:    []string{sourceDigest},
		Facts:            []Fact{{Key: "supply", Value: "1"}},
	}

	unknownNetwork := base
	unknownNetwork.NetworkID = "imaginary-mainnet"
	if _, err := unknownNetwork.Seal(); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("unexpected network error: %v", err)
	}

	unknownKind := base
	unknownKind.Kind = "mystery"
	if _, err := unknownKind.Seal(); err == nil || !strings.Contains(err.Error(), "unsupported radar event kind") {
		t.Fatalf("unexpected kind error: %v", err)
	}
}

func TestUnavailableEventCanRepresentExplicitEvidenceGap(t *testing.T) {
	sealed, err := (Event{
		Producer:         "sui-adapter/mainnet",
		Kind:             KindNetworkHealth,
		NetworkID:        "sui-mainnet",
		SubjectKind:      "network",
		SubjectID:        "sui-mainnet",
		ObservedAtUnixMS: 1780000000000,
		State:            securityevidence.StateUnavailable,
	}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	if err := sealed.Verify(); err != nil {
		t.Fatal(err)
	}
}
