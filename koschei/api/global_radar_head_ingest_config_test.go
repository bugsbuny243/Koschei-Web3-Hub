package main

import (
	"context"
	"strings"
	"testing"

	"koschei/api/internal/radarevent"
	"koschei/api/internal/runtimehealth"
)

type globalRadarHeadConfigEventSink struct{}

func (globalRadarHeadConfigEventSink) InsertGlobalRadarEvents(context.Context, []radarevent.Event) error {
	return nil
}

func TestBuildGlobalRadarHeadIngestConfigDisabledByDefault(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "")
	health := runtimehealth.New()
	config, err := buildGlobalRadarHeadIngestConfig(globalRadarHeadConfigEventSink{}, health)
	if err != nil {
		t.Fatal(err)
	}
	if config != nil {
		t.Fatal("head ingest must remain disabled by default")
	}
	snapshot := health.Snapshot()
	if len(snapshot.Entries) != 1 || snapshot.Entries[0].State != runtimehealth.StateDisabled {
		t.Fatalf("unexpected disabled health: %#v", snapshot)
	}
}

func TestBuildGlobalRadarHeadIngestConfigRequiresEventSink(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS", "ethereum-mainnet")
	config, err := buildGlobalRadarHeadIngestConfig(nil, runtimehealth.New())
	if err == nil || !strings.Contains(err.Error(), "EVENT_CLICKHOUSE") {
		t.Fatalf("config=%v err=%v", config, err)
	}
}

func TestBuildGlobalRadarHeadIngestConfigUsesExplicitNetworks(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS", "ethereum-mainnet,base-mainnet,bitcoin-mainnet")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_INTERVAL_SECONDS", "12")
	t.Setenv("ETHEREUM_RPC_URL", "https://ethereum.example")
	t.Setenv("BASE_RPC_URL", "https://base.example")
	t.Setenv("BITCOIN_CORE_RPC_URL", "https://bitcoin.example")
	health := runtimehealth.New()

	config, err := buildGlobalRadarHeadIngestConfig(globalRadarHeadConfigEventSink{}, health)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Targets) != 3 {
		t.Fatalf("targets=%d want 3", len(config.Targets))
	}
	if config.Interval.String() != "12s" {
		t.Fatalf("interval=%s", config.Interval)
	}
	if len(health.Snapshot().Entries) < 5 {
		t.Fatalf("expected worker, sink and target health entries: %#v", health.Snapshot())
	}
}

func TestBuildGlobalRadarHeadIngestConfigRejectsTooFastCadence(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS", "ethereum-mainnet")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_INTERVAL_SECONDS", "1")
	t.Setenv("ETHEREUM_RPC_URL", "https://ethereum.example")
	config, err := buildGlobalRadarHeadIngestConfig(globalRadarHeadConfigEventSink{}, runtimehealth.New())
	if err == nil || !strings.Contains(err.Error(), "between 5 and 300") {
		t.Fatalf("config=%v err=%v", config, err)
	}
}
