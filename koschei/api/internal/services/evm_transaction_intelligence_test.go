
package services

import (
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestAdaptEVMTransactionEvidenceRemainsObservedOnly(t *testing.T) {
	txHash := "0x" + strings.Repeat("a", 64)
	result := networktarget.EVMTransactionEvidenceResult{
		SchemaVersion: networktarget.SchemaVersion,
		Network: "ethereum-mainnet",
		ChainID: "0x1",
		ExpectedChainID: "0x1",
		TransactionHash: txHash,
		From: "0x1111111111111111111111111111111111111111",
		To: "0x2222222222222222222222222222222222222222",
		InputSHA256: strings.Repeat("f", 64),
		InputBytes: 4,
		BlockHash: "0x" + strings.Repeat("c", 64),
		BlockNumber: "0x10",
		ExecutionState: networktarget.EVMTransactionExecutionSuccess,
		ReceiptStatus: "0x1",
		AnalysisPerformed: true,
		EvidenceStatus: IntelligenceEvidenceObserved,
		LiveAvailability: "checked",
	}
	projection, err := AdaptEVMTransactionEvidence(result, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if projection.Subject.Kind != IntelligenceSubjectTransaction || projection.Evidence.Status != IntelligenceEvidenceObserved {
		t.Fatalf("unexpected projection: %+v", projection)
	}
	if projection.Evidence.TransactionHash != txHash || projection.Evidence.BlockOrSlot != 16 {
		t.Fatalf("missing transaction anchors: %+v", projection.Evidence)
	}
}
