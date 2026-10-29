package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOwnerIncidentReviewEventStaysMigrationCompatible(t *testing.T) {
	actionType, payload := ownerIncidentReviewEvent()
	if actionType != "note" {
		t.Fatalf("migration 123 allows note, got %q", actionType)
	}
	review, ok := payload[ownerIncidentReviewPayloadKey].(map[string]any)
	if !ok || review["event"] != ownerIncidentReviewEventName || review["scope"] != "manual_case_review_only" {
		t.Fatalf("invalid owner review audit structure: %#v", payload)
	}
	encoded, err := json.Marshal(payload)
	if err != nil || !json.Valid(encoded) {
		t.Fatalf("invalid owner review audit JSON: %v", err)
	}
}

func TestOwnerIncidentReviewProjectionNeverInventsResponseProof(t *testing.T) {
	start := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	incident := ownerIncidentRecord{
		ID: "case-one", CreatedAt: start, UpdatedAt: start, Status: "open",
		EvidenceRefs: []string{"ref-one"},
	}
	items, _, _ := buildOwnerIncidentTriage([]ownerIncidentRecord{incident, {
		ID: "case-two", CreatedAt: start, UpdatedAt: start, Status: "investigating",
	}}, start.Add(2 * time.Hour))
	event := ownerIncidentReviewProof{
		EvidenceState: "owner_manual_review_recorded", Actor: "owner",
		FirstRecordedAt:  start.Add(12 * time.Minute),
		LatestRecordedAt: start.Add(35 * time.Minute),
		RecordCount:      2, SecondsFromCaseCreation: 720,
	}
	applyOwnerIncidentReviewProofs(items, map[string]ownerIncidentReviewProof{"case-one": event})
	if items[0].OwnerReview == nil ||
		items[0].OwnerReview.SecondsFromCaseCreation != 720 ||
		items[0].OwnerReview.RecordCount != 2 {
		t.Fatalf("manual review proof was not projected: %#v", items[0])
	}
	if items[1].OwnerReview != nil {
		t.Fatal("missing owner review event was fabricated")
	}
	if items[0].Incident.Status != "open" ||
		len(items[0].Incident.EvidenceRefs) != 1 || items[0].Incident.EvidenceRefs[0] != "ref-one" {
		t.Fatal("review projection mutated case/evidence")
	}
}

func TestOwnerIncidentReviewRejectsInvalidPayloadBeforeTransaction(t *testing.T) {
	h := &Handler{}
	for _, body := range []string{
		`{"action":"record_owner_review","summary":""}`,
		`{"action":"record_owner_review","summary":"too few"}`,
		`{"action":"record_owner_review","summary":"Reviewed documentation","analyst_id":"soc-123"}`,
		`{"action":"record_owner_review","summary":"Reviewed documentation","tier":"L3"}`,
		`{"action":"record_owner_review","summary":"Reviewed documentation","ref":"unverified"}`,
		`{"action":"record_owner_review","summary":"Reviewed documentation","severity":"critical"}`,
	} {
		rec := httptest.NewRecorder()
		h.ownerIncidentAction(rec, httptest.NewRequest(http.MethodPost,
			"/api/owner/incidents/fixture", strings.NewReader(body)), "fixture")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("bad request returned %d: %s %s", rec.Code, body, rec.Body.String())
		}
	}
}
