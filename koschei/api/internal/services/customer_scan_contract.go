package services

import (
	"errors"
	"regexp"
	"strings"
)

const (
	CustomerScanTargetEVMAddress   = "evm_address"
	CustomerScanTargetSolana       = "solana_address"
	CustomerScanTargetBitcoin      = "bitcoin_address"
	CustomerScanTargetTxHash       = "transaction_hash"
	CustomerScanTargetUnknown      = "unknown"
	CustomerScanRouteEVMProbe      = "evm_probe"
	CustomerScanRouteSolanaIntel   = "solana_intelligence"
	CustomerScanRouteBitcoinProbe  = "bitcoin_probe"
	CustomerScanRouteTxLookup      = "transaction_lookup"
	CustomerScanRouteUnresolved    = "unresolved"
)

var evmTxHashPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

type CustomerScanTarget struct {
	Raw              string   `json:"raw"`
	NetworkHint      string   `json:"network_hint,omitempty"`
	Kind             string   `json:"kind"`
	Route            string   `json:"route"`
	Classification   string   `json:"classification"`
	RequiresNetwork  bool     `json:"requires_network"`
	Reasons          []string `json:"reasons,omitempty"`
}

// ClassifyCustomerScanTarget decides only which existing analysis family may
// handle a customer supplied target. It deliberately does not claim that a
// syntactically valid target exists, is owned by anyone, is safe, or is live.
func ClassifyCustomerScanTarget(raw, networkHint string) (CustomerScanTarget, error) {
	raw = strings.TrimSpace(raw)
	networkHint = strings.ToLower(strings.TrimSpace(networkHint))
	if raw == "" {
		return CustomerScanTarget{}, errors.New("scan target is required")
	}

	out := CustomerScanTarget{
		Raw:            raw,
		NetworkHint:    networkHint,
		Kind:           CustomerScanTargetUnknown,
		Route:          CustomerScanRouteUnresolved,
		Classification: "syntax_only",
	}

	if evmAddressPattern.MatchString(raw) {
		out.Kind = CustomerScanTargetEVMAddress
		out.Route = CustomerScanRouteEVMProbe
		out.RequiresNetwork = networkHint == ""
		if out.RequiresNetwork {
			out.Reasons = []string{"NETWORK_REQUIRED_FOR_EVM_ADDRESS"}
		}
		return out, nil
	}

	if evmTxHashPattern.MatchString(raw) {
		out.Kind = CustomerScanTargetTxHash
		out.Route = CustomerScanRouteTxLookup
		out.RequiresNetwork = networkHint == ""
		if out.RequiresNetwork {
			out.Reasons = []string{"NETWORK_REQUIRED_FOR_TRANSACTION_HASH"}
		}
		return out, nil
	}

	// Reuse the chain-neutral classifier for Solana syntax only. Classification
	// remains non-evidentiary until a live adapter contributes observed evidence.
	subject := ClassifyIntelligenceSubject(raw, networkHint)
	if subject.ChainFamily == IntelligenceChainFamilySolana {
		out.Kind = CustomerScanTargetSolana
		out.Route = CustomerScanRouteSolanaIntel
		return out, nil
	}
	if subject.ChainFamily == IntelligenceChainFamilyUTXO && subject.Chain == "bitcoin" {
		out.Kind = CustomerScanTargetBitcoin
		out.Route = CustomerScanRouteBitcoinProbe
		return out, nil
	}

	out.Reasons = []string{"TARGET_TYPE_UNRESOLVED"}
	return out, nil
}
