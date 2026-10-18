package services

import (
	"strings"
	"testing"
)

// These are synthetic signed fixtures, NOT live EVM/Bitcoin/Move verdicts.
// Signature/evidence verification is chain-neutral; grade authorization must
// still come from the separate native customer dispatch capability contract.
func TestCustomerCanonicalEvidenceBindingIsNetworkNeutral(t *testing.T) {
	for _, network := range []string{
		"solana-mainnet",
		"ethereum-mainnet", "base-mainnet", "arbitrum-mainnet",
		"polygon-mainnet", "bnb-mainnet", "avalanche-mainnet",
		"bitcoin-mainnet", "sui-mainnet", "aptos-mainnet",
	} {
		t.Run(network, func(t *testing.T) {
			event, source, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
			verdict := FinalizeUnifiedRadarVerdictContractForNetwork(
				event.Target, network, source)
			if !verdict.Signed {
				t.Fatal("synthetic network fixture signing failed")
			}
			err := validateCustomerCanonicalEventEvidenceBinding(
				event.Target, network, event.Signature, verdict, registry)
			if err != nil {
				t.Fatalf("valid signed/cited fixture rejected for %s: %v", network, err)
			}
			// A checked signature cannot be replayed as evidence for a different
			// network. The canonical signing payload binds target and network.
			if err := validateCustomerCanonicalEventEvidenceBinding(
				event.Target, "solana-devnet", event.Signature, verdict, registry); err == nil {
				t.Fatal("unregistered network unexpectedly accepted")
			}
		})
	}
}

func TestCustomerCanonicalEvidenceBindingRejectsMissingOrSpoofedCustomerContext(t *testing.T) {
	event, verdict, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
	for _, tc := range []struct {
		name, target, network, reference string
	}{
		{"missing target", "", event.Network, event.Signature},
		{"missing network", event.Target, "", event.Signature},
		{"missing reference", event.Target, event.Network, ""},
		{"unknown network", event.Target, "fakechain-mainnet", event.Signature},
		{"wrong token", "OtherTarget", event.Network, event.Signature},
		{"wrong reference", event.Target, event.Network, "other-tx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCustomerCanonicalEventEvidenceBinding(tc.target, tc.network, tc.reference, verdict, registry)
			if err == nil {
				t.Fatal("unsigned/unbound customer context unexpectedly accepted")
			}
			if tc.name == "wrong reference" && !strings.Contains(err.Error(), "not cited") {
				t.Fatalf("uncited transaction should fail: %v", err)
			}
		})
	}
}
