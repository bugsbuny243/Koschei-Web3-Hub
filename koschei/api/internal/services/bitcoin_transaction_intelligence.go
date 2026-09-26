package services

import (
	"errors"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

func bitcoinInputFlowAttributes(inputs []networktarget.BitcoinTransactionInputFlow) ([]map[string]any, map[string]int, int) {
	flows := make([]map[string]any, 0, len(inputs))
	scriptTypes := map[string]int{}
	addresses := map[string]struct{}{}
	for _, input := range inputs {
		flow := map[string]any{
			"index":               input.Index,
			"previous_vout":       input.PreviousVout,
			"coinbase":            input.Coinbase,
			"sequence":            input.Sequence,
			"previous_value_sats": input.PreviousValueSats,
		}
		if input.PreviousTxID != "" {
			flow["previous_txid"] = input.PreviousTxID
		}
		if input.PreviousScriptType != "" {
			flow["previous_script_type"] = input.PreviousScriptType
			scriptTypes[input.PreviousScriptType]++
		}
		if input.PreviousAddress != "" {
			flow["previous_address"] = input.PreviousAddress
			addresses[input.PreviousAddress] = struct{}{}
		}
		if input.PreviousScriptPubKeySHA256 != "" {
			flow["previous_scriptpubkey_sha256"] = input.PreviousScriptPubKeySHA256
		}
		flows = append(flows, flow)
	}
	return flows, scriptTypes, len(addresses)
}

func bitcoinOutputFlowAttributes(outputs []networktarget.BitcoinTransactionOutputFlow) ([]map[string]any, map[string]int, int) {
	flows := make([]map[string]any, 0, len(outputs))
	scriptTypes := map[string]int{}
	addresses := map[string]struct{}{}
	for _, output := range outputs {
		flow := map[string]any{
			"index":               output.Index,
			"value_sats":          output.ValueSats,
			"script_type":         output.ScriptType,
			"scriptpubkey_sha256": output.ScriptPubKeySHA256,
		}
		scriptTypes[output.ScriptType]++
		if output.Address != "" {
			flow["address"] = output.Address
			addresses[output.Address] = struct{}{}
		}
		flows = append(flows, flow)
	}
	return flows, scriptTypes, len(addresses)
}

func AdaptBitcoinTransactionEvidence(result networktarget.BitcoinTransactionEvidenceResult, observedAt time.Time) (NetworkProbeIntelligenceProjection, error) {
	if observedAt.IsZero() {
		return NetworkProbeIntelligenceProjection{}, errors.New("observed time is required")
	}
	if result.Network != "bitcoin-mainnet" ||
		!result.AnalysisPerformed ||
		result.EvidenceStatus != IntelligenceEvidenceObserved ||
		result.LiveAvailability != "checked" {
		return NetworkProbeIntelligenceProjection{}, errors.New("completed observed Bitcoin transaction probe is required")
	}
	expectedGenesis, ok := networktarget.ExpectedBitcoinGenesisHash(result.Network)
	if !ok ||
		strings.ToLower(strings.TrimSpace(result.GenesisHash)) != expectedGenesis ||
		strings.ToLower(strings.TrimSpace(result.ExpectedGenesisHash)) != expectedGenesis {
		return NetworkProbeIntelligenceProjection{}, errors.New("Bitcoin transaction network identity is not verified")
	}
	txid := strings.ToLower(strings.TrimSpace(result.TransactionID))
	subject, err := ClassifyUniversalInvestigationSubject(txid, result.Network, IntelligenceSubjectTransaction)
	if err != nil || subject.ChainFamily != IntelligenceChainFamilyUTXO || subject.Chain != "bitcoin" {
		return NetworkProbeIntelligenceProjection{}, errors.New("Bitcoin transaction subject classification mismatch")
	}
	if result.InputCount <= 0 || result.OutputCount <= 0 || result.Size <= 0 || result.Weight <= 0 || result.FeeSats < 0 {
		return NetworkProbeIntelligenceProjection{}, errors.New("Bitcoin transaction evidence is incomplete")
	}
	if len(result.Inputs) != result.InputCount || len(result.Outputs) != result.OutputCount ||
		result.TotalInputSats < 0 || result.TotalOutputSats < 0 || result.DerivedFeeSats < 0 {
		return NetworkProbeIntelligenceProjection{}, errors.New("Bitcoin transaction flow evidence is incomplete")
	}
	if !result.Coinbase && (!result.FeeBalanceChecked || !result.FeeConsistent || result.DerivedFeeSats != result.FeeSats) {
		return NetworkProbeIntelligenceProjection{}, errors.New("Bitcoin transaction fee evidence is inconsistent")
	}
	state := "mempool_observed"
	if result.Confirmed {
		if result.BlockHeight <= 0 || strings.TrimSpace(result.BlockHash) == "" || result.BlockTimeUnix <= 0 {
			return NetworkProbeIntelligenceProjection{}, errors.New("confirmed Bitcoin transaction is missing block evidence")
		}
		state = "confirmed_observed"
	}
	observedAt = observedAt.UTC()
	inputFlows, inputScriptTypes, inputAddresses := bitcoinInputFlowAttributes(result.Inputs)
	outputFlows, outputScriptTypes, outputAddresses := bitcoinOutputFlowAttributes(result.Outputs)
	attributes := map[string]any{
		"genesis_hash":                expectedGenesis,
		"transaction_response_sha256": strings.ToLower(strings.TrimSpace(result.TransactionSHA256)),
		"version":                     result.Version,
		"locktime":                    result.Locktime,
		"input_count":                 result.InputCount,
		"output_count":                result.OutputCount,
		"size_bytes":                  result.Size,
		"weight":                      result.Weight,
		"fee_sats":                    result.FeeSats,
		"coinbase":                    result.Coinbase,
		"total_input_sats":            result.TotalInputSats,
		"total_output_sats":           result.TotalOutputSats,
		"derived_fee_sats":            result.DerivedFeeSats,
		"fee_balance_checked":         result.FeeBalanceChecked,
		"fee_consistent":              result.FeeConsistent,
		"input_flows":                 inputFlows,
		"output_flows":                outputFlows,
		"input_script_types":          inputScriptTypes,
		"output_script_types":         outputScriptTypes,
		"input_address_count":         inputAddresses,
		"output_address_count":        outputAddresses,
		"confirmed":                   result.Confirmed,
		"evidence_scope":              "read_only_bitcoin_transaction_probe",
		"flow_scope":                  "utxo_prevout_output_observation",
	}
	if result.Confirmed {
		attributes["block_hash"] = strings.ToLower(strings.TrimSpace(result.BlockHash))
		attributes["block_time_unix"] = result.BlockTimeUnix
	}
	identity := strings.Join([]string{"bitcoin-transaction-probe", subject.ID, txid, state, observedAt.Format(time.RFC3339Nano)}, ":")
	evidence := IntelligenceEvidence{
		ID:              intelligenceStableID(identity),
		SubjectID:       subject.ID,
		ChainFamily:     IntelligenceChainFamilyUTXO,
		Chain:           "bitcoin",
		Network:         result.Network,
		Source:          "bitcoin_esplora_probe",
		Status:          IntelligenceEvidenceObserved,
		TransactionHash: txid,
		BlockOrSlot:     result.BlockHeight,
		ObservedAt:      observedAt,
		Method:          "transaction_lookup",
		StateChange:     state,
		Provenance:      networkProbeEvidenceProvenance,
		Confidence:      0.8,
		Attributes:      attributes,
	}
	return NetworkProbeIntelligenceProjection{Subject: subject, Evidence: evidence}, nil
}
