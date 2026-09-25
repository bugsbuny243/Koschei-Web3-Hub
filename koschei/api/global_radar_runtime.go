package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	koscheiclickhouse "koschei/api/internal/clickhouse"
	apihttp "koschei/api/internal/http"
	"koschei/api/internal/radarcursor"
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
	radarcursor.RecoveryStore
	VerifyGlobalRadarIngestCheckpointSchema(context.Context) error
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
	if err := client.VerifyGlobalRadarIngestCheckpointSchema(ctx); err != nil {
		return nil, fmt.Errorf("verify Global Radar ClickHouse ingest checkpoint schema: %w", err)
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

func globalRadarConfirmationEndpoint(networkID string) string {
	var name string
	switch networkID {
	case "ethereum-mainnet":
		name = "KOSCHEI_GLOBAL_RADAR_CONFIRMATION_ETHEREUM_RPC_URL"
	case "base-mainnet":
		name = "KOSCHEI_GLOBAL_RADAR_CONFIRMATION_BASE_RPC_URL"
	case "arbitrum-mainnet":
		name = "KOSCHEI_GLOBAL_RADAR_CONFIRMATION_ARBITRUM_RPC_URL"
	case "optimism-mainnet":
		name = "KOSCHEI_GLOBAL_RADAR_CONFIRMATION_OPTIMISM_RPC_URL"
	case "polygon-mainnet":
		name = "KOSCHEI_GLOBAL_RADAR_CONFIRMATION_POLYGON_RPC_URL"
	case "bnb-mainnet":
		name = "KOSCHEI_GLOBAL_RADAR_CONFIRMATION_BNB_RPC_URL"
	case "avalanche-mainnet":
		name = "KOSCHEI_GLOBAL_RADAR_CONFIRMATION_AVALANCHE_RPC_URL"
	case "bitcoin-mainnet":
		name = "KOSCHEI_GLOBAL_RADAR_CONFIRMATION_BITCOIN_CORE_RPC_URL"
	default:
		return ""
	}
	return strings.TrimSpace(os.Getenv(name))
}

func buildGlobalRadarHeadIngestConfig(eventSink services.GlobalRadarTelemetryEventSink, cursorStore radarcursor.RecoveryStore, health *runtimehealth.Registry) (*services.GlobalRadarHeadIngestConfig, error) {
	if strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED")) != "1" {
		if health != nil {
			health.Register("worker.global-radar-head-ingest", "worker", "", false)
		}
		return nil, nil
	}
	if eventSink == nil || cursorStore == nil {
		return nil, fmt.Errorf("Global Radar head ingest requires KOSCHEI_GLOBAL_RADAR_EVENT_CLICKHOUSE_ENABLED=1 with durable checkpoint storage")
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
	maxBlocksPerCycle := 4
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_BLOCKS_PER_CYCLE")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 64 {
			return nil, fmt.Errorf("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_BLOCKS_PER_CYCLE must be between 1 and 64")
		}
		maxBlocksPerCycle = parsed
	}
	maxEventsPerBlock := 6000
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_EVENTS_PER_BLOCK")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 25000 {
			return nil, fmt.Errorf("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_EVENTS_PER_BLOCK must be between 1 and 25000")
		}
		maxEventsPerBlock = parsed
	}
	requireConfirmation := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_REQUIRE_CONFIRMATION")) == "1"
	autoReorgRecovery := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_AUTO_REORG_RECOVERY")) == "1"
	maxReorgRewind := 12
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_REORG_REWIND")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 64 {
			return nil, fmt.Errorf("KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_REORG_REWIND must be between 1 and 64")
		}
		maxReorgRewind = parsed
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
			confirmation := globalRadarConfirmationEndpoint(networkID)
			if (requireConfirmation || autoReorgRecovery) && confirmation == "" {
				return nil, fmt.Errorf("%s confirmation RPC endpoint is required when confirmation or auto reorg recovery is enabled", networkID)
			}
			targets = append(targets, services.GlobalRadarHeadIngestTarget{Kind: services.GlobalRadarHeadIngestEVM, NetworkID: networkID, Endpoint: endpoint, ConfirmationEndpoint: confirmation})
		case "bitcoin-mainnet":
			endpoint := strings.TrimSpace(os.Getenv("BITCOIN_CORE_RPC_URL"))
			if endpoint == "" {
				return nil, fmt.Errorf("BITCOIN_CORE_RPC_URL is required for bitcoin-mainnet Global Radar head ingest")
			}
			confirmation := globalRadarConfirmationEndpoint(networkID)
			if (requireConfirmation || autoReorgRecovery) && confirmation == "" {
				return nil, fmt.Errorf("bitcoin-mainnet confirmation RPC endpoint is required when confirmation or auto reorg recovery is enabled")
			}
			targets = append(targets, services.GlobalRadarHeadIngestTarget{Kind: services.GlobalRadarHeadIngestBitcoin, NetworkID: networkID, Endpoint: endpoint, ConfirmationEndpoint: confirmation})
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
	return &services.GlobalRadarHeadIngestConfig{
		EventSink: eventSink, CursorStore: cursorStore, Targets: targets, Interval: interval, Health: health,
		MaxBlocksPerCycle: maxBlocksPerCycle, MaxEventsPerBlock: maxEventsPerBlock,
		RequireConfirmation: requireConfirmation, AutoReorgRecovery: autoReorgRecovery, MaxReorgRewind: uint64(maxReorgRewind),
	}, nil
}
