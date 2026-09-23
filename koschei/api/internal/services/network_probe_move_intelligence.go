package services

import (
	"errors"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

// AdaptSuiIdentityProbeEvidence projects a verified Sui mainnet identity probe
// into the chain-neutral intelligence model. The probe verifies selected
// network identity only; it does not prove account existence, ownership,
// balances, objects/resources or safety.
func AdaptSuiIdentityProbeEvidence(
	resolution networktarget.Resolution,
	result networktarget.SuiMainnetIdentityProbeResult,
	observedAt time.Time,
) (NetworkProbeIntelligenceProjection, error) {
	if observedAt.IsZero() {
		return NetworkProbeIntelligenceProjection{}, errors.New("observed time is required")
	}
	if resolution.Network.ID != "sui-mainnet" ||
		resolution.Network.Family != IntelligenceChainFamilyMove ||
		!resolution.SyntaxValid {
		return NetworkProbeIntelligenceProjection{}, errors.New("canonical Sui resolution is required")
	}
	if !result.AnalysisPerformed ||
		result.LiveAvailability != "checked" ||
		result.Observation.Network.ID != "sui-mainnet" ||
		result.Observation.EvidenceStatus != IntelligenceEvidenceVerified ||
		strings.TrimSpace(result.ChainIdentifier) != networktarget.SuiMainnetChainIdentifier {
		return NetworkProbeIntelligenceProjection{}, errors.New("verified Sui mainnet identity probe is required")
	}

	subject := ClassifyIntelligenceSubject(resolution.Address, resolution.Network.ID)
	if subject.ChainFamily != IntelligenceChainFamilyMove || subject.Chain != "sui" {
		return NetworkProbeIntelligenceProjection{}, errors.New("Sui subject classification mismatch")
	}

	attributes := map[string]any{
		"chain_identifier":       result.ChainIdentifier,
		"evidence_scope":         "network_identity_only",
		"address_state_observed": false,
		"analysis_performed":     true,
		"live_availability":      "checked",
	}
	evidence := buildNetworkProbeEvidence(
		subject,
		"sui_graphql_identity_probe",
		"chainIdentifier",
		"network_identity_verified",
		observedAt,
		attributes,
	)
	evidence.Status = IntelligenceEvidenceVerified
	evidence.Confidence = 1

	return NetworkProbeIntelligenceProjection{Subject: subject, Evidence: evidence}, nil
}

// AdaptAptosIdentityProbeEvidence projects a verified Aptos mainnet ledger
// identity probe into the chain-neutral intelligence model. Ledger freshness
// is observed network evidence only; it does not prove address existence,
// ownership, balances/resources or safety.
func AdaptAptosIdentityProbeEvidence(
	resolution networktarget.Resolution,
	result networktarget.AptosMainnetIdentityProbeResult,
	observedAt time.Time,
) (NetworkProbeIntelligenceProjection, error) {
	if observedAt.IsZero() {
		return NetworkProbeIntelligenceProjection{}, errors.New("observed time is required")
	}
	if resolution.Network.ID != "aptos-mainnet" ||
		resolution.Network.Family != IntelligenceChainFamilyMove ||
		!resolution.SyntaxValid {
		return NetworkProbeIntelligenceProjection{}, errors.New("canonical Aptos resolution is required")
	}
	if !result.AnalysisPerformed ||
		result.LiveAvailability != "checked" ||
		result.Observation.Network.ID != "aptos-mainnet" ||
		result.Observation.EvidenceStatus != IntelligenceEvidenceVerified ||
		result.ChainID != networktarget.AptosMainnetChainID {
		return NetworkProbeIntelligenceProjection{}, errors.New("verified Aptos mainnet identity probe is required")
	}

	subject := ClassifyIntelligenceSubject(resolution.Address, resolution.Network.ID)
	if subject.ChainFamily != IntelligenceChainFamilyMove || subject.Chain != "aptos" {
		return NetworkProbeIntelligenceProjection{}, errors.New("Aptos subject classification mismatch")
	}

	attributes := map[string]any{
		"chain_id":               result.ChainID,
		"epoch":                  result.Epoch,
		"ledger_version":         result.LedgerVersion,
		"ledger_timestamp":       result.LedgerTimestamp,
		"block_height":           result.BlockHeight,
		"node_role":              result.NodeRole,
		"evidence_scope":         "network_identity_and_ledger_freshness_only",
		"address_state_observed": false,
		"analysis_performed":     true,
		"live_availability":      "checked",
	}
	if strings.TrimSpace(result.GitHash) != "" {
		attributes["git_hash"] = strings.TrimSpace(result.GitHash)
	}

	evidence := buildNetworkProbeEvidence(
		subject,
		"aptos_rest_identity_probe",
		"ledger_info",
		"network_identity_verified",
		observedAt,
		attributes,
	)
	evidence.Status = IntelligenceEvidenceVerified
	evidence.Confidence = 1

	return NetworkProbeIntelligenceProjection{Subject: subject, Evidence: evidence}, nil
}
