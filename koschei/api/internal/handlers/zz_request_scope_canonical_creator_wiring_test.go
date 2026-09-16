package handlers

import (
	"os"
	"strings"
	"testing"
)

func TestUnifiedInvestigationWiresCanonicalCreatorIntoStatelessDossier(t *testing.T) {
	body, err := os.ReadFile("unified_investigation_report.go")
	if err != nil {
		t.Fatalf("read unified investigation report: %v", err)
	}
	text := string(body)
	caseIndex := strings.Index(text, "case store == nil:")
	if caseIndex < 0 {
		t.Fatal("nil-store actor branch missing")
	}
	window := text[caseIndex:]
	if next := strings.Index(window, "\n\t\tdefault:"); next >= 0 {
		window = window[:next]
	}
	for label, needle := range map[string]string{
		"canonical relation build":      "buildRequestScopeCanonicalCreatorMintRelation(core, creator, network)",
		"canonical dossier projection":  "applyRequestScopeCanonicalCreatorRelation(actorDossier, creatorRelation)",
		"non-persistent verdict marker": "actorRun.RuleVerdictPersistence = \"database_unavailable\"",
	} {
		if !strings.Contains(window, needle) {
			t.Fatalf("nil-store actor branch missing %s: %s", label, needle)
		}
	}
}
