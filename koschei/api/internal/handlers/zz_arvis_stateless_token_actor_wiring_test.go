package handlers

import (
	"os"
	"strings"
	"testing"
)

func TestUnifiedInvestigationAssemblerCollectsActorEvidenceWithoutDatabase(t *testing.T) {
	body, err := os.ReadFile("unified_investigation_report.go")
	if err != nil {
		t.Fatalf("read unified investigation report: %v", err)
	}
	text := string(body)
	const nilStoreCase = "case store == nil:"
	const fundingCall = "h.collectActorFundingOrigin(ctx, nil, creator, network)"
	const liveCall = "h.collectActorDefenseLiveEvidence(ctx, nil, actorDossier)"
	const hydrateCall = "hydrateRequestScopeActorDossier("
	const persistenceMarker = "actorRun.RuleVerdictPersistence = \"database_unavailable\""

	caseIndex := strings.Index(text, nilStoreCase)
	if caseIndex < 0 {
		t.Fatal("unified investigation assembler lost nil-store branch")
	}
	window := text[caseIndex:]
	if next := strings.Index(window, "\n\t\tdefault:"); next >= 0 {
		window = window[:next]
	}
	for label, needle := range map[string]string{
		"request-scope funding": fundingCall,
		"request-scope live evidence": liveCall,
		"request-scope dossier hydration": hydrateCall,
		"truthful verdict persistence": persistenceMarker,
	} {
		if !strings.Contains(window, needle) {
			t.Fatalf("nil-store branch missing %s wiring: %s", label, needle)
		}
	}
	if strings.Contains(window, "actorRun.Status = \"database_unavailable\"") {
		t.Fatal("nil-store branch must not stop live actor collection solely because persistence is unavailable")
	}
}
