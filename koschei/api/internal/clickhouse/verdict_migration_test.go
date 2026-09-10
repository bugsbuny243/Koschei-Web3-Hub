package clickhouse

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const validVerdictMigrationFixture = `CREATE TABLE IF NOT EXISTS koschei_web3.arvis_verdict_snapshots
(
    verdict_id UUID
)
ENGINE = MergeTree
ORDER BY verdict_id;`

func TestValidateTrustedVerdictShadowMigrationRejectsDestructiveAndUnrelatedSQL(t *testing.T) {
	if err := validateTrustedVerdictShadowMigration(validVerdictMigrationFixture); err != nil {
		t.Fatalf("valid additive migration rejected: %v", err)
	}
	for _, candidate := range []string{
		"DROP TABLE koschei_web3.arvis_verdict_snapshots;",
		"ALTER TABLE koschei_web3.arvis_verdict_snapshots ADD COLUMN x String;",
		"CREATE TABLE IF NOT EXISTS koschei_web3.other_table (id UInt64) ENGINE=MergeTree ORDER BY id;",
		validVerdictMigrationFixture + "\nCREATE TABLE IF NOT EXISTS koschei_web3.other_table (id UInt64) ENGINE=MergeTree ORDER BY id;",
	} {
		if err := validateTrustedVerdictShadowMigration(candidate); err == nil {
			t.Fatalf("unsafe verdict migration unexpectedly accepted: %q", candidate)
		}
	}
}

func TestApplyTrustedVerdictShadowMigrationUsesExistingSecureHeaders(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-ClickHouse-Database") != "koschei_web3" || r.Header.Get("X-ClickHouse-User") != "schema-user" || r.Header.Get("X-ClickHouse-Key") != "schema-password" {
			t.Fatal("schema request auth/database headers missing or changed")
		}
		if r.URL.Query().Get("multiquery") != "1" {
			t.Fatal("schema request must opt into multiquery")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read migration body: %v", err)
		}
		if string(body) != validVerdictMigrationFixture {
			t.Fatalf("migration body changed: %q", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", User: "schema-user", Password: "schema-password", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.ApplyTrustedVerdictShadowMigration(context.Background(), validVerdictMigrationFixture); err != nil {
		t.Fatalf("apply verdict migration: %v", err)
	}
}

func TestVerifyVerdictShadowSchemaChecksEngineSortingAndColumns(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(query, "FROM system.tables"):
			_, _ = w.Write([]byte(`{"data":[{"engine":"ReplacingMergeTree","sorting_key":"module_id, verdict_id"}]}`))
		case strings.Contains(query, "FROM system.columns"):
			_, _ = w.Write([]byte(`{"data":[{"count":10}]}`))
		default:
			t.Fatalf("unexpected schema verification query: %q", query)
		}
	}))
	defer server.Close()
	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", User: "schema-user", Password: "schema-password", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.VerifyVerdictShadowSchema(context.Background()); err != nil {
		t.Fatalf("verify verdict schema: %v", err)
	}
}
