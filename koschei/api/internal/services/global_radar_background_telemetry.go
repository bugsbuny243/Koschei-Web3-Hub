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
		observation, event, err := collectGlobalRadarTelemetryTarget(ctx, cfg.HTTPClient, target, now)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", strings.TrimSpace(target.NetworkID), strings.TrimSpace(target.Kind), err))
			continue
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
		return GlobalRadarSnapshot{}, fmt.Errorf("persist global radar telemetry snapshot: %w", err)
	}
	if cfg.EventSink != nil && len(events) > 0 {
		if err := cfg.EventSink.InsertGlobalRadarEvents(ctx, events); err != nil {
			return snapshot, fmt.Errorf("persist global radar telemetry events: %w", err)
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
		return observation, nil, err
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
		return observation, nil, err
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
	workerCtx, cancel := context.WithCancel(ctx)
	go func() {
		run := func() {
			cycleCtx, cycleCancel := context.WithTimeout(workerCtx, 2*time.Minute)
			defer cycleCancel()
			snapshot, err := CollectGlobalRadarBackgroundTelemetry(cycleCtx, cfg)
			if err != nil {
				if len(snapshot.Observations) > 0 {
					log.Printf("global radar background telemetry partial cycle persisted: observations=%d error=%v", len(snapshot.Observations), err)
				} else {
					log.Printf("global radar background telemetry cycle failed: %v", err)
				}
				return
			}
			log.Printf("global radar background telemetry cycle persisted: networks=%d observations=%d", snapshot.Coverage.NetworkCount, snapshot.Coverage.ObservationCount)
		}
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
	return cancel
}
