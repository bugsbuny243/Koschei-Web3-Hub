package services

import (
	"errors"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

func AdaptEVMAssetEvidence(result networktarget.EVMAssetProbeResult, observedAt time.Time) (NetworkProbeIntelligenceProjection, error) {
	if observedAt.IsZero() {
		return NetworkProbeIntelligenceProjection{}, errors.New("observed time is required")
	}
	expectedChainID, ok := networktarget.ExpectedEVMChainID(result.Network)
	if !ok || strings.ToLower(strings.TrimSpace(result.ChainID)) != expectedChainID ||
		strings.ToLower(strings.TrimSpace(result.ExpectedChainID)) != expectedChainID ||
		result.InterfaceState != "erc20_like_surface_observed" ||
		result.EvidenceStatus != IntelligenceEvidenceObserved || result.LiveAvailability != "checked" {
		return NetworkProbeIntelligenceProjection{}, errors.New("completed observed EVM asset probe is required")
	}
	subject := ClassifyIntelligenceSubject(result.Contract, result.Network)
	if subject.ChainFamily != IntelligenceChainFamilyEVM {
		return NetworkProbeIntelligenceProjection{}, errors.New("EVM asset subject classification mismatch")
	}
	if strings.TrimSpace(result.TotalSupply) == "" || strings.TrimSpace(result.ContractBalance) == "" ||
		len(strings.TrimSpace(result.TotalSupplyResponseSHA256)) != 64 ||
		len(strings.TrimSpace(result.ContractBalanceResponseSHA256)) != 64 {
		return NetworkProbeIntelligenceProjection{}, errors.New("EVM asset core evidence is incomplete")
	}
	attributes := map[string]any{
		"chain_id":                         expectedChainID,
		"interface_state":                  result.InterfaceState,
		"total_supply":                     result.TotalSupply,
		"contract_balance":                 result.ContractBalance,
		"total_supply_response_sha256":     strings.ToLower(result.TotalSupplyResponseSHA256),
		"contract_balance_response_sha256": strings.ToLower(result.ContractBalanceResponseSHA256),
		"evidence_scope":                   "read_only_erc20_like_surface_probe",
		"standards_compliance_claimed":     false,
		"safety_claimed":                   false,
	}
	if result.DecimalsObserved {
		attributes["decimals"] = result.Decimals
		attributes["decimals_observed"] = true
	}
	if result.SymbolObserved {
		attributes["symbol"] = result.Symbol
		attributes["symbol_encoding"] = result.SymbolEncoding
		attributes["symbol_observed"] = true
	}
	if result.NameObserved {
		attributes["name"] = result.Name
		attributes["name_encoding"] = result.NameEncoding
		attributes["name_observed"] = true
	}
	evidence := buildNetworkProbeEvidence(subject, "evm_erc20_asset_probe", "eth_call", result.InterfaceState, observedAt, attributes)
	return NetworkProbeIntelligenceProjection{Subject: subject, Evidence: evidence}, nil
}
