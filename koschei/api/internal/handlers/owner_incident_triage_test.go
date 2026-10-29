package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildOwnerIncidentTriageDoesNotInventSLAOrVerdict(t *testing.T) {
	now := time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)
	created := now.Add(-4 * time.Hour)
	updated := now.Add(-30 * time.Minute)
	resolved := now.Add(-time.Hour)
	records := []ownerIncidentRecord{
		{IncidentRef: "KINC-1", Severity: "high", Status: "investigating", CreatedAt: created, UpdatedAt: updated},
		{IncidentRef: "KINC-2", Severity: "critical", Status: "resolved", CreatedAt: created, UpdatedAt: resolved, ResolvedAt: &resolved},
	}
	items, statuses, severities := buildOwnerIncidentTriage(records, now)
	if len(items) != 2 || statuses["investigating"] != 1 || statuses["resolved"] != 1 || severities["high"] != 1 || severities["critical"] != 1 {
		t.Fatalf("bounded queue counts incorrect: items=%v statuses=%v severities=%v", items, statuses, severities)
	}
	if !items[0].Active || items[0].AgeSeconds != 14400 || items[0].SinceLastUpdateSeconds != 1800 || items[0].TimeToResolutionSeconds != nil {
		t.Fatalf("active case age was misinterpreted: %#v", items[0])
	}
	if items[1].Active || items[1].TimeToResolutionSeconds == nil || *items[1].TimeToResolutionSeconds != 10800 {
		t.Fatalf("resolved case timing incorrect: %#v", items[1])
	}
	if items[0].Incident.IncidentRef != records[0].IncidentRef || items[1].Incident.IncidentRef != records[1].IncidentRef {
		t.Fatal("incident identity changed during projection")
	}
}

func TestIncidentElapsedSecondsClampsMissingAndFutureTimestamps(t *testing.T) {
	now := time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)
	if incidentElapsedSeconds(time.Time{}, now) != 0 || incidentElapsedSeconds(now.Add(time.Minute), now) != 0 {
		t.Fatal("missing or future event timestamps must not generate negative age")
	}
}

func TestOwnerIncidentTriageMissingStoreFailsClosed(t *testing.T) {
	h := &Handler{}
	result := httptest.NewRecorder()
	h.OwnerIncidents(result, httptest.NewRequest(http.MethodGet, "/api/owner/incidents?triage=1", nil))
	if result.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing store returned status %d: %s", result.Code, result.Body.String())
	}
	defaultResult := httptest.NewRecorder()
	h.OwnerIncidents(defaultResult, httptest.NewRequest(http.MethodGet, "/api/owner/incidents", nil))
	if defaultResult.Code != http.StatusServiceUnavailable {
		t.Fatalf("default route changed: status %d", defaultResult.Code)
	}
}
