package services

import (
	"errors"
	"fmt"
	"strings"

	"koschei/api/internal/networktarget"
)

// validateCustomerCanonicalEventEvidenceBinding is network-neutral:
// adapters supply a token/contract/wallet target, chain identifier and
// transaction reference. It authenticates one signed deterministic decision
// and checks that VERIFIED rule evidence cites that transaction.
//
// This is *not* authorization to publish a grade on every network. The
// customer-facing dispatcher must separately require a connected native
// evidence adapter and explicit verdict authority for that network.
func validateCustomerCanonicalEventEvidenceBinding(
	target, network, transactionReference string,
	verdict UnifiedRadarVerdict,
	registry GlobalRadarTrustedVerdictRegistry,
) error {
	target = strings.TrimSpace(target)
	network = strings.TrimSpace(network)
	transactionReference = strings.TrimSpace(transactionReference)
	if target == "" || network == "" || transactionReference == "" {
		return errors.New("canonical decision needs target, network and transaction reference")
	}
	if _, ok := networktarget.LookupNetwork(network); !ok {
		return errors.New("canonical decision network is not in registered network catalog")
	}
	if strings.TrimSpace(verdict.Target) != target || strings.TrimSpace(verdict.Network) != network {
		return errors.New("canonical decision target/network mismatch")
	}
	if err := VerifyUnifiedRadarVerdictWithTrustedRegistry(verdict, registry); err != nil {
		return fmt.Errorf("canonical decision signature authentication failed: %w", err)
	}
	for _, hit := range verdict.TriggeredRules {
		if strings.ToLower(strings.TrimSpace(hit.EvidenceStatus)) != "verified" {
			continue
		}
		for _, cited := range hit.Signatures {
			if strings.TrimSpace(cited) == transactionReference {
				return nil
			}
		}
		for _, cited := range hit.EvidenceKeys {
			if strings.TrimSpace(cited) == "event-signature:"+transactionReference {
				return nil
			}
		}
	}
	return errors.New("transaction is not cited by a verified rule in the signed canonical decision")
}

// validateArvisStreamCanonicalVerdict applies the existing Solana-only
// Pump/Raydium *collector* policy atop the generic customer evidence gate.
// It must not be reused as the cross-chain customer product's access policy.
// Stream final persistence remains independently disabled/fail-closed.
func validateArvisStreamCanonicalVerdict(
	event arvisStreamTarget,
	verdict UnifiedRadarVerdict,
	registry GlobalRadarTrustedVerdictRegistry,
) error {
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Signature) == "" ||
		strings.TrimSpace(event.Target) == "" {
		return errors.New("ARVIS legacy stream event requires id, target and transaction signature")
	}
	if event.Network != "solana-mainnet" {
		return errors.New("legacy Pump/Raydium collector accepts Solana mainnet only; use chain-native customer adapter for other networks")
	}
	if event.ModuleID != ModulePumpSybilRadar && event.ModuleID != ModuleRaydiumPoolGuardian {
		return errors.New("ARVIS legacy stream source module is not connected")
	}
	if verdict.RulesetVersion != UnifiedRadarRulesetVersionV140 {
		return fmt.Errorf("ARVIS stream decision requires %s", UnifiedRadarRulesetVersionV140)
	}
	return validateCustomerCanonicalEventEvidenceBinding(
		event.Target, event.Network, event.Signature, verdict, registry)
}
