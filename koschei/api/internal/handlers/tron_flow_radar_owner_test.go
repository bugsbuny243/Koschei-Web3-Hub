package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestOwnerTronFlowRadarReportsOffWithoutARunningRadar(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Handler{}).OwnerTronFlowRadar(rec, httptest.NewRequest(http.MethodGet, "/api/owner/radar/tron-flow", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status=%d err=%v", rec.Code, err)
	}
	if body["enabled"] != false || body["storage"] != "in_memory_only" || !strings.Contains(body["reason"].(string), "KOSCHEI_TRON_FLOW_RADAR_ENABLED=1") {
		t.Fatalf("unexpected off response %v", body)
	}
}

func TestOwnerProductionShellLoadsTronFlowRadarPanel(t *testing.T) {
	page, err := os.ReadFile("../../public/owner-production.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `<script src="/js/owner-tron-flow-radar-v1.js?v=2"></script>`) {
		t.Fatal("owner production shell must load the Tron Flow Radar panel")
	}
	script, err := os.ReadFile("../../public/js/owner-tron-flow-radar-v1.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"/api/owner/radar/tron-flow", "credentials:'same-origin'", "securityContent", "Koschei cannot reverse a transfer"} {
		if !strings.Contains(string(script), required) {
			t.Fatalf("panel script missing %q", required)
		}
	}
}

func TestOwnerTronFlowEvidencePackageNeedsARunningRadar(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Handler{}).OwnerTronFlowEvidencePackage(rec, httptest.NewRequest(http.MethodGet, "/api/owner/radar/tron-flow/evidence?finding=tron-flow:x", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "tron_flow_radar_not_running") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
