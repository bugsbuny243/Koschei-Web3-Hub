package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOwnerIncidentManualEscalationTierRanks(t *testing.T) {
	for tier, want := range map[string]int{"L1": 1, "L2": 2, "L3": 3, "": 0, "L4": 0} {
		if got := incidentAnalystTierRank(tier); got != want {
			t.Fatalf("rank(%q) = %d, want %d", tier, got, want)
		}
	}
	if !(incidentAnalystTierRank("L3") > incidentAnalystTierRank("L2")) ||
		!(incidentAnalystTierRank("L2") > incidentAnalystTierRank("L1")) ||
		incidentAnalystTierRank("L1") > incidentAnalystTierRank("L2") {
		t.Fatal("tier escalation order invalid")
	}
}

func TestOwnerIncidentManualEscalationUsesCompatibleAuditedNotes(t *testing.T) {
	for _, tc := range []struct{ action, tier, state string }{
		{"request_escalation", "L3", "requested"},
		{"clear_escalation", "", "cleared"},
	} {
		actionType, event := ownerIncidentEscalationEvent(tc.action, tc.tier)
		if actionType != "note" {
			t.Fatalf("migration 123 requires note action, got %q", actionType)
		}
		val, ok := event[ownerIncidentEscalationPayloadKey].(map[string]any)
		if !ok || val["state"] != tc.state {
			t.Fatalf("invalid escalation payload: %#v", event)
		}
		if tc.tier == "" {
			if _, present := val["target_tier"]; present {
				t.Fatalf("clear event cannot retain a target tier: %#v", val)
			}
		} else if val["target_tier"] != tc.tier {
			t.Fatalf("missing requested tier: %#v", val)
		}
		data, err := json.Marshal(event)
		if err != nil || !json.Valid(data) {
			t.Fatalf("invalid audit JSON: %v", err)
		}
	}
}

func TestOwnerIncidentEscalationReadProjectionNeverClaimsAcknowledgement(t *testing.T) {
	at := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	incidents := []ownerIncidentRecord{
		{ID: "active", Status: "investigating", EvidenceRefs: []string{"evidence-a"}},
		{ID: "inactive", Status: "closed"},
		{ID: "changed", Status: "open"},
		{ID: "missing", Status: "open"},
		{ID: "lower-tier", Status: "investigating"},
	}
	items, _, _ := buildOwnerIncidentTriage(incidents, at)
	applyOwnerIncidentAssignments(items, map[string]ownerIncidentAssignment{
		"active":     {Status: "assigned", Tier: "L1", AnalystID: "soc-001", UpdatedAt: at.Add(-time.Hour)},
		"changed":    {Status: "assigned", Tier: "L3", AnalystID: "soc-002", UpdatedAt: at.Add(time.Minute)},
		"lower-tier": {Status: "assigned", Tier: "L1", AnalystID: "soc-003", UpdatedAt: at.Add(time.Minute)},
	})
	applyOwnerIncidentEscalations(items, map[string]ownerIncidentEscalation{
		"active":     {State: "requested", TargetTier: "L2", RequestedAt: at},
		"inactive":   {State: "requested", TargetTier: "L3", RequestedAt: at},
		"changed":    {State: "requested", TargetTier: "L3", RequestedAt: at},
		"lower-tier": {State: "requested", TargetTier: "L3", RequestedAt: at},
	})
	if items[0].Escalation == nil || items[0].Escalation.State != "requested" {
		t.Fatalf("active request was lost: %#v", items[0])
	}
	if items[1].Escalation == nil || items[1].Escalation.State != "historical_request_case_inactive" {
		t.Fatalf("closed case must not appear actively pending: %#v", items[1])
	}
	if items[2].Escalation == nil || items[2].Escalation.State != "superseded_by_later_assignment_not_acknowledged" {
		t.Fatalf("assignment change must not imply acknowledgement: %#v", items[2])
	}
	if items[3].Escalation != nil {
		t.Fatalf("missing audit event must not be fabricated: %#v", items[3])
	}
	if items[4].Escalation == nil || items[4].Escalation.State != "requested" {
		t.Fatalf("lower-tier reassignments must preserve pending escalation requests: %#v", items[4])
	}
	if len(items[0].Incident.EvidenceRefs) != 1 || items[0].Incident.EvidenceRefs[0] != "evidence-a" {
		t.Fatal("escalation projected an altered evidence reference")
	}
}

func TestOwnerIncidentManualEscalationRejectsBadPayloadsBeforeStore(t *testing.T) {
	h := &Handler{}
	inputs := []string{
		`{"action":"request_escalation","tier":"L1","summary":"Escalation rationale"}`,
		`{"action":"request_escalation","tier":"L4","summary":"Escalation rationale"}`,
		`{"action":"request_escalation","tier":"L2","summary":"short"}`,
		`{"action":"request_escalation","tier":"L2","analyst_id":"soc-001","summary":"Escalation rationale"}`,
		`{"action":"clear_escalation","tier":"L3","summary":"Manually cleared"}`,
		`{"action":"clear_escalation","analyst_id":"soc-002"}`,
	}
	for _, body := range inputs {
		r := httptest.NewRequest(http.MethodPost, "/api/owner/incidents/example", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ownerIncidentAction(rec, r, "example")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("invalid request %s returned %d: %s", body, rec.Code, rec.Body.String())
		}
	}
}
