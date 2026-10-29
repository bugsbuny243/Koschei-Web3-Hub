package handlers

import (
	"testing"
	"time"
)

func TestOwnerIncidentMilestonesUseOnlyRecordedActions(t *testing.T) {
	created := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	investigate := created.Add(15 * time.Minute)
	contain := created.Add(75 * time.Minute)
	resolve := created.Add(3 * time.Hour)
	reopen := created.Add(4 * time.Hour)
	got := buildOwnerIncidentWorkflowMilestones(created, ownerIncidentMilestoneTimes{
		Investigation: &investigate, Containment: &contain,
		Resolution: &resolve, Reopen: &reopen,
	})
	if got == nil || got.SecondsToFirstInvestigation == nil || *got.SecondsToFirstInvestigation != 900 {
		t.Fatalf("recorded initial investigation elapsed incorrectly: %#v", got)
	}
	if got.SecondsToFirstContainment == nil || *got.SecondsToFirstContainment != 4500 {
		t.Fatalf("first containment label elapsed incorrectly: %#v", got)
	}
	if got.SecondsToFirstResolution == nil || *got.SecondsToFirstResolution != 10800 || got.FirstReopenAt == nil {
		t.Fatalf("resolution/reopen record incorrectly represented: %#v", got)
	}
	if got.EvidenceBasis != "owner_incident_actions_timestamp_not_verified_response_or_enforcement" {
		t.Fatalf("workflow record was presented as verified technical enforcement: %#v", got)
	}
}

func TestOwnerIncidentMilestonesMissingEvidenceDoesNotInventResponse(t *testing.T) {
	created := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	if buildOwnerIncidentWorkflowMilestones(created, ownerIncidentMilestoneTimes{}) != nil {
		t.Fatal("missing workflow events must not fabricate response milestones")
	}
	before := created.Add(-1 * time.Hour)
	if buildOwnerIncidentWorkflowMilestones(created, ownerIncidentMilestoneTimes{Investigation: &before}) != nil {
		t.Fatal("invalid precreation event must not fabricate response timing")
	}
	after := created.Add(time.Second)
	if buildOwnerIncidentWorkflowMilestones(time.Time{}, ownerIncidentMilestoneTimes{Investigation: &after}) != nil {
		t.Fatal("missing case creation timestamp must not fabricate response timing")
	}
}

func TestOwnerIncidentMilestonesRetainHistoricalCasesWithoutMutatingEvidence(t *testing.T) {
	created := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	contain := created.Add(time.Hour)
	incidents := []ownerIncidentRecord{
		{ID: "case-1", Status: "open", EvidenceRefs: []string{"evidence-a"}, CreatedAt: created},
		{ID: "case-2", Status: "closed", CreatedAt: created},
	}
	items, _, _ := buildOwnerIncidentTriage(incidents, created.Add(2*time.Hour))
	milestones := map[string]*ownerIncidentWorkflowMilestones{
		"case-1": buildOwnerIncidentWorkflowMilestones(created, ownerIncidentMilestoneTimes{Containment: &contain}),
	}
	applyOwnerIncidentMilestones(items, milestones)
	if items[0].WorkflowMilestones == nil || items[0].WorkflowMilestones.FirstContainmentRecordAt == nil {
		t.Fatal("recorded case event was not projected")
	}
	if items[1].WorkflowMilestones != nil {
		t.Fatal("unobserved incident timeline was fabricated")
	}
	if items[0].Incident.Status != "open" || len(items[0].Incident.EvidenceRefs) != 1 || items[0].Incident.EvidenceRefs[0] != "evidence-a" {
		t.Fatal("milestone projection modified case status or source evidence")
	}
}
