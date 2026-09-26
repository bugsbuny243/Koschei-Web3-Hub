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
		FeeSats:           1000,
		TotalInputSats:    5000,
		TotalOutputSats:   4000,
		DerivedFeeSats:    1000,
		FeeBalanceChecked: true,
		FeeConsistent:     true,
		Inputs: []networktarget.BitcoinTransactionInputFlow{{
			Index:                      0,
			PreviousTxID:               strings.Repeat("d", 64),
			PreviousVout:               1,
			Sequence:                   4294967293,
			PreviousValueSats:          5000,
			PreviousScriptType:         "p2pkh",
			PreviousAddress:            "1BoatSLRHtKNngkdXEeobR76b53LETtpyT",
			PreviousScriptPubKeySHA256: strings.Repeat("e", 64),
		}},
		Outputs: []networktarget.BitcoinTransactionOutputFlow{
			{Index: 0, ValueSats: 2500, ScriptType: "p2pkh", Address: "1BoatSLRHtKNngkdXEeobR76b53LETtpyT", ScriptPubKeySHA256: strings.Repeat("f", 64)},
			{Index: 1, ValueSats: 1500, ScriptType: "op_return", ScriptPubKeySHA256: strings.Repeat("0", 64)},
		},
		Confirmed:         true,
		BlockHeight:       900000,
		BlockHash:         strings.Repeat("b", 64),
		BlockTimeUnix:     1700000000,
		TransactionSHA256: strings.Repeat("c", 64),
		AnalysisPerformed: true,
		EvidenceStatus:    IntelligenceEvidenceObserved,
		LiveAvailability:  "checked",
	}
	projection, err := AdaptBitcoinTransactionEvidence(result, time.Unix(1700000100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if projection.Subject.Kind != IntelligenceSubjectTransaction || projection.Subject.Chain != "bitcoin" {
		t.Fatalf("unexpected subject: %+v", projection.Subject)
	}
	if projection.Evidence.TransactionHash != txid || projection.Evidence.BlockOrSlot != 900000 || projection.Evidence.StateChange != "confirmed_observed" {
		t.Fatalf("unexpected evidence: %+v", projection.Evidence)
	}
	attrs := projection.Evidence.Attributes
	if attrs["total_input_sats"] != int64(5000) || attrs["total_output_sats"] != int64(4000) ||
		attrs["derived_fee_sats"] != int64(1000) || attrs["fee_consistent"] != true ||
		attrs["flow_scope"] != "utxo_prevout_output_observation" {
		t.Fatalf("Bitcoin UTXO flow totals missing: %#v", attrs)
	}
	inputs, ok := attrs["input_flows"].([]map[string]any)
	if !ok || len(inputs) != 1 || inputs[0]["previous_txid"] != strings.Repeat("d", 64) ||
		inputs[0]["previous_value_sats"] != int64(5000) {
		t.Fatalf("Bitcoin input flow projection missing: %#v", attrs["input_flows"])
	}
	outputs, ok := attrs["output_flows"].([]map[string]any)
	if !ok || len(outputs) != 2 || outputs[0]["value_sats"] != int64(2500) || outputs[1]["script_type"] != "op_return" {
		t.Fatalf("Bitcoin output flow projection missing: %#v", attrs["output_flows"])
	}
	if attrs["input_address_count"] != 1 || attrs["output_address_count"] != 1 {
		t.Fatalf("Bitcoin address flow counts missing: %#v", attrs)
	}
}
