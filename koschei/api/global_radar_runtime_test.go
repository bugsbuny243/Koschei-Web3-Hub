package main

import (
	"context"
	"strings"
	"testing"
)

func TestBuildGlobalRadarSnapshotSinkDisabledByDefault(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED", "")
	sink, err := buildGlobalRadarSnapshotSink(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sink != nil {
		t.Fatal("Global Radar ClickHouse sink must remain disabled by default")
	}
}

func TestBuildGlobalRadarSnapshotSinkFailsClosedWhenEnabledWithoutConfiguration(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED", "1")
	t.Setenv("CLICKHOUSE_HTTP_URL", "")
	t.Setenv("CLICKHOUSE_PASSWORD", "")
	sink, err := buildGlobalRadarSnapshotSink(context.Background())
	if err == nil || !strings.Contains(err.Error(), "CLICKHOUSE_HTTP_URL is required") {
		t.Fatalf("sink=%v err=%v", sink, err)
	}
}

func TestBuildGlobalRadarEventSinkDisabledByDefault(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_EVENT_CLICKHOUSE_ENABLED", "")
	sink, err := buildGlobalRadarEventSink(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sink != nil {
		t.Fatal("Global Radar event ClickHouse sink must remain disabled by default")
	}
}

func TestBuildGlobalRadarEventSinkFailsClosedWhenEnabledWithoutConfiguration(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_EVENT_CLICKHOUSE_ENABLED", "1")
	t.Setenv("CLICKHOUSE_HTTP_URL", "")
	t.Setenv("CLICKHOUSE_PASSWORD", "")
	sink, err := buildGlobalRadarEventSink(context.Background())
	if err == nil || !strings.Contains(err.Error(), "CLICKHOUSE_HTTP_URL is required") {
		t.Fatalf("sink=%v err=%v", sink, err)
	}
}
