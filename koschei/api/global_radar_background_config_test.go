package main

import (
	"context"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

type globalRadarBackgroundTestSink struct{}

func (globalRadarBackgroundTestSink) InsertGlobalRadarSnapshot(context.Context, services.GlobalRadarSnapshot) error { return nil }

func TestBuildGlobalRadarBackgroundTelemetryConfigDisabledByDefault(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_BACKGROUND_ENABLED", "")
	config, err := buildGlobalRadarBackgroundTelemetryConfig(globalRadarBackgroundTestSink{})
	if err != nil { t.Fatal(err) }
	if config != nil { t.Fatal("background telemetry must remain disabled by default") }
}

func TestBuildGlobalRadarBackgroundTelemetryConfigRequiresDurableSink(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_BACKGROUND_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_BACKGROUND_NETWORKS", "ethereum-mainnet")
	config, err := buildGlobalRadarBackgroundTelemetryConfig(nil)
	if err == nil || !strings.Contains(err.Error(), "CLICKHOUSE") {
		t.Fatalf("config=%v err=%v", config, err)
	}
}

func TestBuildGlobalRadarBackgroundTelemetryConfigUsesExplicitNetworks(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_RADAR_BACKGROUND_ENABLED", "1")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_BACKGROUND_NETWORKS", "ethereum-mainnet,bitcoin-mainnet")
	t.Setenv("ETHEREUM_RPC_URL", "https://ethereum.example")
	t.Setenv("ETHEREUM_BEACON_URL", "https://beacon.example")
	t.Setenv("BITCOIN_CORE_RPC_URL", "https://bitcoin.example")
	t.Setenv("KOSCHEI_GLOBAL_RADAR_BACKGROUND_INTERVAL_SECONDS", "900")
	config, err := buildGlobalRadarBackgroundTelemetryConfig(globalRadarBackgroundTestSink{})
	if err != nil { t.Fatal(err) }
	if len(config.Targets) != 4 {
		t.Fatalf("targets=%d want 4", len(config.Targets))
	}
}
