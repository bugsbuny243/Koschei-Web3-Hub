package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/radarevent"
	"koschei/api/internal/runtimehealth"
)

const (
	GlobalRadarTelemetryEVMNode        = "evm_node"
	GlobalRadarTelemetryEthereumBeacon = "ethereum_beacon"
	GlobalRadarTelemetryBitcoinCore    = "bitcoin_core_node"
	GlobalRadarTelemetryBitcoinPoW     = "bitcoin_pow_network"
)

type GlobalRadarSnapshotSink interface {
	InsertGlobalRadarSnapshot(context.Context, GlobalRadarSnapshot) error
}

type GlobalRadarTelemetryEventSink interface {
	InsertGlobalRadarEvents(context.Context, []radarevent.Event) error
}

type GlobalRadarTelemetryTarget struct {
	Kind      string
	NetworkID string
	Endpoint  string
}

type GlobalRadarBackgroundTelemetryConfig struct {
	Sink       GlobalRadarSnapshotSink
	EventSink  GlobalRadarTelemetryEventSink
	Targets    []GlobalRadarTelemetryTarget
	Interval   time.Duration
	HTTPClient *http.Client
	Now        func() time.Time
	Health     *runtimehealth.Registry
}

func CollectGlobalRadarBackgroundTelemetry(ctx context.Context, cfg GlobalRadarBackgroundTelemetryConfig) (GlobalRadarSnapshot, error) {
	if cfg.Sink == nil {
		return GlobalRadarSnapshot{}, fmt.Errorf("global radar telemetry sink is required")
	}
	if len(cfg.Targets) == 0 {
		return GlobalRadarSnapshot{}, fmt.Errorf("global radar telemetry targets are required")
	}
	now := time.Now().UTC()
	if cfg.Now != nil {
		now = cfg.Now().UTC()
	}
	observations := make([]GlobalRadarObservation, 0, len(cfg.Targets))
	events := make([]radarevent.Event, 0, len(cfg.Targets))
	errs := make([]error, 0)
	for _, target := range cfg.Targets {
		healthID := "global-radar.telemetry." + strings.ToLower(strings.TrimSpace(target.NetworkID)) + "." + strings.ToLower(strings.TrimSpace(target.Kind))
		if cfg.Health != nil {
			cfg.Health.Register(healthID, "network_telemetry", target.NetworkID, true)
		}
		observation, event, err := collectGlobalRadarTelemetryTarget(ctx, cfg.HTTPClient, target, now)
		if err != nil {
			wrapped := fmt.Errorf("%s/%s: %w", strings.TrimSpace(target.NetworkID), strings.TrimSpace(target.Kind), err)
			errs = append(errs, wrapped)
			if cfg.Health != nil {
				cfg.Health.Failure(healthID, wrapped)
			}
			continue
		}
		if cfg.Health != nil {
			cfg.Health.Success(healthID, 1)
		}
		observations = append(observations, observation)
		if event != nil {
			events = append(events, *event)
		}
	}
	if len(observations) == 0 {
		if len(errs) == 0 {
			return GlobalRadarSnapshot{}, fmt.Errorf("global radar telemetry cycle produced no observations")
		}
		return GlobalRadarSnapshot{}, errors.Join(errs...)
	}
	snapshot, err := BuildGlobalRadarSnapshot(observations, nil, nil, nil, now)
	if err != nil {
		return GlobalRadarSnapshot{}, err
	}
	if err := cfg.Sink.InsertGlobalRadarSnapshot(ctx, snapshot); err != nil {
		if cfg.Health != nil {
			cfg.Health.Failure("global-radar.telemetry.snapshot-sink", err)
		}
		return GlobalRadarSnapshot{}, fmt.Errorf("persist global radar telemetry snapshot: %w", err)
	}
	if cfg.Health != nil {
		cfg.Health.Success("global-radar.telemetry.snapshot-sink", len(observations))
	}
	if cfg.EventSink != nil && len(events) > 0 {
		if err := cfg.EventSink.InsertGlobalRadarEvents(ctx, events); err != nil {
			if cfg.Health != nil {
				cfg.Health.Failure("global-radar.telemetry.event-sink", err)
			}
			return snapshot, fmt.Errorf("persist global radar telemetry events: %w", err)
		}
		if cfg.Health != nil {
			cfg.Health.Success("global-radar.telemetry.event-sink", len(events))
		}
	}
	if len(errs) > 0 {
		return snapshot, errors.Join(errs...)
	}
	return snapshot, nil
}

func collectGlobalRadarTelemetryTarget(ctx context.Context, client *http.Client, target GlobalRadarTelemetryTarget, observedAt time.Time) (GlobalRadarObservation, *radarevent.Event, error) {
	kind := strings.ToLower(strings.TrimSpace(target.Kind))
	networkID := strings.ToLower(strings.TrimSpace(target.NetworkID))
	endpoint := strings.TrimSpace(target.Endpoint)
	if endpoint == "" {
		return GlobalRadarObservation{}, nil, fmt.Errorf("telemetry endpoint is required")
	}
	switch kind {
	case GlobalRadarTelemetryEVMNode:
		result, err := networktarget.ProbeEVMNodeTelemetry(ctx, client, endpoint, networkID, observedAt)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		observation, err := ProjectEVMNodeTelemetryToGlobalRadar(result)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		event, err := radarevent.BuildEVMNodeTelemetryEventFromResult("evm-node-telemetry-adapter", result)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		return observation, &event, nil
	case GlobalRadarTelemetryEthereumBeacon:
		if networkID != "ethereum-mainnet" {
			return GlobalRadarObservation{}, nil, fmt.Errorf("ethereum beacon telemetry requires ethereum-mainnet")
		}
		result, err := networktarget.ProbeEthereumBeaconTelemetry(ctx, client, endpoint, observedAt)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		observation, err := ProjectEthereumBeaconTelemetryToGlobalRadar(result)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		event, err := radarevent.BuildEthereumBeaconTelemetryEventFromResult("ethereum-beacon-telemetry-adapter", result)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		return observation, &event, nil
	case GlobalRadarTelemetryBitcoinCore:
		if networkID != "bitcoin-mainnet" {
			return GlobalRadarObservation{}, nil, fmt.Errorf("bitcoin core telemetry requires bitcoin-mainnet")
		}
		result, err := networktarget.ProbeBitcoinCoreNodeTelemetry(ctx, client, endpoint, observedAt)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		observation, err := ProjectBitcoinCoreNodeTelemetryToGlobalRadar(result)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		event, err := radarevent.BuildBitcoinCoreNodeTelemetryEventFromResult("bitcoin-core-telemetry-adapter", result)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		return observation, &event, nil
	case GlobalRadarTelemetryBitcoinPoW:
		if networkID != "bitcoin-mainnet" {
			return GlobalRadarObservation{}, nil, fmt.Errorf("bitcoin PoW telemetry requires bitcoin-mainnet")
		}
		result, err := networktarget.ProbeBitcoinPoWNetworkTelemetry(ctx, client, endpoint, observedAt)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		observation, err := ProjectBitcoinPoWNetworkTelemetryToGlobalRadar(result)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		event, err := radarevent.BuildBitcoinPoWNetworkTelemetryEventFromResult("bitcoin-pow-telemetry-adapter", result)
		if err != nil {
			return GlobalRadarObservation{}, nil, err
		}
		return observation, &event, nil
	default:
		return GlobalRadarObservation{}, nil, fmt.Errorf("unsupported global radar telemetry kind %q", kind)
	}
}

func StartGlobalRadarBackgroundTelemetry(ctx context.Context, cfg GlobalRadarBackgroundTelemetryConfig) func() {
	if cfg.Sink == nil || len(cfg.Targets) == 0 {
		return func() {}
	}
	interval := cfg.Interval
	if interval < 5*time.Minute {
		interval = 5 * time.Minute
	}
	if interval > time.Hour {
		interval = time.Hour
	}
	if cfg.Health != nil {
		cfg.Health.Register("worker.global-radar-background-telemetry", "worker", "", true)
		cfg.Health.Register("global-radar.telemetry.snapshot-sink", "storage", "", true)
		if cfg.EventSink != nil {
			cfg.Health.Register("global-radar.telemetry.event-sink", "storage", "", true)
		}
	}
	workerCtx, cancel := context.WithCancel(ctx)
	go func() {
		run := func() {
			cycleCtx, cycleCancel := context.WithTimeout(workerCtx, 2*time.Minute)
			defer cycleCancel()
			snapshot, err := CollectGlobalRadarBackgroundTelemetry(cycleCtx, cfg)
			if err != nil {
				if cfg.Health != nil {
					cfg.Health.Failure("worker.global-radar-background-telemetry", err)
				}
				if len(snapshot.Observations) > 0 {
					log.Printf("global radar background telemetry partial cycle persisted: observations=%d error=%v", len(snapshot.Observations), err)
				} else {
					log.Printf("global radar background telemetry cycle failed: %v", err)
				}
				return
			}
			if cfg.Health != nil {
				cfg.Health.Success("worker.global-radar-background-telemetry", snapshot.Coverage.ObservationCount)
			}
			log.Printf("global radar background telemetry cycle persisted: networks=%d observations=%d", snapshot.Coverage.NetworkCount, snapshot.Coverage.ObservationCount)
		}
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				if cfg.Health != nil {
					cfg.Health.Stop("worker.global-radar-background-telemetry")
				}
				return
			case <-ticker.C:
				run()
			}
		}
	}()
	return cancel
}
