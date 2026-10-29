package handlers

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOwnerIncidentHandoffDigestIsDeterministicAndBoundToContent(t *testing.T) {
	stamp := time.Date(2026, 10, 29, 11, 0, 0, 0, time.UTC)
	doc := ownerIncidentHandoffContent{
		SchemaVersion: ownerIncidentHandoffSchema,
		Incident: ownerIncidentRecord{
			ID: "123", IncidentRef: "KSI-123",
			Title: "manual review", CreatedAt: stamp, UpdatedAt: stamp,
			EvidenceRefs: []string{"evidence-1"}, AlertRefs: []string{}, DossierRefs: []string{},
		},
		Actions: []ownerIncidentHandoffAction{{
			ID: "action-1", ActionType: "note", Actor: "owner",
			CreatedAt: stamp, Payload: json.RawMessage(`{"soc_escalation_v1":{"state":"requested","target_tier":"L2"}}`),
		}},
	}
	raw, digest, err := marshalOwnerIncidentHandoff(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(digest) != 64 {
		t.Fatalf("invalid SHA-256 digest length: %d", len(digest))
	}
	if _, err := hex.DecodeString(digest); err != nil {
		t.Fatal(err)
	}
	again, repeatDigest, err := marshalOwnerIncidentHandoff(doc)
	if err != nil || string(raw) != string(again) || digest != repeatDigest {
		t.Fatal("same serialized content must yield the same digest")
	}
	doc.Actions[0].Payload = json.RawMessage(`{"soc_escalation_v1":{"state":"cleared"}}`)
	_, changedDigest, err := marshalOwnerIncidentHandoff(doc)
	if err != nil || digest == changedDigest {
		t.Fatal("changed audit data must change the digest")
	}
	doc.Actions[0].Payload = json.RawMessage(`{"broken":`)
	if _, _, err := marshalOwnerIncidentHandoff(doc); err == nil {
		t.Fatal("invalid JSON in audit data must not be hashed as a valid package")
	}
}

func TestOwnerIncidentHandoffMissingStoreFailsClosed(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()
	h.OwnerIncidentItem(rec, httptest.NewRequest(http.MethodGet, "/api/owner/incidents/test-id?handoff=1", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("owner incident handoff must fail with 503 when database missing, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "content_sha256") {
		t.Fatal("unavailable handoff must not fabricate a digest")
	}
}
