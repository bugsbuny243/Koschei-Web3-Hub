package services

import (
	"context"
	"testing"
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
