package networktarget

import (
	"reflect"
	"testing"
	"time"
)

func telemetryPercent(value float64) *float64 {
	return &value
}

func TestCatalogExposesTelemetryImplementationWithoutClaimingDeploymentHealth(t *testing.T) {
	expected := map[string]string{
		"solana-mainnet":    "validator_probe_ready",
		"ethereum-mainnet":  "execution_and_beacon_probe_ready",
		"base-mainnet":      "rpc_node_and_finality_probe_ready",
		"arbitrum-mainnet":  "rpc_node_and_finality_probe_ready",
		"optimism-mainnet":  "rpc_node_and_finality_probe_ready",
		"polygon-mainnet":   "rpc_node_probe_ready",
		"bnb-mainnet":       "rpc_node_probe_ready",
		"avalanche-mainnet": "rpc_node_probe_ready",
		"bitcoin-mainnet":   "core_node_and_pow_network_probe_ready",
		"sui-mainnet":       "chain_identity_probe_ready",
		"aptos-mainnet":     "ledger_identity_probe_ready",
		"cosmoshub-mainnet": "cometbft_validator_probe_ready",
		"osmosis-mainnet":   "cometbft_validator_probe_ready",
	}
	for _, network := range Catalog() {
		if network.Environment == "" {
			t.Fatalf("%s environment missing", network.ID)
		}
		if network.ConsensusFamily == "" {
			t.Fatalf("%s consensus family missing", network.ID)
		}
		if got, ok := expected[network.ID]; !ok || network.NodeTelemetryStatus != got {
			t.Fatalf("%s node telemetry status=%q want=%q", network.ID, network.NodeTelemetryStatus, got)
		}
	}
}

func TestNormalizeNetworkTelemetryPreservesSourceBackedEvidence(t *testing.T) {
	observedAt := time.Date(2026, 9, 23, 1, 2, 3, 0, time.FixedZone("test", 3*60*60))
	input := NetworkTelemetryInput{
		NetworkID:      "solana-mainnet",
		SubjectKind:    "validator",
		SubjectID:      "validator-vote-account",
		Source:         "solana-rpc:getVoteAccounts",
		ObservedAt:     observedAt,
		EvidenceStatus: "observed",
		StakeSharePct:  telemetryPercent(12.5),
		ASN:            "as13335",
		CountryCode:    "tr",
		LocationSource: "provider-coarse-geo",
	}

	got, err := NormalizeNetworkTelemetry(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != NetworkTelemetrySchemaVersion {
		t.Fatalf("schema=%q", got.SchemaVersion)
	}
	if got.Network.ID != "solana-mainnet" || got.SubjectKind != "validator" || got.SubjectID != "validator-vote-account" {
		t.Fatalf("unexpected identity: %#v", got)
	}
	if got.ObservedAt.Location() != time.UTC || !got.ObservedAt.Equal(observedAt) {
		t.Fatalf("observed_at was not normalized to UTC: %v", got.ObservedAt)
	}
	if got.ASN != "AS13335" || got.CountryCode != "TR" || got.LocationSource != "provider-coarse-geo" {
		t.Fatalf("source-backed coarse location was not normalized: %#v", got)
	}
	if got.StakeSharePct == nil || *got.StakeSharePct != 12.5 || got.HashSharePct != nil {
		t.Fatalf("unexpected consensus contribution: %#v", got)
	}
	if !reflect.DeepEqual(got.MissingEvidence, []string{"client_family"}) {
		t.Fatalf("missing evidence=%v", got.MissingEvidence)
	}
}

func TestNormalizeNetworkTelemetryMakesMissingEvidenceExplicit(t *testing.T) {
	got, err := NormalizeNetworkTelemetry(NetworkTelemetryInput{
		NetworkID:      "bitcoin-mainnet",
		SubjectKind:    "network",
		Source:         "bitcoin-observer",
		ObservedAt:     time.Date(2026, 9, 23, 1, 2, 3, 0, time.UTC),
		EvidenceStatus: "verified",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SubjectID != "bitcoin-mainnet" {
		t.Fatalf("network subject id=%q", got.SubjectID)
	}
	wantMissing := []string{"client_family", "consensus_contribution", "asn", "country_code"}
	if !reflect.DeepEqual(got.MissingEvidence, wantMissing) {
		t.Fatalf("missing evidence=%v want=%v", got.MissingEvidence, wantMissing)
	}
}

func TestNormalizeNetworkTelemetryRejectsUnboundedClaims(t *testing.T) {
	now := time.Date(2026, 9, 23, 1, 2, 3, 0, time.UTC)
	cases := []NetworkTelemetryInput{
		{NetworkID: "unknown-mainnet", SubjectKind: "node", SubjectID: "n", Source: "s", ObservedAt: now, EvidenceStatus: "observed"},
		{NetworkID: "solana-mainnet", SubjectKind: "person", SubjectID: "n", Source: "s", ObservedAt: now, EvidenceStatus: "observed"},
		{NetworkID: "solana-mainnet", SubjectKind: "node", Source: "s", ObservedAt: now, EvidenceStatus: "observed"},
		{NetworkID: "solana-mainnet", SubjectKind: "node", SubjectID: "n", ObservedAt: now, EvidenceStatus: "observed"},
		{NetworkID: "solana-mainnet", SubjectKind: "node", SubjectID: "n", Source: "s", EvidenceStatus: "observed"},
		{NetworkID: "solana-mainnet", SubjectKind: "node", SubjectID: "n", Source: "s", ObservedAt: now, EvidenceStatus: "inferred"},
		{NetworkID: "solana-mainnet", SubjectKind: "validator", SubjectID: "n", Source: "s", ObservedAt: now, EvidenceStatus: "observed", StakeSharePct: telemetryPercent(100.01)},
		{NetworkID: "bitcoin-mainnet", SubjectKind: "miner", SubjectID: "m", Source: "s", ObservedAt: now, EvidenceStatus: "observed", HashSharePct: telemetryPercent(-0.01)},
		{NetworkID: "cosmoshub-mainnet", SubjectKind: "validator", SubjectID: "v", Source: "s", ObservedAt: now, EvidenceStatus: "observed", VotingPowerSharePct: telemetryPercent(100.01)},
		{NetworkID: "solana-mainnet", SubjectKind: "node", SubjectID: "n", Source: "s", ObservedAt: now, EvidenceStatus: "observed", ASN: "not-an-asn"},
		{NetworkID: "solana-mainnet", SubjectKind: "node", SubjectID: "n", Source: "s", ObservedAt: now, EvidenceStatus: "observed", CountryCode: "TR"},
	}
	for i, input := range cases {
		if _, err := NormalizeNetworkTelemetry(input); err == nil {
			t.Fatalf("case %d unexpectedly accepted: %#v", i, input)
		}
	}
}

func TestLookupNetworkDoesNotFallback(t *testing.T) {
	if network, ok := LookupNetwork("ethereum-mainnet"); !ok || network.ConsensusFamily != "proof_of_stake" {
		t.Fatalf("ethereum lookup failed: %#v %v", network, ok)
	}
	if _, ok := LookupNetwork("ethereum"); ok {
		t.Fatal("partial network id unexpectedly resolved")
	}
}
