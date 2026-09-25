package clickhouse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/radarcursor"
)

func sampleGlobalRadarCheckpoint() radarcursor.Checkpoint {
	return radarcursor.Checkpoint{
		CursorKey:         "global-radar/head/ethereum-mainnet/evm_head",
		NetworkID:         "ethereum-mainnet",
		StreamKind:        "evm_head",
		Height:            42,
		BlockHash:         "0xabc",
		ParentHash:        "0xdef",
		SourceEventSHA256: strings.Repeat("a", 64),
		State:             radarcursor.StateCanonical,
		ObservedAt:        time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
	}
}

func TestSaveGlobalRadarIngestCheckpointUsesJSONEachRow(t *testing.T) {
	want := sampleGlobalRadarCheckpoint()
	var got globalRadarIngestCheckpointRow
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "INSERT INTO global_radar_ingest_checkpoints FORMAT JSONEachRow" {
			t.Fatalf("query=%q", r.URL.Query().Get("query"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SaveGlobalRadarIngestCheckpoint(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if got.CursorKey != want.CursorKey || got.Height != want.Height || got.BlockHash != want.BlockHash || got.CheckpointVersion == 0 {
		t.Fatalf("unexpected checkpoint row: %#v", got)
	}
}

func TestLoadGlobalRadarIngestCheckpointReturnsLatestRow(t *testing.T) {
	want := sampleGlobalRadarCheckpoint()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Query().Get("query"), "ORDER BY checkpoint_version DESC") {
			t.Fatalf("unexpected query: %s", r.URL.Query().Get("query"))
		}
		if r.URL.Query().Get("param_cursor") != want.CursorKey {
			t.Fatalf("cursor=%q", r.URL.Query().Get("param_cursor"))
		}
		_ = json.NewEncoder(w).Encode(globalRadarIngestCheckpointReadRow{
			CursorKey:         want.CursorKey,
			SchemaVersion:     radarcursor.SchemaVersion,
			NetworkID:         want.NetworkID,
			StreamKind:        want.StreamKind,
			Height:            want.Height,
			BlockHash:         want.BlockHash,
			ParentHash:        want.ParentHash,
			SourceEventSHA256: want.SourceEventSHA256,
			State:             want.State,
			ObservedAtMillis:  want.ObservedAt.UnixMilli(),
			CheckpointVersion: 77,
		})
	}))
	defer server.Close()

	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := client.LoadGlobalRadarIngestCheckpoint(context.Background(), want.CursorKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Height != want.Height || got.BlockHash != want.BlockHash || got.SourceEventSHA256 != want.SourceEventSHA256 {
		t.Fatalf("unexpected checkpoint: ok=%t %#v", ok, got)
	}
}

func TestLoadGlobalRadarIngestCheckpointMissing(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "")
	}))
	defer server.Close()
	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := client.LoadGlobalRadarIngestCheckpoint(context.Background(), "cursor")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("missing checkpoint reported as present")
	}
}

func TestVerifyGlobalRadarIngestCheckpointSchema(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(query, "FROM system.tables"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"engine": "ReplacingMergeTree", "sorting_key": globalRadarIngestCheckpointSortingKey,
			}}})
		case strings.Contains(query, "FROM system.columns"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"count": 12}}})
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyGlobalRadarIngestCheckpointSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
}
