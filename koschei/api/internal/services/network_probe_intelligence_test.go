package services

import (
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestAdaptEVMProbeEvidenceProducesObservedEvidenceOnly(t *testing.T) {
	resolution, err := networktarget.Resolve("ethereum-mainnet", "0xAbCdEf0000000000000000000000000000001234")
	if err != nil {
		t.Fatalf("resolve EVM: %v", err)
	}
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = "observed"
	resolution.LiveAvailability = "checked"
	observedAt := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	projection, err := AdaptEVMProbeEvidence(networktarget.EVMProbeResult{
		SchemaVersion:     networktarget.SchemaVersion,
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		ContractCodeState: "no_contract_code_observed",
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}, observedAt)
	if err != nil {
		t.Fatalf("adapt EVM probe: %v", err)
	}
	if projection.Subject.ChainFamily != IntelligenceChainFamilyEVM {
		t.Fatalf("subject family=%q", projection.Subject.ChainFamily)
	}
	if projection.Evidence.Status != IntelligenceEvidenceObserved || projection.Evidence.Confidence != 0.8 {
		t.Fatalf("evidence overstated: %+v", projection.Evidence)
	}
	if projection.Evidence.StateChange != "no_contract_code_observed" || projection.Evidence.Provenance != networkProbeEvidenceProvenance {
		t.Fatalf("unexpected evidence projection: %+v", projection.Evidence)
	}
	if projection.Evidence.ObservedAt != observedAt {
		t.Fatalf("observed_at=%s", projection.Evidence.ObservedAt)
	}
}

func TestAdaptEVMProbeEvidenceRejectsChainMismatch(t *testing.T) {
	resolution, _ := networktarget.Resolve("base-mainnet", "0x1111111111111111111111111111111111111111")
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = "observed"
	resolution.LiveAvailability = "checked"
	_, err := AdaptEVMProbeEvidence(networktarget.EVMProbeResult{
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x2105",
		ContractCodeState: "no_contract_code_observed",
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("EVM chain mismatch was accepted")
	}
}

func TestAdaptBitcoinProbeEvidenceProducesCanonicalUTXOEvidence(t *testing.T) {
	resolution, err := networktarget.Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	if err != nil {
		t.Fatalf("resolve Bitcoin: %v", err)
	}
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = "observed"
	resolution.LiveAvailability = "checked"
	genesis, ok := networktarget.ExpectedBitcoinGenesisHash("bitcoin-mainnet")
	if !ok {
		t.Fatal("Bitcoin mainnet trust anchor unavailable")
	}
	observedAt := time.Date(2026, 9, 11, 10, 5, 0, 0, time.UTC)
	projection, err := AdaptBitcoinProbeEvidence(networktarget.BitcoinProbeResult{
		SchemaVersion:       networktarget.SchemaVersion,
		Resolution:          resolution,
		GenesisHash:         genesis,
		ExpectedGenesisHash: genesis,
		ActivityState:       "no_activity_observed",
		ConfirmedTXCount:    0,
		MempoolTXCount:      0,
		FundedSats:          0,
		SpentSats:           0,
		AnalysisPerformed:   true,
		EvidenceStatus:      "observed",
		LiveAvailability:    "checked",
	}, observedAt)
	if err != nil {
		t.Fatalf("adapt Bitcoin probe: %v", err)
	}
	if projection.Subject.ChainFamily != IntelligenceChainFamilyUTXO || projection.Subject.Chain != "bitcoin" {
		t.Fatalf("Bitcoin subject mismatch: %+v", projection.Subject)
	}
	if projection.Evidence.Status != IntelligenceEvidenceObserved || projection.Evidence.Confidence != 0.8 {
		t.Fatalf("Bitcoin evidence overstated: %+v", projection.Evidence)
	}
	if projection.Evidence.StateChange != "no_activity_observed" {
		t.Fatalf("zero activity was changed semantically: %+v", projection.Evidence)
	}
}

func TestAdaptBitcoinProbeEvidenceRejectsUnverifiedNetworkIdentity(t *testing.T) {
	resolution, _ := networktarget.Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = "observed"
	resolution.LiveAvailability = "checked"
	_, err := AdaptBitcoinProbeEvidence(networktarget.BitcoinProbeResult{
		Resolution:          resolution,
		GenesisHash:         "wrong",
		ExpectedGenesisHash: "wrong",
		ActivityState:       "activity_observed",
		AnalysisPerformed:   true,
		EvidenceStatus:      "observed",
		LiveAvailability:    "checked",
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("Bitcoin network mismatch was accepted")
	}
}

func TestNetworkProbeEvidenceRequiresObservationTime(t *testing.T) {
	if _, err := AdaptEVMProbeEvidence(networktarget.EVMProbeResult{}, time.Time{}); err == nil {
		t.Fatal("zero observation time was accepted")
	}
	if _, err := AdaptBitcoinProbeEvidence(networktarget.BitcoinProbeResult{}, time.Time{}); err == nil {
		t.Fatal("zero observation time was accepted")
	}
}
