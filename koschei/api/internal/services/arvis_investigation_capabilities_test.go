package services

import "testing"

func TestArvisInvestigationCapabilitiesExposeEvidenceFirstBuildout(t *testing.T) {
	capabilities := ArvisInvestigationCapabilities()
	if len(capabilities) == 0 {
		t.Fatal("expected investigation capability map")
	}
	seen := map[string]ArvisInvestigationCapability{}
	for _, capability := range capabilities {
		if capability.ID == "" {
			t.Fatalf("capability missing id: %#v", capability)
		}
		if capability.ActorRulesetVersion != ActorDefenseRulesetVersion {
			t.Fatalf("%s actor ruleset=%q", capability.ID, capability.ActorRulesetVersion)
		}
		if capability.UnifiedRulesetVersion != UnifiedRadarRulesetVersion {
			t.Fatalf("%s unified ruleset=%q", capability.ID, capability.UnifiedRulesetVersion)
		}
		if len(capability.CanonicalSections) == 0 {
			t.Fatalf("%s missing canonical section reference", capability.ID)
		}
		if capability.EvidencePolicy == "" {
			t.Fatalf("%s missing evidence policy", capability.ID)
		}
		if _, exists := seen[capability.ID]; exists {
			t.Fatalf("duplicate capability id %s", capability.ID)
		}
		seen[capability.ID] = capability
	}
	for _, required := range []string{
		"liquidity_drain_attribution",
		"transaction_intent",
		"mev_sandwich",
		"market_manipulation",
		"cross_chain_intelligence",
		"unverified_cross_chain_crime_patterns",
	} {
		if _, exists := seen[required]; !exists {
			t.Fatalf("missing capability %s", required)
		}
	}
	if seen["cross_chain_intelligence"].Status != ArvisCapabilitySchemaOnly {
		t.Fatalf("cross-chain capability must remain schema-only until verified ingestion exists: %#v", seen["cross_chain_intelligence"])
	}
	if seen["unverified_cross_chain_crime_patterns"].Status != ArvisCapabilityUnavailable {
		t.Fatalf("criminal-pattern claims must remain unavailable without verified rows: %#v", seen["unverified_cross_chain_crime_patterns"])
	}
}

func TestArvisBundleIncludesInvestigationCapabilityMap(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "")
	analysis := AnalyzeArvisRadars(SecurityRadarRequest{Target: "Mint11111111111111111111111111111111111111", Network: "solana-mainnet", Mode: "manual_test"})
	raw, ok := analysis.Bundle.Metadata["investigation_capabilities"].([]ArvisInvestigationCapability)
	if !ok {
		t.Fatalf("bundle missing typed capability map: %#v", analysis.Bundle.Metadata["investigation_capabilities"])
	}
	if len(raw) == 0 {
		t.Fatal("bundle capability map is empty")
	}
	if scope, _ := analysis.Bundle.Metadata["investigation_capability_scope"].(string); scope != ArvisCapabilityRulesetScope {
		t.Fatalf("unexpected capability scope %q", scope)
	}
	for _, capability := range raw {
		if capability.Status == ArvisCapabilityUnavailable && len(capability.PrimaryModules) > 0 {
			t.Fatalf("unavailable claim should not point to a production evidence arm: %#v", capability)
		}
	}
}
