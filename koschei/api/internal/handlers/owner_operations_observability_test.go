package handlers

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func TestOwnerDatabaseServiceStatusWithoutDatabase(t *testing.T) {
	got := ownerDatabaseServiceStatus(context.Background(), nil)
	if got["status"] != "unavailable" {
		t.Fatalf("status=%v", got["status"])
	}
	if got["query_observability"] != "unavailable" {
		t.Fatalf("query_observability=%v", got["query_observability"])
	}
	if _, ok := got["pool"]; ok {
		t.Fatalf("nil database exposed pool stats: %#v", got)
	}
}

func TestOwnerDatabaseServiceStatusPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}

	got := ownerDatabaseServiceStatus(context.Background(), db)
	if got["status"] != "connected" {
		t.Fatalf("status=%v", got["status"])
	}
	pool, ok := got["pool"].(map[string]any)
	if !ok {
		t.Fatalf("pool stats missing: %#v", got)
	}
	for _, key := range []string{"max_open_connections", "open_connections", "in_use", "idle", "wait_count", "wait_duration_ms"} {
		if _, exists := pool[key]; !exists {
			t.Fatalf("pool key %q missing: %#v", key, pool)
		}
	}
	if status, _ := got["query_observability"].(string); status != "enabled" {
		t.Fatalf("query observability status=%q want enabled: %#v", status, got)
	}
}
