package clickhouse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifyGlobalRadarGraphSchemaAcceptsCanonicalTable(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(query, "FROM system.tables"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"engine":      "ReplacingMergeTree",
					"sorting_key": globalRadarGraphSortingKey,
				}},
			})
		case strings.Contains(query, "FROM system.columns"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"count": 18}},
			})
		default:
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   DefaultDatabase,
		User:       "default",
		Password:   "test-secret",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyGlobalRadarGraphSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyGlobalRadarGraphSchemaRejectsIncompleteColumns(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(query, "FROM system.tables") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"engine":      "ReplacingMergeTree",
					"sorting_key": globalRadarGraphSortingKey,
				}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"count": 17}},
		})
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   DefaultDatabase,
		User:       "default",
		Password:   "test-secret",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = client.VerifyGlobalRadarGraphSchema(context.Background())
	if err == nil || !strings.Contains(err.Error(), "17 of 18") {
		t.Fatalf("unexpected error: %v", err)
	}
}
