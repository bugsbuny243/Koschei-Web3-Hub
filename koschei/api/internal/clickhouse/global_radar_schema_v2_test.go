package clickhouse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGlobalRadarGraphV2OptInIsExplicitAndSharedByReaders(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_GRAPH_V2_ENABLED", "")
	if got := globalRadarGraphTableName(); got != "global_radar_graph_records" {
		t.Fatalf("default changed legacy graph table: %q", got)
	}
	if got := globalRadarGraphQuery("FROM global_radar_graph_records FINAL"); got != "FROM global_radar_graph_records FINAL" {
		t.Fatalf("default graph reader unexpectedly changed: %q", got)
	}
	t.Setenv("KOSCHEI_GLOBAL_RADAR_GRAPH_V2_ENABLED", "1")
	if got := globalRadarGraphTableName(); got != "global_radar_graph_records_v2" {
		t.Fatalf("V2 explicitly enabled but not selected: %q", got)
	}
	if got := globalRadarGraphQuery("FROM global_radar_graph_records FINAL"); got != "FROM global_radar_graph_records_v2 FINAL" {
		t.Fatalf("graph read/replay table was not switched: %q", got)
	}
	if !strings.Contains(globalRadarGraphExpectedSortingKey(), "record_id, snapshot_sha256, payload_sha256") {
		t.Fatal("V2 sorting key must include exact snapshot identity")
	}
	t.Setenv("KOSCHEI_GLOBAL_RADAR_GRAPH_V2_ENABLED", "true")
	if got := globalRadarGraphTableName(); got != "global_radar_graph_records" {
		t.Fatalf("non-explicit V2 value must not activate it: %q", got)
	}
}

func TestGlobalRadarGraphV2SchemaRejectsLegacyDedupKey(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_GRAPH_V2_ENABLED", "1")
	sortingKey := globalRadarGraphSortingKey
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("param_table") != "global_radar_graph_records_v2" {
			t.Errorf("V2 schema metadata queried wrong table")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Query().Get("query"), "FROM system.tables"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"engine": "ReplacingMergeTree", "sorting_key": sortingKey}},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"count": 18}}})
		}
	}))
	defer server.Close()
	client, err := New(Config{HTTPURL: server.URL, Database: DefaultDatabase, User: "default", Password: "test", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyGlobalRadarGraphSchema(context.Background()); err == nil || !strings.Contains(err.Error(), "sorting_key") {
		t.Fatalf("V2 must fail closed on legacy snapshot-collapsing key: %v", err)
	}
	sortingKey = globalRadarGraphV2SortingKey
	if err := client.VerifyGlobalRadarGraphSchema(context.Background()); err != nil {
		t.Fatalf("V2 with proper snapshot isolation should be accepted: %v", err)
	}
}

func TestGlobalRadarGraphV2MigrationIsAdditive(t *testing.T) {
	statement := `CREATE DATABASE IF NOT EXISTS koschei_web3;
CREATE TABLE IF NOT EXISTS koschei_web3.global_radar_graph_records_v2
(snapshot_sha256 FixedString(64), ingest_version UInt64)
ENGINE = ReplacingMergeTree(ingest_version)
ORDER BY (snapshot_sha256)`
	if err := validateTrustedGlobalRadarMigration(statement); err != nil {
		t.Fatalf("V2 table should be in the trusted additive migration allowlist: %v", err)
	}
}
