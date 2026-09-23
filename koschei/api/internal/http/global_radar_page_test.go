package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/radarevent"
	"koschei/api/internal/radargraph"
	"koschei/api/internal/radarpath"
	"koschei/api/internal/securityevidence"
)

func TestGlobalRadarOperatorSurfaceUsesRealCoverageAndTruthBoundaries(t *testing.T) {
	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/fabric/radar", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, marker := range []string{
		"KOSCHEI GLOBAL",
		"CRYPTO RADAR",
		"No fabricated global totals",
		"No wallet geolocation",
		"Hypotheses cannot mint ARVIS evidence",
		"Live chain totals withheld",
		"Solana",
		"Ethereum",
		"Bitcoin",
		radarevent.SchemaVersionV1,
		radargraph.SchemaVersionV1,
		radarpath.SchemaVersionV1,
		securityevidence.SchemaVersionV1,
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("operator surface missing %q", marker)
		}
	}
	for _, fabricated := range []string{"248M", "12.4M", "3.1M", "320+"} {
		if strings.Contains(body, fabricated) {
			t.Fatalf("operator surface contains fabricated marketing metric %q", fabricated)
		}
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("operator surface must not be cached")
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("operator surface must preserve security headers")
	}
}

func TestGlobalRadarSnapshotPublishesPipelineSchemasAndHypothesisBoundary(t *testing.T) {
	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/fabric/radar/global", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var snapshot globalRadarSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.EventSchema != radarevent.SchemaVersionV1 ||
		snapshot.EntityGraphSchema != radargraph.SchemaVersionV1 ||
		snapshot.AttackPathSchema != radarpath.SchemaVersionV1 ||
		snapshot.SecurityEvidenceSchema != securityevidence.SchemaVersionV1 {
		t.Fatalf("unexpected pipeline schemas: %#v", snapshot)
	}
	if snapshot.TruthBoundary.HypothesisPromotionAllowed {
		t.Fatal("hypothesis promotion unexpectedly enabled")
	}
}

func TestGlobalRadarOperatorSurfaceRejectsMutationMethods(t *testing.T) {
	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodPost, "/fabric/radar", nil),
	)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
