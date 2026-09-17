package handlers

import (
	"os"
	"strings"
	"testing"
)

func TestUnifiedInvestigationWiresStatelessDistributionBeforeActorVerdict(t *testing.T) {
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
	collect := strings.Index(window, "collectRequestScopeCanonicalActorDistribution(ctx, creatorRelation, network)")
	apply := strings.Index(window, "applyRequestScopeActorDistributionEvidence(actorDossier, requestDistributionEvidence)")
	rehydrate := strings.LastIndex(window, "hydrateRequestScopeActorDossier(")
	if collect < 0 || apply < 0 || rehydrate < 0 {
		t.Fatalf("stateless distribution wiring incomplete: collect=%d apply=%d rehydrate=%d", collect, apply, rehydrate)
	}
	if !(collect < apply && apply < rehydrate) {
		t.Fatalf("stateless distribution order invalid: collect=%d apply=%d rehydrate=%d", collect, apply, rehydrate)
	}
	if strings.Contains(window, "distributionRun.Status = \"persistence_unavailable\"") {
		t.Fatal("nil-store branch still hard-disables current-token distribution")
	}

	verdictIndex := strings.Index(text, "actorVerdict := services.EvaluateActorDefenseRules")
	if verdictIndex < 0 || caseIndex+collect >= verdictIndex {
		t.Fatal("stateless distribution must execute before actor verdict evaluation")
	}
}
