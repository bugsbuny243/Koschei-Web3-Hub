package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	koscheiclickhouse "koschei/api/internal/clickhouse"
	apihttp "koschei/api/internal/http"
	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/services"
)

const globalRadarClickHouseStartupTimeout = 10 * time.Second

type globalRadarGraphStore interface {
	apihttp.GlobalRadarSnapshotSink
	apihttp.GlobalRadarGraphReader
}

func buildGlobalRadarSnapshotSink(parent context.Context) (globalRadarGraphStore, error) {
	if strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED")) != "1" {
		return nil, nil
	}

	client, err := koscheiclickhouse.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("create Global Radar ClickHouse client: %w", err)
	}

	ctx, cancel := context.WithTimeout(parent, globalRadarClickHouseStartupTimeout)
	defer cancel()
	if err := client.VerifyGlobalRadarGraphSchema(ctx); err != nil {
		return nil, fmt.Errorf("verify Global Radar ClickHouse schema: %w", err)
	}
	return client, nil
}

type globalRadarEventStore interface {
	apihttp.GlobalRadarEventSink
	apihttp.GlobalRadarEventReader
}

func buildGlobalRadarEventSink(parent context.Context) (globalRadarEventStore, error) {
	if strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_EVENT_CLICKHOUSE_ENABLED")) != "1" {
		return nil, nil
	}

	client, err := koscheiclickhouse.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("create Global Radar event ClickHouse client: %w", err)
	}

	ctx, cancel := context.WithTimeout(parent, globalRadarClickHouseStartupTimeout)
	defer cancel()
	if err := client.VerifyGlobalRadarEventSchema(ctx); err != nil {
		return nil, fmt.Errorf("verify Global Radar ClickHouse event schema: %w", err)
	}
	return client, nil
}

func buildGlobalRadarBackgroundTelemetryConfig(sink services.GlobalRadarSnapshotSink, eventSinks ...services.GlobalRadarTelemetryEventSink) (*services.GlobalRadarBackgroundTelemetryConfig, error) {
	if strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_BACKGROUND_ENABLED")) != "1" {
		return nil, nil
	}
	if sink == nil {
		return nil, fmt.Errorf("Global Radar background telemetry requires KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED=1")
	}
	rawNetworks := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_BACKGROUND_NETWORKS"))
	if rawNetworks == "" {
		return nil, fmt.Errorf("KOSCHEI_GLOBAL_RADAR_BACKGROUND_NETWORKS is required when background telemetry is enabled")
	}
	interval := 15 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_BACKGROUND_INTERVAL_SECONDS")); raw != "" {
		seconds, err := time.ParseDuration(raw + "s")
		if err != nil || seconds < 5*time.Minute || seconds > time.Hour {
			return nil, fmt.Errorf("KOSCHEI_GLOBAL_RADAR_BACKGROUND_INTERVAL_SECONDS must be between 300 and 3600")
		}
		interval = seconds
	}
	seen := map[string]bool{}
	targets := make([]services.GlobalRadarTelemetryTarget, 0)
	for _, raw := range strings.Split(rawNetworks, ",") {
		networkID := strings.ToLower(strings.TrimSpace(raw))
		if networkID == "" || seen[networkID] {
			continue
		}
		seen[networkID] = true
		switch networkID {
		case "ethereum-mainnet", "base-mainnet", "arbitrum-mainnet", "optimism-mainnet", "polygon-mainnet", "bnb-mainnet", "avalanche-mainnet":
			endpoint := globalRadarEVMEndpoint(networkID)
			if endpoint == "" {
				return nil, fmt.Errorf("%s RPC endpoint is required for Global Radar background telemetry", networkID)
			}
			targets = append(targets, services.GlobalRadarTelemetryTarget{Kind: services.GlobalRadarTelemetryEVMNode, NetworkID: networkID, Endpoint: endpoint})
			if networkID == "ethereum-mainnet" {
				if beacon := strings.TrimSpace(os.Getenv("ETHEREUM_BEACON_URL")); beacon != "" {
					targets = append(targets, services.GlobalRadarTelemetryTarget{Kind: services.GlobalRadarTelemetryEthereumBeacon, NetworkID: networkID, Endpoint: beacon})
				}
			}
		case "bitcoin-mainnet":
			endpoint := strings.TrimSpace(os.Getenv("BITCOIN_CORE_RPC_URL"))
			if endpoint == "" {
				return nil, fmt.Errorf("BITCOIN_CORE_RPC_URL is required for bitcoin-mainnet Global Radar background telemetry")
			}
			targets = append(targets,
				services.GlobalRadarTelemetryTarget{Kind: services.GlobalRadarTelemetryBitcoinCore, NetworkID: networkID, Endpoint: endpoint},
				services.GlobalRadarTelemetryTarget{Kind: services.GlobalRadarTelemetryBitcoinPoW, NetworkID: networkID, Endpoint: endpoint},
			)
		default:
			return nil, fmt.Errorf("Global Radar background telemetry does not yet support %q", networkID)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("Global Radar background telemetry has no configured targets")
	}
	var eventSink services.GlobalRadarTelemetryEventSink
	if len(eventSinks) > 0 {
		eventSink = eventSinks[0]
	}
	return &services.GlobalRadarBackgroundTelemetryConfig{Sink: sink, EventSink: eventSink, Targets: targets, Interval: interval}, nil
}

func globalRadarEVMEndpoint(networkID string) string {
	var name string
	switch networkID {
	case "ethereum-mainnet":
		name = "ETHEREUM_RPC_URL"
	case "base-mainnet":
		name = "BASE_RPC_URL"
	case "arbitrum-mainnet":
		name = "ARBITRUM_RPC_URL"
	case "optimism-mainnet":
		name = "OPTIMISM_RPC_URL"
	case "polygon-mainnet":
		name = "POLYGON_RPC_URL"
	case "bnb-mainnet":
		name = "BNB_RPC_URL"
	case "avalanche-mainnet":
		name = "AVALANCHE_RPC_URL"
	default:
		return ""
	}
	return strings.TrimSpace(os.Getenv(name))
}


func buildGlobalRadarHeadIngestConfig(eventSink services.GlobalRadarTelemetryEventSink, health *runtimehealth.Registry) (*services.GlobalRadarHeadIngestConfig, error) {
	if strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED")) != "1" {
		if health != nil {
			health.Register("worker.global-radar-head-ingest", "worker", "", false)
		}
		return nil, nil
	}
	if eventSink == nil {
		return nil, fmt.Errorf("Global Radar head ingest requires KOSCHEI_GLOBAL_RADAR_EVENT_CLICKHOUSE_ENABLED=1")
	}
	rawNetworks := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS"))
	if rawNetworks == "" {
		return nil, fmt.Errorf("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS is required when head ingest is enabled")
	}
	interval := 15 * time.Second
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_INTERVAL_SECONDS")); raw != "" {
		parsed, err := time.ParseDuration(raw + "s")
		if err != nil || parsed < 5*time.Second || parsed > 5*time.Minute {
			return nil, fmt.Errorf("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_INTERVAL_SECONDS must be between 5 and 300")
		}
		interval = parsed
	}
	seen := map[string]bool{}
	targets := make([]services.GlobalRadarHeadIngestTarget, 0)
	for _, raw := range strings.Split(rawNetworks, ",") {
		networkID := strings.ToLower(strings.TrimSpace(raw))
		if networkID == "" || seen[networkID] {
			continue
		}
		seen[networkID] = true
		switch networkID {
		case "ethereum-mainnet", "base-mainnet", "arbitrum-mainnet", "optimism-mainnet", "polygon-mainnet", "bnb-mainnet", "avalanche-mainnet":
			endpoint := globalRadarEVMEndpoint(networkID)
			if endpoint == "" {
				return nil, fmt.Errorf("%s RPC endpoint is required for Global Radar head ingest", networkID)
			}
			targets = append(targets, services.GlobalRadarHeadIngestTarget{Kind: services.GlobalRadarHeadIngestEVM, NetworkID: networkID, Endpoint: endpoint})
		case "bitcoin-mainnet":
			endpoint := strings.TrimSpace(os.Getenv("BITCOIN_CORE_RPC_URL"))
			if endpoint == "" {
				return nil, fmt.Errorf("BITCOIN_CORE_RPC_URL is required for bitcoin-mainnet Global Radar head ingest")
			}
			targets = append(targets, services.GlobalRadarHeadIngestTarget{Kind: services.GlobalRadarHeadIngestBitcoin, NetworkID: networkID, Endpoint: endpoint})
		default:
			return nil, fmt.Errorf("Global Radar head ingest does not yet support %q", networkID)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("Global Radar head ingest has no configured targets")
	}
	if health != nil {
		health.Register("worker.global-radar-head-ingest", "worker", "", true)
		health.Register("global-radar.head.event-sink", "storage", "", true)
		for _, target := range targets {
			health.Register(services.GlobalRadarHeadIngestTargetHealthID(target), "head_ingest", target.NetworkID, true)
		}
	}
	return &services.GlobalRadarHeadIngestConfig{EventSink: eventSink, Targets: targets, Interval: interval, Health: health}, nil
}
