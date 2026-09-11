package http

import "testing"

func TestNetworkDeploymentCatalogReportsBitcoinParserReady(t *testing.T) {
	for _, state := range networkDeploymentCatalog() {
		if state.NetworkID != "bitcoin-mainnet" {
			continue
		}
		if state.CollectorRuntime != "parser_ready_collector_not_connected" {
			t.Fatalf("bitcoin collector_runtime=%q", state.CollectorRuntime)
		}
		if state.LiveAvailability != "collector_not_connected" {
			t.Fatalf("bitcoin live_availability=%q", state.LiveAvailability)
		}
		return
	}
	t.Fatal("bitcoin-mainnet missing from deployment catalog")
}
