package networktarget

import "testing"

func TestGlobalRadarCapabilitiesCoverCatalogWithoutPromotingLiveHealth(t *testing.T) {
	catalog := Catalog()
	capabilities := GlobalRadarCapabilities()
	if len(capabilities) != len(catalog) {
		t.Fatalf("capability count=%d catalog=%d", len(capabilities), len(catalog))
	}

	seen := map[string]RadarCapability{}
	for _, capability := range capabilities {
		if capability.NetworkID == "" || capability.Family == "" {
			t.Fatalf("incomplete capability identity: %#v", capability)
		}
		if capability.EvidenceBoundary != "repository_capability_only_live_availability_not_checked" {
			t.Fatalf("%s fabricated live boundary: %q", capability.NetworkID, capability.EvidenceBoundary)
		}
		if _, exists := seen[capability.NetworkID]; exists {
			t.Fatalf("duplicate network capability %s", capability.NetworkID)
		}
		seen[capability.NetworkID] = capability
	}
	for _, network := range catalog {
		if _, ok := seen[network.ID]; !ok {
			t.Fatalf("network %s missing from global radar capability contract", network.ID)
		}
	}
}

func TestGlobalRadarCapabilitiesKeepVerdictAuthorityExplicit(t *testing.T) {
	seen := map[string]RadarCapability{}
	for _, capability := range GlobalRadarCapabilities() {
		seen[capability.NetworkID] = capability
	}

	if got := seen["solana-mainnet"].VerdictAuthority; got != "arvis_existing" {
		t.Fatalf("solana verdict authority=%q", got)
	}
	for _, id := range []string{
		"ethereum-mainnet",
		"base-mainnet",
		"arbitrum-mainnet",
		"optimism-mainnet",
		"polygon-mainnet",
		"bnb-mainnet",
		"avalanche-mainnet",
		"bitcoin-mainnet",
		"sui-mainnet",
		"aptos-mainnet",
	} {
		if got := seen[id].VerdictAuthority; got != "not_connected" {
			t.Fatalf("%s verdict authority was silently promoted: %q", id, got)
		}
	}
}

func TestGlobalRadarCapabilitiesKeepMoveDispatcherFailClosed(t *testing.T) {
	for _, capability := range GlobalRadarCapabilities() {
		if capability.Family != "move" {
			continue
		}
		if capability.IdentityResolution != "implemented" ||
			capability.TransactionObservation != "not_connected" ||
			capability.StateObservation != "not_connected" ||
			capability.ContractOrProgramAnalysis != "not_connected" ||
			capability.CrossChainCorrelation != "not_connected" {
			t.Fatalf("%s move capability was overclaimed: %#v", capability.NetworkID, capability)
		}
	}
}
