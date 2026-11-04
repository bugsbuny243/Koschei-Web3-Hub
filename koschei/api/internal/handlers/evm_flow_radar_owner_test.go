package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestOwnerEVMFlowRadarReportsOffWithoutARunningRadar(t *testing.T) {
	t.Setenv("KOSCHEI_EVM_FLOW_RADAR_ENABLED", "")
	rec := httptest.NewRecorder()
	(&Handler{}).OwnerEVMFlowRadar(rec, httptest.NewRequest(http.MethodGet, "/api/owner/radar/evm-flow", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status=%d err=%v", rec.Code, err)
	}
	if body["enabled"] != false || body["storage"] != "in_memory_only" || !strings.Contains(body["reason"].(string), "KOSCHEI_EVM_FLOW_RADAR_ENABLED=1") {
		t.Fatalf("unexpected off response %v", body)
	}

	t.Setenv("KOSCHEI_EVM_FLOW_RADAR_ENABLED", "true")
	rec = httptest.NewRecorder()
	(&Handler{}).OwnerEVMFlowRadar(rec, httptest.NewRequest(http.MethodGet, "/api/owner/radar/evm-flow", nil))
	if !strings.Contains(rec.Body.String(), "enabled but no network started") {
		t.Fatalf("an enabled radar with no running network must say so: %s", rec.Body.String())
	}
}

func TestOwnerEVMFlowEvidencePackageNeedsARunningRadar(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Handler{}).OwnerEVMFlowEvidencePackage(rec, httptest.NewRequest(http.MethodGet, "/api/owner/radar/evm-flow/evidence?finding=evm-flow:x", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "evm_flow_radar_not_running") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOwnerProductionShellLoadsEVMFlowRadarPanel(t *testing.T) {
	page, err := os.ReadFile("../../public/owner-production.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `<script src="/js/owner-evm-flow-radar-v1.js?v=1"></script>`) {
		t.Fatal("owner production shell must load the EVM Flow Radar panel")
	}
	script, err := os.ReadFile("../../public/js/owner-evm-flow-radar-v1.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"/api/owner/radar/evm-flow", "/api/owner/radar/evm-flow/evidence", "credentials:'same-origin'", "securityContent", "Koschei cannot reverse a transfer", "No ClickHouse"} {
		if !strings.Contains(string(script), required) {
			t.Fatalf("panel script missing %q", required)
		}
	}
}
