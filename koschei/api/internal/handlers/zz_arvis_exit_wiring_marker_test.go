package handlers

import (
	"os"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func TestUnifiedInvestigationAssemblerWiresRequestScopeActorExitBeforeC008(t *testing.T) {
	body, err := os.ReadFile("unified_investigation_report.go")
	if err != nil {
		t.Fatalf("read unified investigation report: %v", err)
	}
	text := string(body)
	const evidenceBuild = "requestScopeExitEvidence := append([]services.ActorDefenseEvidenceRecord{}, actorDossier.Evidence...)"
	const requestScopeCall = "actorExit = applyRequestScopeActorExitRecurrence(&core, actorExit, requestScopeExitEvidence, creator, network, target)"
	const c008Call = "behavior = services.ApplyCrossTokenExitEventRecurrenceRuleV140(behavior, actorExit, now)"

	if !strings.Contains(text, evidenceBuild) {
		t.Fatalf("unified investigation assembler must build request-scope exit evidence without requiring persistence")
	}
	requestScopeIndex := strings.Index(text, requestScopeCall)
	if requestScopeIndex < 0 {
		t.Fatalf("unified investigation assembler must wire request-scope actor exit recurrence")
	}
	c008Index := strings.Index(text, c008Call)
	if c008Index < 0 {
		t.Fatalf("unified investigation assembler must evaluate URD-C008")
	}
	if requestScopeIndex > c008Index {
		t.Fatalf("request-scope actor exit recurrence must be selected before URD-C008")
	}
}

func TestPreferRequestScopeActorExitPreservesStrongPersistentVerified(t *testing.T) {
	current := services.ActorExitRecurrence{
		Available: true, EvidenceStatus: "verified", DistinctTargetsWithEvents: 3,
		OtherTargets: []string{"mint-a", "mint-b"}, ReferencesComplete: true,
	}
	requestScope := services.ActorExitRecurrence{
		Available: true, EvidenceStatus: "verified", DistinctTargetsWithEvents: 2,
		OtherTargets: []string{"mint-a"}, ReferencesComplete: true,
	}
	if preferRequestScopeActorExit(current, requestScope) {
		t.Fatal("weaker request-scope recurrence replaced stronger persistent VERIFIED evidence")
	}
}

func TestPreferRequestScopeActorExitWorksWithoutPersistentRecurrence(t *testing.T) {
	current := services.ActorExitRecurrence{
		Status: "unavailable", EvidenceStatus: "unavailable",
	}
	requestScope := services.ActorExitRecurrence{
		Available: true, EvidenceStatus: "verified", DistinctTargetsWithEvents: 2,
		OtherTargets: []string{"mint-a"}, ReferencesComplete: true,
	}
	if !preferRequestScopeActorExit(current, requestScope) {
		t.Fatal("verified request-scope recurrence should be usable without persistent recurrence")
	}
}
