package services

import (
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestAdaptBitcoinTransactionEvidence(t *testing.T) {
	txid := strings.Repeat("a", 64)
	result := networktarget.BitcoinTransactionEvidenceResult{
		SchemaVersion:       networktarget.SchemaVersion,
		Network:             "bitcoin-mainnet",
		GenesisHash:         "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
		ExpectedGenesisHash: "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
		TransactionID:       txid,
		Version:             2,
		InputCount:          1,
		OutputCount:         2,
		Size:                200,
		Weight:              800,
		FeeSats:             1000,
		Confirmed:           true,
		BlockHeight:         900000,
		BlockHash:           strings.Repeat("b", 64),
		BlockTimeUnix:       1700000000,
		TransactionSHA256:   strings.Repeat("c", 64),
		AnalysisPerformed:   true,
		EvidenceStatus:      IntelligenceEvidenceObserved,
		LiveAvailability:    "checked",
	}
	projection, err := AdaptBitcoinTransactionEvidence(result, time.Unix(1700000100, 0).UTC())
	if err != nil {\n\t\tt.Fatal(err)\n\t}
	if projection.Subject.Kind != IntelligenceSubjectTransaction || projection.Subject.Chain != "bitcoin" {
		t.Fatalf("unexpected subject: %+v", projection.Subject)
	}
	if projection.Evidence.TransactionHash != txid || projection.Evidence.BlockOrSlot != 900000 || projection.Evidence.StateChange != "confirmed_observed" {
		t.Fatalf("unexpected evidence: %+v", projection.Evidence)
	}
}
