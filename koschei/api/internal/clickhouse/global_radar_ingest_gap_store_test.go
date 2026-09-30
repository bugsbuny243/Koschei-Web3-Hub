package clickhouse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/radarcursor"
)

func sampleGlobalRadarGap(t *testing.T) radarcursor.Gap {
	t.Helper()
	gap, err := radarcursor.NewGap(
		"global-radar/head/ethereum-mainnet/evm_head",
		"ethereum-mainnet",
		"evm_head",
		101,
		104,
		"block_collect_failed",
		time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	return gap
}

func TestSaveGlobalRadarIngestGapUsesJSONEachRow(t *testing.T) {
	want := sampleGlobalRadarGap(t)
	var got globalRadarIngestGapRow
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "INSERT INTO global_radar_ingest_gaps FORMAT JSONEachRow" {
			t.Fatalf("query=%q", r.URL.Query().Get("query"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", User: "radar-user", Password: "radar-password", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SaveGlobalRadarIngestGap(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if got.GapKey != want.GapKey || got.NextHeight != 101 || got.EndHeight != 104 || got.GapVersion == 0 {
		t.Fatalf("unexpected gap row: %#v", got)
	}
}

func TestLoadOpenGlobalRadarIngestGapUsesFinalState(t *testing.T) {
	want := sampleGlobalRadarGap(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "FROM global_radar_ingest_gaps FINAL") || !strings.Contains(query, "state='open'") {
			t.Fatalf("unexpected query: %s", query)
		}
		_ = json.NewEncoder(w).Encode(globalRadarIngestGapReadRow{
			GapKey: want.GapKey, SchemaVersion: want.SchemaVersion, CursorKey: want.CursorKey,
			NetworkID: want.NetworkID, StreamKind: want.StreamKind,
			InitialStartHeight: want.InitialStartHeight, NextHeight: want.NextHeight, EndHeight: want.EndHeight,
			Reason: want.Reason, State: want.State,
			DetectedAtMillis: want.DetectedAt.UnixMilli(), UpdatedAtMillis: want.UpdatedAt.UnixMilli(), GapVersion: 99,
		})
	}))
	defer server.Close()

	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", User: "radar-user", Password: "radar-password", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := client.LoadOpenGlobalRadarIngestGap(context.Background(), want.CursorKey)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.GapKey != want.GapKey || got.NextHeight != want.NextHeight {
		t.Fatalf("unexpected gap: found=%t %#v", found, got)
	}
}

func TestVerifyGlobalRadarIngestGapSchema(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(query, "FROM system.tables"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"engine": "ReplacingMergeTree", "sorting_key": globalRadarIngestGapSortingKey,
			}}})
		case strings.Contains(query, "FROM system.columns"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"count": 15}}})
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", User: "radar-user", Password: "radar-password", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyGlobalRadarIngestGapSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
}
