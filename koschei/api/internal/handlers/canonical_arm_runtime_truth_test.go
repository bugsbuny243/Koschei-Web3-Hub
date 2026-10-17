package handlers

import (
	"fmt"
	"strings"
	"testing"
)

func TestCanonicalARVISRuntimeCoverageDoesNotConfuse14ModulesWith7Completed(t *testing.T) {
	modules := make([]map[string]any, 14)
	for i := range modules {
		modules[i] = map[string]any{"module_id": fmt.Sprintf("arm_%02d", i+1)}
	}
	report := map[string]any{}
	report["modules"] = modules
	report["full_scan_live_evidence"] = map[string]any{"status": "complete"}
	report["investigation_coverage"] = map[string]any{"capability_total": 14, "attempted": 9, "completed": 7, "evidence_producing": 7, "evidence_pending": 6, "not_applicable": 1, "source_unavailable": 0, "insufficient_evidence": 0, "critical_gaps": []string{"holder_funding_relations", "launch_and_first_buyer_history"}, "status": "partial_investigation"}
	coverage := buildCanonicalIntegrationCoverage(report)
	got := coverage.Capabilities["arvis_14_arm_engine"]
	if !coverage.LiveScanRequested || got.Status != canonicalCapabilityPartial ||
		!got.WiredToCanonicalRadar || !got.EvidenceBacked ||
		!strings.Contains(strings.Join(got.Limitations, " "), "7/14 completed") {
		t.Fatalf("14 architecture entries were promoted to completed arms: %#v", got)
	}
	if coverage.OverallStatus == "complete" {
		t.Fatalf("runtime gaps hidden from capability integration summary: %#v", coverage)
	}
}

func TestCanonicalARVISRuntimeCoverageWithoutProofFailsClosed(t *testing.T) {
	base := canonicalStatusFromCount("ARVIS", 14, 14, true, "modules", "")
	if base.Status != canonicalCapabilityActive {
		t.Fatalf("fixture setup not active: %#v", base)
	}
	check := func(name string, raw any, want string) {
		t.Run(name, func(t *testing.T) {
			item := canonicalArmRuntimeTruth(base, raw, 14)
			if item.Status != want {
				t.Fatalf("got %s want %s: %#v", item.Status, want, item)
			}
		})
	}
	check("missing runtime", nil, canonicalCapabilityPartial)
	check("seven completed", map[string]any{"capability_total": 14, "completed": 7, "evidence_producing": 7, "evidence_pending": 6, "not_applicable": 1, "status": "partial_investigation"}, canonicalCapabilityPartial)
	check("counts contradict total", map[string]any{"capability_total": 14, "completed": 7, "evidence_pending": 4, "evidence_producing": 7, "not_applicable": 1}, canonicalCapabilityPartial)
	check("wrong declared architecture", map[string]any{"capability_total": 15, "completed": 14, "not_applicable": 1, "status": "complete_investigation"}, canonicalCapabilityPartial)
	check("all applicable completed", map[string]any{"capability_total": 14, "completed": 13, "evidence_producing": 11, "not_applicable": 1, "critical_gaps": []string{}, "status": "complete_investigation"}, canonicalCapabilityActive)
	check("all completed but gap persists", map[string]any{"capability_total": 14, "completed": 14, "evidence_producing": 12, "critical_gaps": []string{"holder_funding_relations"}, "status": "partial_investigation"}, canonicalCapabilityPartial)
	check("all completed but status disagrees", map[string]any{"capability_total": 14, "completed": 14, "evidence_producing": 11, "status": "partial_investigation"}, canonicalCapabilityPartial)
	check("negative counter", map[string]any{"capability_total": 14, "completed": 15, "evidence_producing": 14, "evidence_pending": -1}, canonicalCapabilityPartial)
}
