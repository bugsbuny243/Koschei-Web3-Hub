package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/radarevent"
)

func TestGlobalRadarSnapshotExposesTruthfulMultiNetworkControlPlane(t *testing.T) {
	t.Setenv("ETHEREUM_RPC_URL", "")
	t.Setenv("BITCOIN_ESPLORA_URL", "")

	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/fabric/radar/global", nil),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("global radar snapshot must not be cached")
	}

	var snapshot globalRadarSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != globalRadarSchemaVersion {
		t.Fatalf("schema=%q", snapshot.SchemaVersion)
	}
	if snapshot.EventSchema != radarevent.SchemaVersionV1 {
		t.Fatalf("event schema=%q", snapshot.EventSchema)
	}
	if snapshot.Scope != "multi-network-observation-control-plane" {
		t.Fatalf("scope=%q", snapshot.Scope)
	}
	if snapshot.Summary.RegisteredNetworks != len(networktarget.Catalog()) {
		t.Fatalf("registered=%d want=%d", snapshot.Summary.RegisteredNetworks, len(networktarget.Catalog()))
	}
	if snapshot.TruthBoundary.LiveChainMetricsIncluded {
		t.Fatal("phase 1 must not fabricate aggregate live chain metrics")
	}
	if snapshot.TruthBoundary.WalletGeolocationIncluded {
		t.Fatal("wallet geography must not be inferred")
	}
	if !snapshot.TruthBoundary.InfrastructureGeographyOnly || !snapshot.TruthBoundary.CapabilityIsNotLiveEvidence {
		t.Fatal("truth boundary is incomplete")
	}
	if snapshot.TruthBoundary.MissingEvidencePolicy != "unknown" {
		t.Fatalf("missing evidence policy=%q", snapshot.TruthBoundary.MissingEvidencePolicy)
	}

	seen := map[string]globalRadarNetworkState{}
	for _, state := range snapshot.Networks {
		seen[state.Network.ID] = state
	}
	for _, networkID := range []string{
		"solana-mainnet",
		"ethereum-mainnet",
		"bitcoin-mainnet",
		"sui-mainnet",
		"aptos-mainnet",
	} {
		if _, ok := seen[networkID]; !ok {
			t.Fatalf("global radar missing %s", networkID)
		}
	}
	if seen["ethereum-mainnet"].CollectorRuntime != "configuration_required" {
		t.Fatalf("ethereum runtime=%q", seen["ethereum-mainnet"].CollectorRuntime)
	}
	if seen["bitcoin-mainnet"].CollectorRuntime != "configuration_required" {
		t.Fatalf("bitcoin runtime=%q", seen["bitcoin-mainnet"].CollectorRuntime)
	}
}

func TestGlobalRadarSnapshotRejectsMutationMethods(t *testing.T) {
	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodPost, "/fabric/radar/global", nil),
	)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
