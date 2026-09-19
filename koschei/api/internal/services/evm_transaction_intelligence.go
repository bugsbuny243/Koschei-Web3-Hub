package services

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

func AdaptEVMTransactionEvidence(result networktarget.EVMTransactionEvidenceResult, observedAt time.Time) (NetworkProbeIntelligenceProjection, error) {
	if observedAt.IsZero() {
		return NetworkProbeIntelligenceProjection{}, errors.New("observed time is required")
	}
	if !result.AnalysisPerformed || result.EvidenceStatus != IntelligenceEvidenceObserved || result.LiveAvailability != "checked" {
		return NetworkProbeIntelligenceProjection{}, errors.New("completed observed EVM transaction probe is required")
	}
	expectedChainID, ok := networktarget.ExpectedEVMChainID(result.Network)
	if !ok || strings.ToLower(strings.TrimSpace(result.ChainID)) != expectedChainID || strings.ToLower(strings.TrimSpace(result.ExpectedChainID)) != expectedChainID {
		return NetworkProbeIntelligenceProjection{}, errors.New("EVM transaction chain identity is not verified")
	}
	subject, err := ClassifyUniversalInvestigationSubject(result.TransactionHash, result.Network, IntelligenceSubjectTransaction)
	if err != nil || subject.ChainFamily != IntelligenceChainFamilyEVM {
		return NetworkProbeIntelligenceProjection{}, errors.New("EVM transaction subject classification mismatch")
	}
	if result.ExecutionState != networktarget.EVMTransactionExecutionUnknown &&
		result.ExecutionState != networktarget.EVMTransactionExecutionPending &&
		result.ExecutionState != networktarget.EVMTransactionExecutionSuccess &&
		result.ExecutionState != networktarget.EVMTransactionExecutionReverted {
		return NetworkProbeIntelligenceProjection{}, errors.New("unsupported EVM transaction execution state")
	}

	attrs := map[string]any{
		"chain_id":            result.ChainID,
		"expected_chain_id":   result.ExpectedChainID,
		"from":                result.From,
		"to":                  result.To,
		"value":               result.Value,
		"nonce":               result.Nonce,
		"gas":                 result.Gas,
		"gas_price":           result.GasPrice,
		"transaction_type":    result.TransactionType,
		"input_sha256":        result.InputSHA256,
		"input_bytes":         result.InputBytes,
		"block_hash":          result.BlockHash,
		"block_number":        result.BlockNumber,
		"execution_state":     result.ExecutionState,
		"receipt_status":      result.ReceiptStatus,
		"receipt_root":        result.ReceiptRoot,
		"gas_used":            result.GasUsed,
		"cumulative_gas_used": result.CumulativeGasUsed,
		"effective_gas_price": result.EffectiveGasPrice,
		"contract_address":    result.ContractAddress,
		"log_count":           len(result.Logs),
		"evidence_scope":      "read_only_transaction_receipt_probe",
		"analysis_performed":  true,
		"live_availability":   "checked",
	}
	if len(result.Logs) > 0 {
		logs := make([]map[string]any, 0, len(result.Logs))
		for _, item := range result.Logs {
			logs = append(logs, map[string]any{
				"address":   item.Address,
				"topics":    append([]string(nil), item.Topics...),
				"log_index": item.LogIndex,
				"removed":   item.Removed,
			})
		}
		attrs["logs"] = logs
	}

	var block int64
	if result.BlockNumber != "" {
		value := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(result.BlockNumber)), "0x")
		parsed, parseErr := strconv.ParseUint(value, 16, 63)
		if parseErr != nil {
			return NetworkProbeIntelligenceProjection{}, errors.New("EVM transaction block number is invalid")
		}
		block = int64(parsed)
	}

	evidence := IntelligenceEvidence{
		ID:              intelligenceStableID(strings.Join([]string{"evm-transaction", subject.ID, result.TransactionHash, observedAt.UTC().Format(time.RFC3339Nano)}, ":")),
		SubjectID:       subject.ID,
		ChainFamily:     subject.ChainFamily,
		Chain:           subject.Chain,
		Network:         subject.Network,
		Source:          "ethereum_jsonrpc",
		Status:          IntelligenceEvidenceObserved,
		TransactionHash: result.TransactionHash,
		BlockOrSlot:     block,
		ObservedAt:      observedAt.UTC(),
		Address:         result.From,
		Contract:        result.To,
		Method:          "eth_getTransactionByHash+eth_getTransactionReceipt",
		StateChange:     result.ExecutionState,
		Provenance:      networkProbeEvidenceProvenance,
		Confidence:      0.9,
		Attributes:      attrs,
	}
	return NetworkProbeIntelligenceProjection{Subject: subject, Evidence: evidence}, nil
}
