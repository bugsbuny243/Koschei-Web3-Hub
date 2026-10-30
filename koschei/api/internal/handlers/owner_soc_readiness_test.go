package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSOCRuntimeReadinessStatesPreserveUnknown(t *testing.T) {
	for _, tc := range []struct {
		cases  int
		audit  bool
		linked int
		want   string
	}{
		{0, false, 0, "not_accepted_no_case_observations"},
		{0, true, 0, "not_accepted_no_case_observations"},
		{1, false, 0, "not_accepted_no_audit_event_observations"},
		{5, true, 0, "not_accepted_no_linked_sample_audit_events"},
		{5, true, 1, "bounded_case_audit_link_observed_not_end_to_end_accepted"},
	} {
		if actual := socReadinessState(tc.cases, tc.audit, tc.linked); actual != tc.want {
			t.Fatalf("cases=%d audit=%v: got %q, want %q", tc.cases, tc.audit, actual, tc.want)
		}
	}
}

func TestSOCRuntimeReadinessFailsClosedWithoutDB(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()
	h.OwnerSOCReadiness(rec, httptest.NewRequest(http.MethodGet, "/api/owner/soc/readiness", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("DB missing gave HTTP %d; want 503", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner read probe response may not be cached")
	}
	var payload struct {
		OK        bool                      `json:"ok"`
		Readiness ownerSOCReadinessSnapshot `json:"readiness"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid readiness JSON: %v", err)
	}
	if payload.OK || payload.Readiness.State != "unavailable_not_accepted" ||
		payload.Readiness.CasesTableReadable || payload.Readiness.ActionsTableReadable {
		t.Fatalf("missing DB was presented as ready: %#v", payload)
	}
	if payload.Readiness.SchemaVersion != ownerSOCReadinessVersion ||
		payload.Readiness.Scope != socReadinessScope || payload.Readiness.Boundary != socReadinessBoundary {
		t.Fatal("readiness truth-boundary contract missing")
	}
}

func TestSOCReadinessOperationsNeverUsesReadReplicaAsPrimary(t *testing.T) {
	// An absent primary cannot inherit readiness from a legacy read source.
	got := probeSOCOperations(context.Background(), nil)
	if got.State != "unavailable_not_accepted" || got.MetadataReadable || got.HistoryReadable || got.ReadSource != "primary_db" {
		t.Fatalf("%+v", got)
	}
}
