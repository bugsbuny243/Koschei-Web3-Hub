package services

import (
	"errors"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

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
	state := "mempool_observed"
	if result.Confirmed {
		if result.BlockHeight <= 0 || strings.TrimSpace(result.BlockHash) == "" || result.BlockTimeUnix <= 0 {
			return NetworkProbeIntelligenceProjection{}, errors.New("confirmed Bitcoin transaction is missing block evidence")
		}
		state = "confirmed_observed"
	}
	observedAt = observedAt.UTC()
	attributes := map[string]any{
		"genesis_hash": expectedGenesis,
		"transaction_response_sha256": strings.ToLower(strings.TrimSpace(result.TransactionSHA256)),
		"version": result.Version,
		"locktime": result.Locktime,
		"input_count": result.InputCount,
		"output_count": result.OutputCount,
		"size_bytes": result.Size,
		"weight": result.Weight,
		"fee_sats": result.FeeSats,
		"confirmed": result.Confirmed,
		"evidence_scope": "read_only_bitcoin_transaction_probe",
	}
	if result.Confirmed {
		attributes["block_hash"] = strings.ToLower(strings.TrimSpace(result.BlockHash))
		attributes["block_time_unix"] = result.BlockTimeUnix
	}
	identity := strings.Join([]string{"bitcoin-transaction-probe", subject.ID, txid, state, observedAt.Format(time.RFC3339Nano)}, ":")
	evidence := IntelligenceEvidence{
		ID: intelligenceStableID(identity),
		SubjectID: subject.ID,
		ChainFamily: IntelligenceChainFamilyUTXO,
		Chain: "bitcoin",
		Network: result.Network,
		Source: "bitcoin_esplora_probe",
		Status: IntelligenceEvidenceObserved,
		TransactionHash: txid,
		BlockOrSlot: result.BlockHeight,
		ObservedAt: observedAt,
		Method: "transaction_lookup",
		StateChange: state,
		Provenance: networkProbeEvidenceProvenance,
		Confidence: 0.8,
		Attributes: attributes,
	}
	return NetworkProbeIntelligenceProjection{Subject: subject, Evidence: evidence}, nil
}
