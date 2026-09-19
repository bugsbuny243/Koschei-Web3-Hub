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
		SchemaVersion:   networktarget.SchemaVersion,
		Network:         "ethereum-mainnet",
		ChainID:         "0x1",
		ExpectedChainID: "0x1",
		TransactionHash: txHash,
		From:            "0x1111111111111111111111111111111111111111",
		To:              "0x2222222222222222222222222222222222222222",
		InputSHA256:     strings.Repeat("f", 64),
		InputBytes:      4,
		BlockHash:       "0x" + strings.Repeat("c", 64),
		BlockNumber:     "0x10",
		ExecutionState:  networktarget.EVMTransactionExecutionSuccess,
		ReceiptStatus:   "0x1",
		Logs: []networktarget.EVMTransactionLogSummary{{
			Address:  "0x3333333333333333333333333333333333333333",
			Topics:   []string{"0x" + strings.Repeat("b", 64)},
			LogIndex:   "0x0",
			Removed:    true,
			DataSHA256: strings.Repeat("e", 64),
			DataBytes:  32,
		}},
		AnalysisPerformed: true,
		EvidenceStatus:    IntelligenceEvidenceObserved,
		LiveAvailability:  "checked",
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
	logs, ok := projection.Evidence.Attributes["logs"].([]map[string]any)
	if !ok || len(logs) != 1 {
		t.Fatalf("structured logs missing: %#v", projection.Evidence.Attributes["logs"])
	}
	if logs[0]["removed"] != true || logs[0]["log_index"] != "0x0" {
		t.Fatalf("log reorg/index metadata lost: %#v", logs[0])
	}
	if logs[0]["data_sha256"] != strings.Repeat("e", 64) || logs[0]["data_bytes"] != 32 {
		t.Fatalf("log data evidence lost: %#v", logs[0])
	}
	topics, ok := logs[0]["topics"].([]string)
	if !ok || len(topics) != 1 || topics[0] != "0x"+strings.Repeat("b", 64) {
		t.Fatalf("log topics lost: %#v", logs[0]["topics"])
	}
}

func TestAdaptEVMTransactionEvidenceAcceptsUnknownExecutionWithoutPromotingIt(t *testing.T) {
	txHash := "0x" + strings.Repeat("9", 64)
	result := networktarget.EVMTransactionEvidenceResult{
		SchemaVersion:     networktarget.SchemaVersion,
		Network:           "ethereum-mainnet",
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		TransactionHash:   txHash,
		From:              "0x1111111111111111111111111111111111111111",
		BlockHash:         "0x" + strings.Repeat("c", 64),
		BlockNumber:       "0x10",
		ExecutionState:    networktarget.EVMTransactionExecutionUnknown,
		ReceiptRoot:       "0x" + strings.Repeat("d", 64),
		AnalysisPerformed: true,
		EvidenceStatus:    IntelligenceEvidenceObserved,
		LiveAvailability:  "checked",
	}
	projection, err := AdaptEVMTransactionEvidence(result, time.Unix(2, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if projection.Evidence.StateChange != networktarget.EVMTransactionExecutionUnknown {
		t.Fatalf("unknown execution was promoted: %#v", projection.Evidence)
	}
	if projection.Evidence.Attributes["receipt_root"] != result.ReceiptRoot {
		t.Fatalf("receipt root missing: %#v", projection.Evidence.Attributes)
	}
}
