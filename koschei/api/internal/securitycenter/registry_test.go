package securitycenter

import "testing"

func TestCurrentRegistryHasUniqueCapabilityAndAssetIDs(t *testing.T) {
	snapshot := Current()
	if !snapshot.PreserveExisting {
		t.Fatal("security center must preserve existing capabilities")
	}
	if snapshot.BreakingChanges {
		t.Fatal("security center must remain additive")
	}

	capabilities := map[string]bool{}
	for _, capability := range snapshot.Capabilities {
		if capability.ID == "" {
			t.Fatal("capability id is required")
		}
		if capabilities[capability.ID] {
			t.Fatalf("duplicate capability id %q", capability.ID)
		}
		capabilities[capability.ID] = true
	}

	assets := map[string]bool{}
	for _, binding := range snapshot.AssetBindings {
		if binding.Asset == "" || binding.State == "" {
			t.Fatalf("invalid asset binding: %#v", binding)
		}
		if assets[binding.Asset] {
			t.Fatalf("duplicate asset binding %q", binding.Asset)
		}
		assets[binding.Asset] = true
	}
}

func TestCurrentRegistryCoversEveryRegisteredNetwork(t *testing.T) {
	snapshot := Current()
	covered := map[string]bool{}
	for _, capability := range snapshot.Capabilities {
		for _, networkID := range capability.NetworkIDs {
			covered[networkID] = true
		}
	}
	for _, network := range snapshot.Networks {
		if !covered[network.ID] {
			t.Fatalf("registered network %q is not represented in the security-center graph", network.ID)
		}
	}
}
