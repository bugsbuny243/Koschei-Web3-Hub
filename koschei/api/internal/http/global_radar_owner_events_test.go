package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	koscheiclickhouse "koschei/api/internal/clickhouse"
	"koschei/api/internal/radarevent"
)

type recordingGlobalRadarEventReader struct {
	requests []koscheiclickhouse.GlobalRadarEventReadRequest
	events   []koscheiclickhouse.GlobalRadarStoredEvent
}

func (r *recordingGlobalRadarEventReader) ReadGlobalRadarEvents(_ context.Context, request koscheiclickhouse.GlobalRadarEventReadRequest) ([]koscheiclickhouse.GlobalRadarStoredEvent, error) {
	r.requests = append(r.requests, request)
	return append([]koscheiclickhouse.GlobalRadarStoredEvent(nil), r.events...), nil
}

func TestOwnerGlobalRadarEventsRequiresOwnerAuth(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "owner-test-secret")
	t.Setenv("OWNER_WALLET", "")
	reader := &recordingGlobalRadarEventReader{}
	server := NewServer(nil, "", "", "", t.TempDir(), WithGlobalRadarEventReader(reader))

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/owner/radar/global/events?network=ethereum-mainnet", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(reader.requests) != 0 {
		t.Fatal("unauthenticated event read reached ClickHouse reader")
	}
}

func TestOwnerGlobalRadarEventsReturnsReverifiedHistoricalEvents(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "owner-test-secret")
	t.Setenv("OWNER_WALLET", "")
	observedAt := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	reader := &recordingGlobalRadarEventReader{
		events: []koscheiclickhouse.GlobalRadarStoredEvent{{
			Event: radarevent.Event{
				SchemaVersion:    radarevent.SchemaVersionV1,
				Producer:         "test-adapter",
				Kind:             radarevent.KindNetworkHealth,
				NetworkID:        "ethereum-mainnet",
				SubjectKind:      "node",
				SubjectID:        "node-1",
				ObservedAtUnixMS: observedAt.UnixMilli(),
			},
			ObservedAt:    observedAt,
			IngestedAt:    observedAt.Add(time.Minute),
			PayloadSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			IngestVersion: 7,
		}},
	}
	server := NewServer(nil, "", "", "", t.TempDir(), WithGlobalRadarEventReader(reader))
	request := httptest.NewRequest(http.MethodGet, "/api/owner/radar/global/events?network=ethereum-mainnet&kind=network_health&limit=10", nil)
	request.Header.Set("x-koschei-secret", "owner-test-secret")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(reader.requests) != 1 {
		t.Fatalf("reader requests=%d", len(reader.requests))
	}
	gotRequest := reader.requests[0]
	if gotRequest.Network != "ethereum-mainnet" || gotRequest.Kind != radarevent.KindNetworkHealth || gotRequest.Limit != 10 {
		t.Fatalf("unexpected reader request: %#v", gotRequest)
	}
	var payload globalRadarOwnerEventsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Count != 1 ||
		payload.Scope != "persisted_canonical_events_not_current_chain_truth" ||
		payload.TruthBoundary.CurrentChainState ||
		!payload.TruthBoundary.EventDigestReverified ||
		!payload.TruthBoundary.RowPayloadIdentityMatched {
		t.Fatalf("unexpected response: %#v", payload)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner event records must not be cached")
	}
}


func TestOwnerGlobalRadarEventsAcceptsLogKind(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "owner-test-secret")
	t.Setenv("OWNER_WALLET", "")
	reader := &recordingGlobalRadarEventReader{}
	server := NewServer(nil, "", "", "", t.TempDir(), WithGlobalRadarEventReader(reader))
	request := httptest.NewRequest(http.MethodGet, "/api/owner/radar/global/events?network=ethereum-mainnet&kind=log&limit=5", nil)
	request.Header.Set("x-koschei-secret", "owner-test-secret")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(reader.requests) != 1 || reader.requests[0].Kind != radarevent.KindLog || reader.requests[0].Limit != 5 {
		t.Fatalf("unexpected log read request: %#v", reader.requests)
	}
}

func TestOwnerGlobalRadarEventsRejectsUnknownKindBeforeReader(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "owner-test-secret")
	t.Setenv("OWNER_WALLET", "")
	reader := &recordingGlobalRadarEventReader{}
	server := NewServer(nil, "", "", "", t.TempDir(), WithGlobalRadarEventReader(reader))
	request := httptest.NewRequest(http.MethodGet, "/api/owner/radar/global/events?network=ethereum-mainnet&kind=safe", nil)
	request.Header.Set("x-koschei-secret", "owner-test-secret")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(reader.requests) != 0 {
		t.Fatal("invalid event kind reached ClickHouse reader")
	}
}
