package main

import (
	"testing"
	"time"
)

func TestFirstNonEmptyEnv(t *testing.T) {
	t.Setenv("DATABASE_READ_URL", "")
	t.Setenv("DATABASE_URL", "postgres://fallback")
	if got := firstNonEmptyEnv("DATABASE_READ_URL", "DATABASE_URL"); got != "postgres://fallback" {
		t.Fatalf("fallback env=%q", got)
	}

	t.Setenv("DATABASE_READ_URL", "postgres://read")
	if got := firstNonEmptyEnv("DATABASE_READ_URL", "DATABASE_URL"); got != "postgres://read" {
		t.Fatalf("preferred env=%q", got)
	}
}

func TestPublishTimeoutDefaultAndBounds(t *testing.T) {
	t.Setenv("KOSCHEI_PUBLIC_REGISTRY_PUBLISH_TIMEOUT_SECONDS", "")
	got, err := publishTimeout()
	if err != nil {
		t.Fatalf("default timeout: %v", err)
	}
	if got != defaultPublishTimeoutSeconds*time.Second {
		t.Fatalf("default timeout=%s", got)
	}

	t.Setenv("KOSCHEI_PUBLIC_REGISTRY_PUBLISH_TIMEOUT_SECONDS", "90")
	got, err = publishTimeout()
	if err != nil {
		t.Fatalf("configured timeout: %v", err)
	}
	if got != 90*time.Second {
		t.Fatalf("configured timeout=%s", got)
	}

	for _, bad := range []string{"9", "601", "bad"} {
		t.Setenv("KOSCHEI_PUBLIC_REGISTRY_PUBLISH_TIMEOUT_SECONDS", bad)
		if _, err := publishTimeout(); err == nil {
			t.Fatalf("expected timeout %q to fail", bad)
		}
	}
}

func TestRunFailsBeforeNetworkWhenRequiredConfigurationIsMissing(t *testing.T) {
	t.Setenv("DATABASE_READ_URL", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("GOOGLE_DRIVE_ARCHIVE_FOLDER_ID", "")
	t.Setenv("GOOGLE_DRIVE_SERVICE_ACCOUNT_JSON", "")
	if err := run(); err == nil {
		t.Fatal("expected missing database configuration to fail")
	}
}
