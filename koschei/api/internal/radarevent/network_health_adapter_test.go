package radarevent

import (
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/securityevidence"
)

func TestBuildNetworkHealthEventPreservesObservedInfrastructureEvidence(t *testing.T) {
	stake := 2.5
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "ethereum-mainnet",
		SubjectKind:    "validator",
		SubjectID:      "validator-42",
		Source:         "beacon-api:node.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "verified",
		ClientFamily:   "lighthouse",
		StakeSharePct:  &stake,
		ASN:            "AS64500",
		CountryCode:    "DE",
		LocationSource: "provider-infrastructure-registry",
	})
	if err != nil {
		t.Fatal(err)
	}

	sourceDigest := strings.Repeat("a", 64)
	event, err := BuildNetworkHealthEvent("ethereum-beacon-adapter", observation, sourceDigest)
	if err != nil {
		t.Fatal(err)
	}
	if event.Kind != KindNetworkHealth || event.State != securityevidence.StateVerified {
		t.Fatalf("kind=%q state=%q", event.Kind, event.State)
	}
	if event.NetworkID != "ethereum-mainnet" || event.SubjectID != "validator-42" {
		t.Fatalf("unexpected subject: %#v", event)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}

	facts := map[string]Fact{}
	for _, fact := range event.Facts {
		facts[fact.Key] = fact
	}
	for _, key := range []string{"client_family", "stake_share_pct", "asn", "country_code", "location_source"} {
		if _, ok := facts[key]; !ok {
			t.Fatalf("missing fact %q", key)
		}
		if facts[key].EvidenceSHA256 != sourceDigest {
			t.Fatalf("fact %q is not bound to source evidence", key)
		}
	}
	if facts["country_code"].Value != "DE" {
		t.Fatalf("country code=%q", facts["country_code"].Value)
	}
}

func TestBuildNetworkHealthEventDoesNotInventGeography(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "bitcoin-mainnet",
		SubjectKind:    "network",
		SubjectID:      "bitcoin-mainnet",
		Source:         "bitcoin-core",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "observed",
	})
	if err != nil {
		t.Fatal(err)
	}

	event, err := BuildNetworkHealthEvent("bitcoin-core-adapter", observation, strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range event.Facts {
		if fact.Key == "country_code" || fact.Key == "location_source" || fact.Key == "asn" {
			t.Fatalf("invented infrastructure geography fact: %#v", fact)
		}
	}
}

func TestBuildNetworkHealthEventRejectsUnboundSource(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "solana-mainnet",
		SubjectKind:    "validator",
		SubjectID:      "validator-1",
		Source:         "solana-validator-probe",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "observed",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := BuildNetworkHealthEvent("solana-validator-adapter", observation, "not-a-digest"); err == nil {
		t.Fatal("invalid source digest was accepted")
	}
}
