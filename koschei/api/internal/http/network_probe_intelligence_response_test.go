package http

import (
	"encoding/json"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
)

func TestEVMNetworkProbeResponsePreservesRootContractAndAddsIntelligence(t *testing.T) {
	resolution, err := networktarget.Resolve("ethereum-mainnet", "0xabcdef0000000000000000000000000000001234")
	if err != nil {
		t.Fatalf("resolve EVM target: %v", err)
	}
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = services.IntelligenceEvidenceObserved
	resolution.LiveAvailability = "checked"
	result := networktarget.EVMProbeResult{
		SchemaVersion:     networktarget.SchemaVersion,
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		ContractCodeState: "no_contract_code_observed",
		AnalysisPerformed: true,
		EvidenceStatus:    services.IntelligenceEvidenceObserved,
		LiveAvailability:  "checked",
	}
	projection, err := services.AdaptEVMProbeEvidence(result, time.Date(2026, 9, 11, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("adapt EVM probe: %v", err)
	}

	assertNetworkProbeResponseContract(t, evmNetworkProbeResponse{EVMProbeResult: result, Intelligence: projection}, "chain_id", "0x1")
}

func TestBitcoinNetworkProbeResponsePreservesRootContractAndAddsIntelligence(t *testing.T) {
	resolution, err := networktarget.Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	if err != nil {
		t.Fatalf("resolve Bitcoin target: %v", err)
	}
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = services.IntelligenceEvidenceObserved
	resolution.LiveAvailability = "checked"
	genesis, ok := networktarget.ExpectedBitcoinGenesisHash("bitcoin-mainnet")
	if !ok {
		t.Fatal("Bitcoin mainnet genesis anchor unavailable")
	}
	result := networktarget.BitcoinProbeResult{
		SchemaVersion:       networktarget.SchemaVersion,
		Resolution:          resolution,
		GenesisHash:         genesis,
		ExpectedGenesisHash: genesis,
		ActivityState:       "no_activity_observed",
		AnalysisPerformed:   true,
		EvidenceStatus:      services.IntelligenceEvidenceObserved,
		LiveAvailability:    "checked",
	}
	projection, err := services.AdaptBitcoinProbeEvidence(result, time.Date(2026, 9, 11, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("adapt Bitcoin probe: %v", err)
	}

	assertNetworkProbeResponseContract(t, bitcoinNetworkProbeResponse{BitcoinProbeResult: result, Intelligence: projection}, "activity_state", "no_activity_observed")
}

func assertNetworkProbeResponseContract(t *testing.T, response any, rootKey, rootValue string) {
	t.Helper()
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, _ := payload[rootKey].(string); got != rootValue {
		t.Fatalf("root field %q=%q want=%q payload=%s", rootKey, got, rootValue, string(encoded))
	}
	intelligence, ok := payload["intelligence"].(map[string]any)
	if !ok {
		t.Fatalf("intelligence projection missing: %s", string(encoded))
	}
	if _, exists := intelligence["decision"]; exists {
		t.Fatalf("probe projection must not create a decision: %s", string(encoded))
	}
	evidence, ok := intelligence["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("intelligence evidence missing: %s", string(encoded))
	}
	if status, _ := evidence["status"].(string); status != services.IntelligenceEvidenceObserved {
		t.Fatalf("evidence status=%q payload=%s", status, string(encoded))
	}
}
