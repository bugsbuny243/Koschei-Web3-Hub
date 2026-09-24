package clickhouse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"koschei/api/internal/radarevent"
	"koschei/api/internal/securityevidence"
)

func sampleGlobalRadarEvent(t *testing.T) radarevent.Event {
	t.Helper()
	event, err := (radarevent.Event{
		Producer:         "ethereum-node-adapter",
		Kind:             radarevent.KindNetworkHealth,
		NetworkID:        "ethereum-mainnet",
		SubjectKind:      "node",
		SubjectID:        "ethereum-mainnet:node:example",
		ObservedAtUnixMS: time.Date(2026, 9, 24, 6, 0, 0, 123000000, time.UTC).UnixMilli(),
		State:            securityevidence.StateObserved,
		NativeRefs: []radarevent.NativeReference{{
			Kind: "rpc_endpoint", Value: "node.example",
		}},
		SourceDigests: []string{strings.Repeat("a", 64)},
		Facts: []radarevent.Fact{{
			Key: "peer_count", Value: "12", Unit: "peers", EvidenceSHA256: strings.Repeat("a", 64),
		}},
	}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestGlobalRadarEventRowFromEventPreservesCanonicalDigest(t *testing.T) {
	event := sampleGlobalRadarEvent(t)
	row, err := globalRadarEventRowFromEvent(event, 42)
	if err != nil {
		t.Fatal(err)
	}
	if row.EventSHA256 != event.EventSHA256 || row.SchemaVersion != radarevent.SchemaVersionV1 {
		t.Fatalf("event identity changed: %#v", row)
	}
	if row.NetworkID != "ethereum-mainnet" || row.SubjectID != event.SubjectID {
		t.Fatalf("event boundary changed: %#v", row)
	}
	if row.PayloadSHA256 != jsonPayloadSHA256(json.RawMessage(row.PayloadJSON)) {
		t.Fatal("payload hash does not bind stored event JSON")
	}
	if row.IngestVersion != 42 {
		t.Fatalf("ingest_version=%d", row.IngestVersion)
	}
}

func TestInsertGlobalRadarEventsUsesCanonicalJSONEachRowContract(t *testing.T) {
	event := sampleGlobalRadarEvent(t)
	var rows []globalRadarEventRow
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("query"); got != "INSERT INTO global_radar_events FORMAT JSONEachRow" {
			t.Fatalf("insert query=%q", got)
		}
		if r.URL.Query().Get("async_insert") != "1" || r.URL.Query().Get("wait_for_async_insert") != "1" {
			t.Fatal("Global Radar event insert must wait for async acceptance")
		}
		decoder := json.NewDecoder(r.Body)
		for {
			var row globalRadarEventRow
			err := decoder.Decode(&row)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			rows = append(rows, row)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   "koschei_web3",
		User:       "radar-user",
		Password:   "radar-password",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.InsertGlobalRadarEvents(context.Background(), []radarevent.Event{event}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].EventSHA256 != event.EventSHA256 {
		t.Fatalf("unexpected inserted rows: %#v", rows)
	}
}

func TestInsertGlobalRadarEventsRejectsTamperedEventBeforeNetworkWrite(t *testing.T) {
	event := sampleGlobalRadarEvent(t)
	event.Facts[0].Value = "99"
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   "koschei_web3",
		User:       "radar-user",
		Password:   "radar-password",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.InsertGlobalRadarEvents(context.Background(), []radarevent.Event{event}); err == nil {
		t.Fatal("tampered Global Radar event was accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("tampered event reached ClickHouse")
	}
}

func TestVerifyGlobalRadarEventSchemaAcceptsCanonicalTable(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(query, "FROM system.tables"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"engine":      "ReplacingMergeTree",
					"sorting_key": globalRadarEventSortingKey,
				}},
			})
		case strings.Contains(query, "FROM system.columns"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"count": 15}},
			})
		default:
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   "koschei_web3",
		User:       "radar-user",
		Password:   "radar-password",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyGlobalRadarEventSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
}
