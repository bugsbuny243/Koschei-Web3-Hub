package http

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestKOSCHRouteTierAssignments(t *testing.T) {
	source := serverSource(t)
	for _, expected := range []string{
		`koschTier("basic", method("POST", h.SecurityRadarCheck))`,
		`koschTier("basic", method("GET", h.SecurityRadarDetailV3))`,
		`koschTier("pro", method("GET", h.OwnerActorSecurityIntelligence))`,
		`koschTier("pro", method("GET", h.OwnerCreatorIntelligence))`,
		`koschTier("pro", method("GET", h.SecurityRadarGraph))`,
		`koschTier("pro", method("GET", h.SecurityRadarExposureReport))`,
		`koschTierAccess("enterprise", h.APIKeysCollection)`,
		`apiKeyEnterpriseMetered(method("POST", h.B2BTokenScan))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("missing route policy: %s", expected)
		}
	}
}

func TestFreeCoreRoutesDoNotUseKOSCHTierOrQuota(t *testing.T) {
	source := serverSource(t)
	for _, expected := range []string{
		`mux.HandleFunc("/api/token/scan", method("POST", h.TokenScan))`,
		`mux.HandleFunc("/api/v1/risk/badge", method("GET", h.SecurityRiskBadge))`,
		`mux.HandleFunc("/api/arvis/preflight", method("POST", h.ARVISPreflight))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("free route changed or became gated: %s", expected)
		}
	}
	if strings.Contains(source, `koschTier("basic", method("POST", h.ARVISPreflight))`) {
		t.Fatal("free preflight must not consume KOSCH quota")
	}
}

func serverSource(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller path unavailable")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "server.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
