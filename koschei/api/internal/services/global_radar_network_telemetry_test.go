package services

import (
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestAdaptNetworkTelemetryToGlobalRadarPreservesMissingEvidence(t *testing.T) {
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "solana-mainnet",
		SubjectKind:    "validator",
		SubjectID:      "Vote111",
		Source:         "solana-rpc:getVoteAccounts",
		ObservedAt:     time.Date(2026, 9, 23, 17, 0, 0, 0, time.UTC),
		EvidenceStatus: IntelligenceEvidenceObserved,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := AdaptNetworkTelemetryToGlobalRadar(observation)
	if err != nil {
		t.Fatal(err)
	}
	if got.ObservationKind != GlobalRadarObservationNode ||
		got.Subject.Kind != "validator" ||
		got.Subject.Network != "solana-mainnet" {
		t.Fatalf("unexpected telemetry projection: %#v", got)
	}
	missing, ok := got.Evidence.Attributes["missing_evidence"].([]string)
	if !ok || len(missing) == 0 {
		t.Fatalf("missing evidence was lost: %#v", got.Evidence.Attributes)
	}
	if got.Evidence.Attributes["missing_is_risk"] != false {
		t.Fatalf("missing telemetry was converted into risk: %#v", got.Evidence.Attributes)
	}
	if got.DecisionState != "evidence_only_no_verdict_created" {
		t.Fatalf("telemetry fabricated a verdict: %#v", got)
	}
}

func TestAdaptNetworkTelemetryToGlobalRadarSupportsBitcoinNetworkTelemetry(t *testing.T) {
	hashShare := 12.5
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "bitcoin-mainnet",
		SubjectKind:    "network",
		SubjectID:      "bitcoin-mainnet",
		Source:         "bitcoin-core:getnetworkhashps",
		ObservedAt:     time.Date(2026, 9, 23, 17, 5, 0, 0, time.UTC),
		EvidenceStatus: IntelligenceEvidenceVerified,
		HashSharePct:   &hashShare,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := AdaptNetworkTelemetryToGlobalRadar(observation)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject.ChainFamily != IntelligenceChainFamilyUTXO ||
		got.Subject.Chain != "bitcoin" ||
		got.Evidence.Status != IntelligenceEvidenceVerified ||
		got.Evidence.Confidence != 1 {
		t.Fatalf("unexpected bitcoin telemetry projection: %#v", got)
	}
	if got.Evidence.Attributes["hash_share_pct"] != 12.5 {
		t.Fatalf("hash share missing: %#v", got.Evidence.Attributes)
	}
}

func TestAdaptNetworkTelemetryToGlobalRadarRejectsUnnormalizedInput(t *testing.T) {
	if _, err := AdaptNetworkTelemetryToGlobalRadar(networktarget.NetworkTelemetryObservation{
		SchemaVersion:  "wrong",
		Network:        networktarget.Network{ID: "solana-mainnet", Family: "solana"},
		SubjectKind:    "validator",
		SubjectID:      "Vote111",
		Source:         "source",
		ObservedAt:     time.Now().UTC(),
		EvidenceStatus: IntelligenceEvidenceObserved,
	}); err == nil {
		t.Fatal("wrong telemetry schema was accepted")
	}
}
