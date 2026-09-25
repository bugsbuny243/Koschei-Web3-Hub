package main

import (
	"context"
	"strings"
	"testing"

	"koschei/api/internal/radarcursor"
	"koschei/api/internal/radarevent"
	"koschei/api/internal/runtimehealth"
)

type globalRadarHeadConfigEventSink struct{}

func (globalRadarHeadConfigEventSink) InsertGlobalRadarEvents(context.Context, []radarevent.Event) error {
	return nil
}

type globalRadarHeadConfigCursorStore struct{}

func (globalRadarHeadConfigCursorStore) LoadGlobalRadarIngestCheckpoint(context.Context, string) (radarcursor.Checkpoint, bool, error) {
	return radarcursor.Checkpoint{}, false, nil
}

func (globalRadarHeadConfigCursorStore) SaveGlobalRadarIngestCheckpoint(context.Context, radarcursor.Checkpoint) error {
	return nil
}

func (globalRadarHeadConfigCursorStore) LoadGlobalRadarCanonicalCheckpointAtHeight(context.Context, string, uint64) (radarcursor.Checkpoint, bool, error) {
	return radarcursor.Checkpoint{}, false, nil
}

func TestBuildGlobalRadarHeadIngestConfigDisabledByDefault(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "")
	health := runtimehealth.New()
	config, err := buildGlobalRadarHeadIngestConfig(globalRadarHeadConfigEventSink{}, globalRadarHeadConfigCursorStore{}, health)
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

func TestBuildGlobalRadarHeadIngestConfigRequiresDurableStore(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS", "ethereum-mainnet")
	config, err := buildGlobalRadarHeadIngestConfig(nil, nil, runtimehealth.New())
	if err == nil || !strings.Contains(err.Error(), "durable checkpoint") {
		t.Fatalf("config=%v err=%v", config, err)
	}
}

func TestBuildGlobalRadarHeadIngestConfigUsesExplicitNetworksAndBounds(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS", "ethereum-mainnet,base-mainnet,bitcoin-mainnet")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_INTERVAL_SECONDS", "12")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_BLOCKS_PER_CYCLE", "7")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_EVENTS_PER_BLOCK", "9000")
	t.Setenv("ETHEREUM_RPC_URL", "https://ethereum.example")
	t.Setenv("BASE_RPC_URL", "https://base.example")
	t.Setenv("BITCOIN_CORE_RPC_URL", "https://bitcoin.example")
	health := runtimehealth.New()

	config, err := buildGlobalRadarHeadIngestConfig(globalRadarHeadConfigEventSink{}, globalRadarHeadConfigCursorStore{}, health)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Targets) != 3 {
		t.Fatalf("targets=%d want 3", len(config.Targets))
	}
	if config.Interval.String() != "12s" {
		t.Fatalf("interval=%s", config.Interval)
	}
	if config.MaxBlocksPerCycle != 7 || config.MaxEventsPerBlock != 9000 {
		t.Fatalf("bounds=%d/%d", config.MaxBlocksPerCycle, config.MaxEventsPerBlock)
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
	config, err := buildGlobalRadarHeadIngestConfig(globalRadarHeadConfigEventSink{}, globalRadarHeadConfigCursorStore{}, runtimehealth.New())
	if err == nil || !strings.Contains(err.Error(), "between 5 and 300") {
		t.Fatalf("config=%v err=%v", config, err)
	}
}

func TestBuildGlobalRadarHeadIngestConfigRejectsOversizedCycle(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS", "ethereum-mainnet")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_BLOCKS_PER_CYCLE", "65")
	t.Setenv("ETHEREUM_RPC_URL", "https://ethereum.example")
	config, err := buildGlobalRadarHeadIngestConfig(globalRadarHeadConfigEventSink{}, globalRadarHeadConfigCursorStore{}, runtimehealth.New())
	if err == nil || !strings.Contains(err.Error(), "between 1 and 64") {
		t.Fatalf("config=%v err=%v", config, err)
	}
}

func TestBuildGlobalRadarHeadIngestConfigRequiresConfirmationEndpointWhenEnabled(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS", "ethereum-mainnet")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_REQUIRE_CONFIRMATION", "1")
	t.Setenv("ETHEREUM_RPC_URL", "https://ethereum-primary.example")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_CONFIRMATION_ETHEREUM_RPC_URL", "")

	config, err := buildGlobalRadarHeadIngestConfig(globalRadarHeadConfigEventSink{}, globalRadarHeadConfigCursorStore{}, runtimehealth.New())
	if err == nil || !strings.Contains(err.Error(), "confirmation RPC endpoint") {
		t.Fatalf("config=%v err=%v", config, err)
	}
}

func TestBuildGlobalRadarHeadIngestConfigWiresIndependentConfirmationAndRecovery(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS", "ethereum-mainnet")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_REQUIRE_CONFIRMATION", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_AUTO_REORG_RECOVERY", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_REORG_REWIND", "20")
	t.Setenv("ETHEREUM_RPC_URL", "https://ethereum-primary.example")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_CONFIRMATION_ETHEREUM_RPC_URL", "https://ethereum-confirmation.example")

	config, err := buildGlobalRadarHeadIngestConfig(globalRadarHeadConfigEventSink{}, globalRadarHeadConfigCursorStore{}, runtimehealth.New())
	if err != nil {
		t.Fatal(err)
	}
	if !config.RequireConfirmation || !config.AutoReorgRecovery || config.MaxReorgRewind != 20 {
		t.Fatalf("unexpected recovery config: %#v", config)
	}
	if len(config.Targets) != 1 || config.Targets[0].Endpoint == config.Targets[0].ConfirmationEndpoint || config.Targets[0].ConfirmationEndpoint == "" {
		t.Fatalf("confirmation endpoint not independently wired: %#v", config.Targets)
	}
}
