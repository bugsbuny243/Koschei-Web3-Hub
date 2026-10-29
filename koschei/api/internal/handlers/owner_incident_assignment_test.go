package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOwnerIncidentAnalystAssignmentInputValidation(t *testing.T) {
	for _, tier := range []string{"L1", "L2", "L3"} {
		if !validIncidentAnalystTier(tier) {
			t.Fatalf("valid analyst tier %q rejected", tier)
		}
	}
	for _, tier := range []string{"", "L0", "L4", "owner", "L2/L3"} {
		if validIncidentAnalystTier(tier) {
			t.Fatalf("invalid analyst tier %q accepted", tier)
		}
	}
	for _, id := range []string{"soc-analyst-001", "Operator_3", "team.alpha"} {
		if !validIncidentAnalystID(id) {
			t.Fatalf("valid opaque analyst ID %q rejected", id)
		}
	}
	for _, id := range []string{"", "a", " name", "../admin", "analyst@example.org", "two words", strings.Repeat("x", 65)} {
		if validIncidentAnalystID(id) {
			t.Fatalf("invalid analyst ID %q accepted", id)
		}
	}
}

func TestOwnerIncidentAssignmentEventsAreVersionedNotes(t *testing.T) {
	action, fields := ownerIncidentAssignmentEvent("assign_analyst", "soc-001", "L2")
	if action != "note" {
		t.Fatalf("migration 123 only accepts existing action types, got %q", action)
	}
	payload, ok := fields[ownerIncidentAssignmentPayloadKey].(map[string]any)
	if !ok || payload["status"] != "assigned" || payload["analyst_id"] != "soc-001" || payload["tier"] != "L2" {
		t.Fatalf("assignment audit payload incorrect: %#v", fields)
	}
	action, fields = ownerIncidentAssignmentEvent("unassign_analyst", "", "")
	payload, ok = fields[ownerIncidentAssignmentPayloadKey].(map[string]any)
	if action != "note" || !ok || payload["status"] != "unassigned" {
		t.Fatalf("unassignment audit payload incorrect: %#v", fields)
	}
	if _, ok := payload["analyst_id"]; ok {
		t.Fatalf("unassignment event must not invent an assigned analyst: %#v", payload)
	}
	encoded, err := json.Marshal(fields)
	if err != nil || !json.Valid(encoded) {
		t.Fatalf("audit event is not valid JSON: %v", err)
	}
}

func TestOwnerIncidentTriageAssignmentsDoNotChangeEvidence(t *testing.T) {
	before := []ownerIncidentRecord{
		{ID: "incident-1", IncidentRef: "KSI-1", EvidenceRefs: []string{"ev-1"}},
		{ID: "incident-2", IncidentRef: "KSI-2"},
		{ID: "incident-3", IncidentRef: "KSI-3"},
	}
	items, _, _ := buildOwnerIncidentTriage(before, time.Now())
	stamp := time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)
	assignments := map[string]ownerIncidentAssignment{
		"incident-1": {Status: "assigned", AnalystID: "soc-001", Tier: "L1", UpdatedAt: stamp},
		"incident-2": {Status: "unassigned", UpdatedAt: stamp},
	}
	applyOwnerIncidentAssignments(items, assignments)
	if items[0].AnalystAssignment == nil || items[0].AnalystAssignment.Tier != "L1" {
		t.Fatalf("latest analyst assignment not projected: %#v", items[0])
	}
	if items[1].AnalystAssignment == nil || items[1].AnalystAssignment.Status != "unassigned" || items[1].AnalystAssignment.AnalystID != "" {
		t.Fatalf("unassignment not projected: %#v", items[1])
	}
	if items[2].AnalystAssignment != nil {
		t.Fatalf("missing assignment was fabricated: %#v", items[2])
	}
	if len(items[0].Incident.EvidenceRefs) != 1 || items[0].Incident.EvidenceRefs[0] != "ev-1" {
		t.Fatal("assignment projection mutated existing incident evidence")
	}
}

func TestOwnerIncidentAssignmentRejectsInvalidRequestsBeforePersistence(t *testing.T) {
	h := &Handler{}
	cases := []string{
		`{"action":"assign_analyst","analyst_id":"../../root","tier":"L1"}`,
		`{"action":"assign_analyst","analyst_id":"soc-001","tier":"L4"}`,
		`{"action":"assign_analyst","analyst_id":"","tier":"L1"}`,
		`{"action":"unassign_analyst","analyst_id":"soc-001"}`,
		`{"action":"note","analyst_id":"soc-001","tier":"L1"}`,
	}
	for _, body := range cases {
		rec := httptest.NewRecorder()
		h.ownerIncidentAction(rec, httptest.NewRequest(http.MethodPost, "/api/owner/incidents/abc", strings.NewReader(body)), "abc")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("invalid payload %s returned %d: %s", body, rec.Code, rec.Body.String())
		}
	}
}
