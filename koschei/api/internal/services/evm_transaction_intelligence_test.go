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
		SchemaVersion:      networktarget.SchemaVersion,
		Network:            "ethereum-mainnet",
		ChainID:            "0x1",
		ExpectedChainID:    "0x1",
		TransactionHash:    txHash,
		From:               "0x1111111111111111111111111111111111111111",
		To:                 "0x2222222222222222222222222222222222222222",
		InputSHA256:        strings.Repeat("f", 64),
		InputBytes:         68,
		InputSelector:      "0xa9059cbb",
		InputSelectorHint:  "transfer(address,uint256)",
		StandardEventCount: 1,
		TransferEventCount: 1,
		BlockHash:          "0x" + strings.Repeat("c", 64),
		BlockNumber:        "0x10",
		ExecutionState:     networktarget.EVMTransactionExecutionSuccess,
		ReceiptStatus:      "0x1",
		Logs: []networktarget.EVMTransactionLogSummary{{
			Address:        "0x3333333333333333333333333333333333333333",
			Topics:         []string{"0x" + strings.Repeat("b", 64)},
			LogIndex:       "0x0",
			Removed:        true,
			DataSHA256:     strings.Repeat("e", 64),
			DataBytes:      32,
			SemanticKind:   "standard_transfer",
			SemanticLayout: "erc20_like",
			FromAddress:    "0x1111111111111111111111111111111111111111",
			ToAddress:      "0x2222222222222222222222222222222222222222",
			ValueHex:       "0x1",
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
	if projection.Evidence.Attributes["input_selector"] != "0xa9059cbb" ||
		projection.Evidence.Attributes["input_selector_hint"] != "transfer(address,uint256)" ||
		projection.Evidence.Attributes["selector_hint_only"] != true ||
		projection.Evidence.Attributes["standard_event_count"] != 1 ||
		projection.Evidence.Attributes["transfer_event_count"] != 1 {
		t.Fatalf("semantic transaction evidence missing: %#v", projection.Evidence.Attributes)
	}
	if logs[0]["semantic_kind"] != "standard_transfer" || logs[0]["semantic_layout"] != "erc20_like" ||
		logs[0]["from_address"] != "0x1111111111111111111111111111111111111111" ||
		logs[0]["to_address"] != "0x2222222222222222222222222222222222222222" || logs[0]["value_hex"] != "0x1" {
		t.Fatalf("semantic log projection missing: %#v", logs[0])
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
