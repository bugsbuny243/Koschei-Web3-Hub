package clickhouse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const trustedMigrationFixture = `
-- additive only
CREATE DATABASE IF NOT EXISTS koschei_web3;
CREATE TABLE IF NOT EXISTS koschei_web3.security_radar_stream_events
(
    event_id UUID
)
ENGINE = ReplacingMergeTree
ORDER BY event_id;
`

func TestVerifyTrustedMigrationSHA256RejectsDrift(t *testing.T) {
	digest := sha256.Sum256([]byte(trustedMigrationFixture))
	expected := hex.EncodeToString(digest[:])
	if err := VerifyTrustedMigrationSHA256(trustedMigrationFixture, expected); err != nil {
		t.Fatalf("verify trusted migration checksum: %v", err)
	}
	if err := VerifyTrustedMigrationSHA256(trustedMigrationFixture+"\n", expected); err == nil {
		t.Fatal("migration byte drift must fail checksum verification")
	}
	if err := VerifyTrustedMigrationSHA256(trustedMigrationFixture, "not-a-sha256"); err == nil {
		t.Fatal("malformed migration checksum must be rejected")
	}
}

func TestApplyTrustedMigrationRejectsDestructiveOrUnrelatedSQL(t *testing.T) {
	for name, sqlText := range map[string]string{
		"drop":           "DROP TABLE koschei_web3.security_radar_stream_events",
		"alter":          "ALTER TABLE koschei_web3.security_radar_stream_events ADD COLUMN x UInt8",
		"unrelated":      "CREATE TABLE IF NOT EXISTS koschei_web3.other_table (id UInt64) ENGINE=MergeTree ORDER BY id",
		"databaseSuffix": "CREATE DATABASE IF NOT EXISTS evil_koschei_web3; CREATE TABLE IF NOT EXISTS koschei_web3.security_radar_stream_events (event_id UUID) ENGINE=MergeTree ORDER BY event_id",
		"tableSuffix":    "CREATE TABLE IF NOT EXISTS koschei_web3.security_radar_stream_events_shadow (event_id UUID) ENGINE=MergeTree ORDER BY event_id",
	} {
		t.Run(name, func(t *testing.T) {
			client := &Client{}
			if err := client.ApplyTrustedMigration(context.Background(), sqlText); err == nil {
				t.Fatalf("expected %s SQL to be rejected", name)
			}
		})
	}
}

func TestApplyTrustedMigrationUsesAuthHeadersAndBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if r.URL.Query().Get("multiquery") != "1" {
			t.Fatal("migration request must opt into multiquery")
		}
		if got := r.Header.Get("X-ClickHouse-Database"); got != "koschei_web3" {
			t.Fatalf("database header=%q", got)
		}
		if got := r.Header.Get("X-ClickHouse-User"); got != "schema-user" {
			t.Fatalf("user header=%q", got)
		}
		if got := r.Header.Get("X-ClickHouse-Key"); got != "schema-password" {
			t.Fatal("password header missing or changed")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(body) != trustedMigrationFixture {
			t.Fatal("migration body changed")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   "koschei_web3",
		User:       "schema-user",
		Password:   "schema-password",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.ApplyTrustedMigration(context.Background(), trustedMigrationFixture); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
}

func TestVerifyStreamEventsSchemaChecksEngineSortAndTypes(t *testing.T) {
	request := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request++
		if got := r.URL.Query().Get("param_db"); got != "koschei_web3" {
			t.Fatalf("param_db=%q", got)
		}
		query := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(query, "FROM system.tables"):
			_, _ = fmt.Fprint(w, `{"data":[{"engine":"ReplacingMergeTree","sorting_key":"network, module_id, stream_mode, event_date, event_id"}]}`)
		case strings.Contains(query, "FROM system.columns"):
			if !strings.Contains(query, "decoded_sha256") || !strings.Contains(query, "raw_event_sha256") {
				t.Fatal("schema verification must require evidence payload hashes")
			}
			_, _ = fmt.Fprint(w, `{"data":[{"count":9}]}`)
		default:
			t.Fatalf("unexpected query: %s", query)
		}
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   "koschei_web3",
		User:       "schema-user",
		Password:   "schema-password",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.VerifyStreamEventsSchema(context.Background()); err != nil {
		t.Fatalf("verify schema: %v", err)
	}
	if request != 2 {
		t.Fatalf("requests=%d", request)
	}
}
