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
	GlobalRadarHeadIngestEVM     = "evm_head"
	GlobalRadarHeadIngestBitcoin = "bitcoin_head"
)

type GlobalRadarHeadIngestTarget struct {
	Kind      string
	NetworkID string
	Endpoint  string
}

type GlobalRadarHeadIngestConfig struct {
	EventSink  GlobalRadarTelemetryEventSink
	Targets    []GlobalRadarHeadIngestTarget
	Interval   time.Duration
	HTTPClient *http.Client
	Now        func() time.Time
	Health     *runtimehealth.Registry
}

type globalRadarHeadCandidate struct {
	key    string
	height uint64
	event  radarevent.Event
}

func GlobalRadarHeadIngestTargetHealthID(target GlobalRadarHeadIngestTarget) string {
	return "global-radar.head." + strings.ToLower(strings.TrimSpace(target.NetworkID)) + "." + strings.ToLower(strings.TrimSpace(target.Kind))
}

func RunGlobalRadarHeadIngestCycle(ctx context.Context, cfg GlobalRadarHeadIngestConfig, lastSeen map[string]uint64) (int, error) {
	if cfg.EventSink == nil {
		return 0, fmt.Errorf("global radar head ingest event sink is required")
	}
	if len(cfg.Targets) == 0 {
		return 0, fmt.Errorf("global radar head ingest targets are required")
	}
	if lastSeen == nil {
		return 0, fmt.Errorf("global radar head ingest last-seen state is required")
	}
	now := time.Now().UTC()
	if cfg.Now != nil {
		now = cfg.Now().UTC()
	}

	candidates := make([]globalRadarHeadCandidate, 0, len(cfg.Targets))
	errs := make([]error, 0)
	for _, target := range cfg.Targets {
		key := GlobalRadarHeadIngestTargetHealthID(target)
		if cfg.Health != nil {
			cfg.Health.Register(key, "head_ingest", target.NetworkID, true)
		}
		candidate, err := collectGlobalRadarHeadCandidate(ctx, cfg.HTTPClient, target, now)
		if err != nil {
			wrapped := fmt.Errorf("%s/%s: %w", strings.TrimSpace(target.NetworkID), strings.TrimSpace(target.Kind), err)
			errs = append(errs, wrapped)
			if cfg.Health != nil {
				cfg.Health.Failure(key, wrapped)
			}
			continue
		}
		if cfg.Health != nil {
			cfg.Health.Success(key, 1)
		}
		previous, seen := lastSeen[candidate.key]
		if seen && candidate.height <= previous {
			continue
		}
		candidates = append(candidates, candidate)
	}

	if len(candidates) > 0 {
		events := make([]radarevent.Event, 0, len(candidates))
		for _, candidate := range candidates {
			events = append(events, candidate.event)
		}
		if err := cfg.EventSink.InsertGlobalRadarEvents(ctx, events); err != nil {
			if cfg.Health != nil {
				cfg.Health.Failure("global-radar.head.event-sink", err)
			}
			return 0, fmt.Errorf("persist global radar head events: %w", err)
		}
		if cfg.Health != nil {
			cfg.Health.Success("global-radar.head.event-sink", len(events))
		}
		for _, candidate := range candidates {
			lastSeen[candidate.key] = candidate.height
		}
	}

	if len(errs) > 0 {
		return len(candidates), errors.Join(errs...)
	}
	return len(candidates), nil
}

func collectGlobalRadarHeadCandidate(ctx context.Context, client *http.Client, target GlobalRadarHeadIngestTarget, observedAt time.Time) (globalRadarHeadCandidate, error) {
	kind := strings.ToLower(strings.TrimSpace(target.Kind))
	networkID := strings.ToLower(strings.TrimSpace(target.NetworkID))
	endpoint := strings.TrimSpace(target.Endpoint)
	if endpoint == "" {
		return globalRadarHeadCandidate{}, fmt.Errorf("head ingest endpoint is required")
	}
	switch kind {
	case GlobalRadarHeadIngestEVM:
		result, err := networktarget.ProbeEVMHeadObservation(ctx, client, endpoint, networkID, observedAt)
		if err != nil {
			return globalRadarHeadCandidate{}, err
		}
		event, err := radarevent.BuildEVMHeadBlockEvent("evm-head-ingest-adapter", result)
		if err != nil {
			return globalRadarHeadCandidate{}, err
		}
		return globalRadarHeadCandidate{key: networkID + "/" + kind, height: result.HeadBlock, event: event}, nil
	case GlobalRadarHeadIngestBitcoin:
		if networkID != "bitcoin-mainnet" {
			return globalRadarHeadCandidate{}, fmt.Errorf("bitcoin head ingest requires bitcoin-mainnet")
		}
		result, err := networktarget.ProbeBitcoinHeadObservation(ctx, client, endpoint, observedAt)
		if err != nil {
			return globalRadarHeadCandidate{}, err
		}
		event, err := radarevent.BuildBitcoinHeadBlockEvent("bitcoin-head-ingest-adapter", result)
		if err != nil {
			return globalRadarHeadCandidate{}, err
		}
		return globalRadarHeadCandidate{key: networkID + "/" + kind, height: result.HeadBlock, event: event}, nil
	default:
		return globalRadarHeadCandidate{}, fmt.Errorf("unsupported global radar head ingest kind %q", kind)
	}
}

func StartGlobalRadarHeadIngest(ctx context.Context, cfg GlobalRadarHeadIngestConfig) func() {
	if cfg.EventSink == nil || len(cfg.Targets) == 0 {
		return func() {}
	}
	interval := cfg.Interval
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	if interval > 5*time.Minute {
		interval = 5 * time.Minute
	}
	if cfg.Health != nil {
		cfg.Health.Register("worker.global-radar-head-ingest", "worker", "", true)
		cfg.Health.Register("global-radar.head.event-sink", "storage", "", true)
		for _, target := range cfg.Targets {
			cfg.Health.Register(GlobalRadarHeadIngestTargetHealthID(target), "head_ingest", target.NetworkID, true)
		}
	}

	workerCtx, cancel := context.WithCancel(ctx)
	go func() {
		lastSeen := map[string]uint64{}
		run := func() {
			cycleCtx, cycleCancel := context.WithTimeout(workerCtx, 45*time.Second)
			defer cycleCancel()
			persisted, err := RunGlobalRadarHeadIngestCycle(cycleCtx, cfg, lastSeen)
			if err != nil {
				if cfg.Health != nil {
					cfg.Health.Failure("worker.global-radar-head-ingest", err)
				}
				if persisted > 0 {
					log.Printf("global radar head ingest partial cycle persisted: events=%d error=%v", persisted, err)
				} else {
					log.Printf("global radar head ingest cycle failed: %v", err)
				}
				return
			}
			if cfg.Health != nil {
				cfg.Health.Success("worker.global-radar-head-ingest", persisted)
			}
			log.Printf("global radar head ingest cycle persisted: events=%d", persisted)
		}
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				if cfg.Health != nil {
					cfg.Health.Stop("worker.global-radar-head-ingest")
				}
				return
			case <-ticker.C:
				run()
			}
		}
	}()
	return cancel
}
