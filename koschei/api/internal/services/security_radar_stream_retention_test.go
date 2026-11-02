package services

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func TestStreamRetentionDisabledByDefault(t *testing.T) {
	t.Setenv("KOSCHEI_STREAM_RETENTION_ENABLED", "false")
	t.Setenv("KOSCHEI_STREAM_RETENTION_DAYS", "")
	policy := securityRadarStreamRetentionPolicy()
	if policy.Enabled || policy.Days != 14 {
		t.Fatalf("unexpected default retention policy: %+v", policy)
	}
	affected, err := RunSecurityRadarStreamRetention(context.Background(), nil)
	if err != nil || affected != 0 {
		t.Fatalf("disabled retention must not touch DB: affected=%d err=%v", affected, err)
	}
}

func TestStreamRetentionDaysInvalidFallsBack(t *testing.T) {
	t.Setenv("KOSCHEI_STREAM_RETENTION_ENABLED", "false")
	t.Setenv("KOSCHEI_STREAM_RETENTION_DAYS", "-1")
	if got := securityRadarStreamRetentionPolicy().Days; got != 14 {
		t.Fatalf("invalid retention days must fall back to 14, got %d", got)
	}
}

func TestStreamRetentionRequiresParityApproval(t *testing.T) {
	t.Setenv("KOSCHEI_STREAM_RETENTION_ENABLED", "true")
	t.Setenv("KOSCHEI_STREAM_RETENTION_PARITY_APPROVED", "false")
	if securityRadarStreamRetentionPolicy().Enabled {
		t.Fatal("retention must stay disabled until parity approval")
	}
	affected, err := RunSecurityRadarStreamRetention(context.Background(), nil)
	if err != nil || affected != 0 {
		t.Fatalf("unapproved retention must not access database: affected=%d err=%v", affected, err)
	}
}

func TestStreamRetentionParityFlagsDoNotBypassExternalArchiveProof(t *testing.T) {
	t.Setenv("KOSCHEI_STREAM_RETENTION_ENABLED", "true")
	t.Setenv("KOSCHEI_STREAM_RETENTION_PARITY_APPROVED", "true")
	if !securityRadarStreamRetentionPolicy().Enabled {
		t.Fatal("synthetic parity flags must reach the external archive proof gate")
	}
	// sql.Open does not connect. If this test reaches the old DELETE query,
	// the unreachable test endpoint causes a different error; no production
	// database or deletion is accessed.
	db, err := sql.Open("postgres", "postgres://test:test@127.0.0.1:1/not_a_db?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	affected, err := RunSecurityRadarStreamRetention(context.Background(), db)
	if affected != 0 || err == nil ||
		!strings.Contains(err.Error(), "external_archive_proof_before_source_delete_not_wired") {
		t.Fatalf("parity flags bypassed external archive proof: affected=%d err=%v", affected, err)
	}
}
