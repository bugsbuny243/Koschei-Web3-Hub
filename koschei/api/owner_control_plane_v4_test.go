package main

import (
	"os"
	"strings"
	"testing"
)

func TestOwnerControlPlaneV4BackendContract(t *testing.T) {
	operations, err := os.ReadFile("internal/handlers/owner_operations.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(operations)
	for _, required := range []string{
		"koschei.owner-control-plane.v4",
		"\"professional_entitlements\"",
		"\"paid_plan\":          \"professional\"",
		"\"billing_provider\":   \"polar\"",
		"\"token_telemetry\":    \"audit_only_no_authority\"",
		"func (h *Handler) OwnerTokenTelemetry",
		"koschei.owner-token-telemetry.v1",
		"\"commercial_authority\": false",
		"\"snapshot_authority\": \"none_audit_only\"",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("owner control-plane backend missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"\"kosch_premium\"",
		"func (h *Handler) OwnerKOSCHAccess",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("owner control-plane backend still contains retired contract %q", forbidden)
		}
	}

	server, err := os.ReadFile("internal/http/server.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(server), "\"/api/owner/token-telemetry\"") {
		t.Fatal("owner token telemetry route is not registered")
	}
}
