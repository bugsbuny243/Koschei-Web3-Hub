package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

const networkProbeEvidenceProvenance = "networktarget_read_only_probe"

type NetworkProbeIntelligenceProjection struct {
	Subject  IntelligenceSubject  `json:"subject"`
	Evidence IntelligenceEvidence `json:"evidence"`
}

// AdaptEVMProbeEvidence projects a completed read-only EVM probe into the
// chain-neutral intelligence model. Observed bytecode state is evidence only;
// this adapter does not create a safety, ownership or authorization decision.
func AdaptEVMProbeEvidence(result networktarget.EVMProbeResult, observedAt time.Time) (NetworkProbeIntelligenceProjection, error) {
	if observedAt.IsZero() {
		return NetworkProbeIntelligenceProjection{}, errors.New("observed time is required")
	}
	resolution := result.Resolution
	if resolution.Network.Family != IntelligenceChainFamilyEVM || !resolution.SyntaxValid || !result.AnalysisPerformed || result.EvidenceStatus != IntelligenceEvidenceObserved || result.LiveAvailability != "checked" {
		return NetworkProbeIntelligenceProjection{}, errors.New("completed observed EVM probe is required")
	}
	expectedChainID, ok := networktarget.ExpectedEVMChainID(resolution.Network.ID)
	if !ok || strings.ToLower(strings.TrimSpace(result.ExpectedChainID)) != expectedChainID || strings.ToLower(strings.TrimSpace(result.ChainID)) != expectedChainID {
		return NetworkProbeIntelligenceProjection{}, errors.New("EVM probe chain identity is not verified")
	}
	if result.ContractCodeState != "contract_code_observed" && result.ContractCodeState != "no_contract_code_observed" {
		return NetworkProbeIntelligenceProjection{}, errors.New("unsupported EVM observation state")
	}

	subject := ClassifyIntelligenceSubject(resolution.Address, resolution.Network.ID)
	if subject.ChainFamily != IntelligenceChainFamilyEVM {
		return NetworkProbeIntelligenceProjection{}, errors.New("EVM subject classification mismatch")
	}
	attributes := map[string]any{
		"chain_id":             strings.ToLower(strings.TrimSpace(result.ChainID)),
		"expected_chain_id":    expectedChainID,
		"contract_code_state":  result.ContractCodeState,
		"evidence_scope":       "read_only_network_probe",
		"analysis_performed":   true,
		"live_availability":    "checked",
	}
	if hash := strings.ToLower(strings.TrimSpace(result.ContractCodeHash)); hash != "" {
		attributes["contract_code_sha256"] = hash
	}
	evidence := buildNetworkProbeEvidence(subject, "evm_rpc_probe", "eth_getCode", result.ContractCodeState, observedAt, attributes)
	return NetworkProbeIntelligenceProjection{Subject: subject, Evidence: evidence}, nil
}

// AdaptBitcoinProbeEvidence projects a completed read-only Bitcoin mainnet
// activity observation into the same intelligence model. Activity or inactivity
// remains observed evidence and never implies safety, ownership or nonexistence.
func AdaptBitcoinProbeEvidence(result networktarget.BitcoinProbeResult, observedAt time.Time) (NetworkProbeIntelligenceProjection, error) {
	if observedAt.IsZero() {
		return NetworkProbeIntelligenceProjection{}, errors.New("observed time is required")
	}
	resolution := result.Resolution
	if resolution.Network.ID != "bitcoin-mainnet" || resolution.Network.Family != IntelligenceChainFamilyUTXO || !resolution.SyntaxValid || !result.AnalysisPerformed || result.EvidenceStatus != IntelligenceEvidenceObserved || result.LiveAvailability != "checked" {
		return NetworkProbeIntelligenceProjection{}, errors.New("completed observed Bitcoin probe is required")
	}
	expectedGenesis, ok := networktarget.ExpectedBitcoinGenesisHash(resolution.Network.ID)
	if !ok || strings.ToLower(strings.TrimSpace(result.ExpectedGenesisHash)) != expectedGenesis || strings.ToLower(strings.TrimSpace(result.GenesisHash)) != expectedGenesis {
		return NetworkProbeIntelligenceProjection{}, errors.New("Bitcoin probe network identity is not verified")
	}
	if result.ActivityState != "activity_observed" && result.ActivityState != "no_activity_observed" {
		return NetworkProbeIntelligenceProjection{}, errors.New("unsupported Bitcoin observation state")
	}
	if result.ConfirmedTXCount < 0 || result.MempoolTXCount < 0 || result.FundedSats < 0 || result.SpentSats < 0 {
		return NetworkProbeIntelligenceProjection{}, errors.New("invalid Bitcoin observation counters")
	}

	subject := ClassifyIntelligenceSubject(resolution.Address, resolution.Network.ID)
	if subject.ChainFamily != IntelligenceChainFamilyUTXO || subject.Chain != "bitcoin" {
		return NetworkProbeIntelligenceProjection{}, errors.New("Bitcoin subject classification mismatch")
	}
	attributes := map[string]any{
		"genesis_hash":          expectedGenesis,
		"activity_state":        result.ActivityState,
		"confirmed_tx_count":    result.ConfirmedTXCount,
		"mempool_tx_count":      result.MempoolTXCount,
		"funded_sats":           result.FundedSats,
		"spent_sats":            result.SpentSats,
		"evidence_scope":        "read_only_network_probe",
		"analysis_performed":    true,
		"live_availability":     "checked",
	}
	evidence := buildNetworkProbeEvidence(subject, "bitcoin_esplora_probe", "address_activity", result.ActivityState, observedAt, attributes)
	return NetworkProbeIntelligenceProjection{Subject: subject, Evidence: evidence}, nil
}

func buildNetworkProbeEvidence(subject IntelligenceSubject, source, method, state string, observedAt time.Time, attributes map[string]any) IntelligenceEvidence {
	observedAt = observedAt.UTC()
	identity := strings.Join([]string{
		"network-probe",
		subject.ID,
		strings.TrimSpace(source),
		strings.TrimSpace(state),
		observedAt.Format(time.RFC3339Nano),
	}, ":")
	return IntelligenceEvidence{
		ID:          intelligenceStableID(identity),
		SubjectID:   subject.ID,
		ChainFamily: subject.ChainFamily,
		Chain:       subject.Chain,
		Network:     subject.Network,
		Source:      strings.TrimSpace(source),
		Status:      IntelligenceEvidenceObserved,
		ObservedAt:  observedAt,
		Address:     subject.Raw,
		Method:      strings.TrimSpace(method),
		StateChange: strings.TrimSpace(state),
		Provenance:  networkProbeEvidenceProvenance,
		Confidence:  0.8,
		Attributes:  attributes,
	}
}

func networkProbeProjectionSummary(projection NetworkProbeIntelligenceProjection) string {
	return fmt.Sprintf("%s:%s:%s", projection.Subject.ChainFamily, projection.Evidence.Source, projection.Evidence.Status)
}
