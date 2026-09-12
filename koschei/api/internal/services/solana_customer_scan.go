package services

import (
	"errors"
	"fmt"
	"strings"
)

// SolanaAccountObservation is a bounded read-only observation from
// getAccountInfo. It records chain state only and does not prove ownership,
// authorization, safety, or finality.
type SolanaAccountObservation struct {
	Address    string `json:"address"`
	Network    string `json:"network"`
	Slot       uint64 `json:"slot"`
	Present    bool   `json:"present"`
	Executable bool   `json:"executable"`
	Lamports   uint64 `json:"lamports"`
	Owner      string `json:"owner,omitempty"`
	Space      uint64 `json:"space,omitempty"`
}

func CustomerScanResultFromSolanaObservation(target CustomerScanTarget, observation SolanaAccountObservation) (CustomerScanResult, error) {
	if target.Route != CustomerScanRouteSolanaIntel {
		return CustomerScanResult{}, errors.New("Solana observation requires Solana scan route")
	}
	if target.RequiresNetwork {
		return CustomerScanResult{}, errors.New("network context is required before Solana observation")
	}
	if strings.TrimSpace(observation.Address) != strings.TrimSpace(target.Raw) {
		return CustomerScanResult{}, errors.New("Solana observation address does not match customer target")
	}
	if strings.ToLower(strings.TrimSpace(observation.Network)) != "solana-mainnet" {
		return CustomerScanResult{}, errors.New("unsupported Solana observation network")
	}
	if observation.Slot == 0 {
		return CustomerScanResult{}, errors.New("Solana observation slot is required")
	}
	if observation.Present && strings.TrimSpace(observation.Owner) == "" {
		return CustomerScanResult{}, errors.New("observed Solana account owner program is required")
	}
	if !observation.Present && (observation.Executable || observation.Lamports != 0 || observation.Owner != "" || observation.Space != 0) {
		return CustomerScanResult{}, errors.New("absent Solana account cannot carry account state")
	}

	reasons := []string{"READ_ONLY_SOLANA_ACCOUNT_OBSERVATION"}
	state := "present"
	if !observation.Present {
		state = "absent"
		reasons = append(reasons, "SOLANA_ACCOUNT_NOT_OBSERVED")
	}
	if observation.Executable {
		reasons = append(reasons, "SOLANA_EXECUTABLE_ACCOUNT_OBSERVED")
	}
	trust := Web3TrustVector{Observed: true, Reasons: reasons}
	evidenceRef := fmt.Sprintf("solana-account:%s:%d:%s", strings.TrimSpace(observation.Address), observation.Slot, state)
	return BuildCustomerScanResult(target, trust, []string{evidenceRef})
}
