package handlers

import (
	"os"
	"strings"
	"testing"
)

func TestUnifiedInvestigationAssemblerWiresRequestScopeActorLifecycle(t *testing.T) {
	body, err := os.ReadFile("unified_investigation_report.go")
	if err != nil {
		t.Fatalf("read unified investigation report: %v", err)
	}
	const call = "actorLifecycle = applyRequestScopeActorLifecycleRecurrence(&core, actorLifecycle, externalDiscovery, creator, network, target)"
	if !strings.Contains(string(body), call) {
		t.Fatalf("unified investigation assembler must wire request-scope actor lifecycle recurrence")
	}
}
