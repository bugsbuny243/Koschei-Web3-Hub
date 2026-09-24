package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	koscheiclickhouse "koschei/api/internal/clickhouse"
)

type recordingGlobalRadarGraphReader struct {
	requests []koscheiclickhouse.GlobalRadarGraphReadRequest
	records  []koscheiclickhouse.GlobalRadarStoredRecord
	err      error
}

func (r *recordingGlobalRadarGraphReader) ReadGlobalRadarGraph(_ context.Context, request koscheiclickhouse.GlobalRadarGraphReadRequest) ([]koscheiclickhouse.GlobalRadarStoredRecord, error) {
	r.requests = append(r.requests, request)
	if r.err != nil {
		return nil, r.err
	}
	return append([]koscheiclickhouse.GlobalRadarStoredRecord(nil), r.records...), nil
}

func TestOwnerGlobalRadarGraphRecordsRequiresExistingOwnerAuth(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "owner-test-secret")
	t.Setenv("OWNER_WALLET", "")
	reader := &recordingGlobalRadarGraphReader{}
	server := NewServer(nil, "", "", "", t.TempDir(), WithGlobalRadarGraphReader(reader))

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/owner/radar/global/records?network=ethereum-mainnet", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(reader.requests) != 0 {
		t.Fatal("unauthenticated graph read reached ClickHouse reader")
	}
}

func TestOwnerGlobalRadarGraphRecordsUsesBoundedDefaultWindow(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "owner-test-secret")
	t.Setenv("OWNER_WALLET", "")
	reader := &recordingGlobalRadarGraphReader{
		records: []koscheiclickhouse.GlobalRadarStoredRecord{{
			RecordType:     "observation",
			RecordID:       "observation-1",
			SchemaVersion:  "koschei.global-radar-observation.v1",
			Network:        "ethereum-mainnet",
			SubjectID:      "subject-1",
			RecordKind:     "node",
			EvidenceStatus: "observed",
			RecordedAt:     time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC),
			Payload:        json.RawMessage(`{"ok":true}`),
			PayloadSHA256:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	}
	server := NewServer(nil, "", "", "", t.TempDir(), WithGlobalRadarGraphReader(reader))
	request := httptest.NewRequest(http.MethodGet, "/api/owner/radar/global/records?network=ethereum-mainnet&limit=10", nil)
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
	if gotRequest.Network != "ethereum-mainnet" || gotRequest.Limit != 10 {
		t.Fatalf("unexpected reader request: %#v", gotRequest)
	}
	window := gotRequest.Until.Sub(gotRequest.Since)
	if window < 23*time.Hour+59*time.Minute || window > 24*time.Hour+time.Minute {
		t.Fatalf("default window=%s", window)
	}

	var payload globalRadarOwnerRecordsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SchemaVersion != globalRadarOwnerRecordsSchemaVersion || payload.Count != 1 {
		t.Fatalf("unexpected response: %#v", payload)
	}
	if payload.Scope != "persisted_historical_graph_records_not_current_chain_truth" ||
		payload.TruthBoundary.CurrentChainState ||
		!payload.TruthBoundary.PayloadHashReverified {
		t.Fatalf("truth boundary missing: %#v", payload.TruthBoundary)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner graph records must not be cached")
	}
}

func TestOwnerGlobalRadarGraphRecordsRejectsUnregisteredNetworkBeforeReader(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "owner-test-secret")
	t.Setenv("OWNER_WALLET", "")
	reader := &recordingGlobalRadarGraphReader{}
	server := NewServer(nil, "", "", "", t.TempDir(), WithGlobalRadarGraphReader(reader))
	request := httptest.NewRequest(http.MethodGet, "/api/owner/radar/global/records?network=unknown-mainnet", nil)
	request.Header.Set("x-koschei-secret", "owner-test-secret")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(reader.requests) != 0 {
		t.Fatal("invalid network reached ClickHouse reader")
	}
}
