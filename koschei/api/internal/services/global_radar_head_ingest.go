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
	"koschei/api/internal/radarcursor"
	"koschei/api/internal/radarevent"
	"koschei/api/internal/runtimehealth"
)

const (
	GlobalRadarHeadIngestEVM     = "evm_head"
	GlobalRadarHeadIngestBitcoin = "bitcoin_head"

	defaultGlobalRadarHeadIngestMaxBlocksPerCycle = 4
	maxGlobalRadarHeadIngestMaxBlocksPerCycle     = 64
	defaultGlobalRadarHeadIngestMaxEventsPerBlock = 6000
	maxGlobalRadarHeadIngestMaxEventsPerBlock     = 25000
	defaultGlobalRadarHeadIngestMaxReorgRewind    = 12
	maxGlobalRadarHeadIngestMaxReorgRewind        = 64
)

type GlobalRadarHeadIngestTarget struct {
	Kind                 string
	NetworkID            string
	Endpoint             string
	ConfirmationEndpoint string
}

type GlobalRadarHeadIngestConfig struct {
	EventSink           GlobalRadarTelemetryEventSink
	CursorStore         radarcursor.RecoveryStore
	Targets             []GlobalRadarHeadIngestTarget
	Interval            time.Duration
	HTTPClient          *http.Client
	Now                 func() time.Time
	Health              *runtimehealth.Registry
	MaxBlocksPerCycle   int
	MaxEventsPerBlock   int
	RequireConfirmation bool
	AutoReorgRecovery   bool
	MaxReorgRewind      uint64
}

type globalRadarHeadBundle struct {
	events     []radarevent.Event
	height     uint64
	blockHash  string
	parentHash string
	observedAt time.Time
}

func GlobalRadarHeadIngestTargetHealthID(target GlobalRadarHeadIngestTarget) string {
	return "global-radar.head." + strings.ToLower(strings.TrimSpace(target.NetworkID)) + "." + strings.ToLower(strings.TrimSpace(target.Kind))
}

func GlobalRadarHeadIngestCursorKey(target GlobalRadarHeadIngestTarget) string {
	return "global-radar/head/" + strings.ToLower(strings.TrimSpace(target.NetworkID)) + "/" + strings.ToLower(strings.TrimSpace(target.Kind))
}

func RunGlobalRadarHeadIngestCycle(ctx context.Context, cfg GlobalRadarHeadIngestConfig) (int, error) {
	if cfg.EventSink == nil {
		return 0, fmt.Errorf("global radar head ingest event sink is required")
	}
	if cfg.CursorStore == nil {
		return 0, fmt.Errorf("global radar head ingest cursor store is required")
	}
	if len(cfg.Targets) == 0 {
		return 0, fmt.Errorf("global radar head ingest targets are required")
	}

	now := time.Now().UTC()
	if cfg.Now != nil {
		now = cfg.Now().UTC()
	}

	totalPersisted := 0
	errs := make([]error, 0)
	for _, target := range cfg.Targets {
		healthID := GlobalRadarHeadIngestTargetHealthID(target)
		if cfg.Health != nil {
			cfg.Health.Register(healthID, "head_ingest", target.NetworkID, true)
		}
		persisted, err := runGlobalRadarHeadIngestTarget(ctx, cfg, target, now)
		totalPersisted += persisted
		if err != nil {
			wrapped := fmt.Errorf("%s/%s: %w", strings.TrimSpace(target.NetworkID), strings.TrimSpace(target.Kind), err)
			errs = append(errs, wrapped)
			if cfg.Health != nil {
				cfg.Health.Failure(healthID, wrapped)
			}
			continue
		}
		if cfg.Health != nil {
			cfg.Health.Success(healthID, persisted)
		}
	}

	if len(errs) > 0 {
		return totalPersisted, errors.Join(errs...)
	}
	return totalPersisted, nil
}

func runGlobalRadarHeadIngestTarget(ctx context.Context, cfg GlobalRadarHeadIngestConfig, target GlobalRadarHeadIngestTarget, observedAt time.Time) (int, error) {
	cursorKey := GlobalRadarHeadIngestCursorKey(target)
	checkpoint, found, err := cfg.CursorStore.LoadGlobalRadarIngestCheckpoint(ctx, cursorKey)
	if err != nil {
		if cfg.Health != nil {
			cfg.Health.Failure("global-radar.head.checkpoint-store", err)
		}
		return 0, fmt.Errorf("load durable head checkpoint: %w", err)
	}
	if cfg.Health != nil {
		cfg.Health.Success("global-radar.head.checkpoint-store", 0)
	}
	if found {
		if checkpoint.NetworkID != strings.ToLower(strings.TrimSpace(target.NetworkID)) ||
			checkpoint.StreamKind != strings.ToLower(strings.TrimSpace(target.Kind)) {
			return 0, fmt.Errorf("durable head checkpoint identity mismatch")
		}
		if checkpoint.State == radarcursor.StateReorgObserved {
			if !cfg.AutoReorgRecovery {
				return 0, fmt.Errorf("durable head checkpoint is frozen after reorg observation")
			}
			ancestor, recoverErr := recoverGlobalRadarReorg(ctx, cfg, target, checkpoint, observedAt)
			if recoverErr != nil {
				return 0, fmt.Errorf("durable head checkpoint is frozen after reorg observation: %w", recoverErr)
			}
			return 0, fmt.Errorf("automatic reorg recovery rewound durable cursor to height %d; next cycle will resume", ancestor.Height)
		}
	}

	headHeight, err := probeGlobalRadarHeadHeight(ctx, cfg.HTTPClient, target, observedAt)
	if err != nil {
		return 0, err
	}
	if found && headHeight < checkpoint.Height {
		return 0, fmt.Errorf("provider head %d is behind durable checkpoint %d", headHeight, checkpoint.Height)
	}

	maxBlocks := cfg.MaxBlocksPerCycle
	if maxBlocks <= 0 {
		maxBlocks = defaultGlobalRadarHeadIngestMaxBlocksPerCycle
	}
	if maxBlocks > maxGlobalRadarHeadIngestMaxBlocksPerCycle {
		maxBlocks = maxGlobalRadarHeadIngestMaxBlocksPerCycle
	}
	maxEvents := cfg.MaxEventsPerBlock
	if maxEvents <= 0 {
		maxEvents = defaultGlobalRadarHeadIngestMaxEventsPerBlock
	}
	if maxEvents > maxGlobalRadarHeadIngestMaxEventsPerBlock {
		maxEvents = maxGlobalRadarHeadIngestMaxEventsPerBlock
	}

	if !found {
		bundle, err := collectGlobalRadarHeadBundle(ctx, cfg.HTTPClient, target, headHeight, observedAt)
		if err != nil {
			return 0, err
		}
		if err := confirmGlobalRadarHeadBundle(ctx, cfg, target, bundle, observedAt); err != nil {
			return 0, err
		}
		return persistGlobalRadarHeadBundle(ctx, cfg, target, cursorKey, bundle, maxEvents)
	}

	if headHeight == checkpoint.Height {
		bundle, err := collectGlobalRadarHeadBundle(ctx, cfg.HTTPClient, target, checkpoint.Height, observedAt)
		if err != nil {
			return 0, err
		}
		if err := confirmGlobalRadarHeadBundle(ctx, cfg, target, bundle, observedAt); err != nil {
			return 0, err
		}
		if bundle.blockHash != checkpoint.BlockHash {
			reason := fmt.Sprintf("reorg observed at height %d: stored=%s current=%s", checkpoint.Height, checkpoint.BlockHash, bundle.blockHash)
			return 0, handleGlobalRadarReorg(ctx, cfg, target, checkpoint, observedAt, reason)
		}
		return 0, nil
	}

	persisted := 0
	current := checkpoint
	lastHeight := headHeight
	if span := lastHeight - current.Height; span > uint64(maxBlocks) {
		lastHeight = current.Height + uint64(maxBlocks)
	}
	for height := current.Height + 1; height <= lastHeight; height++ {
		bundle, err := collectGlobalRadarHeadBundle(ctx, cfg.HTTPClient, target, height, observedAt)
		if err != nil {
			return persisted, err
		}
		if err := confirmGlobalRadarHeadBundle(ctx, cfg, target, bundle, observedAt); err != nil {
			return persisted, err
		}
		if bundle.parentHash != current.BlockHash {
			reason := fmt.Sprintf("reorg observed before height %d: expected_parent=%s observed_parent=%s", height, current.BlockHash, bundle.parentHash)
			return persisted, handleGlobalRadarReorg(ctx, cfg, target, current, observedAt, reason)
		}
		count, err := persistGlobalRadarHeadBundle(ctx, cfg, target, cursorKey, bundle, maxEvents)
		persisted += count
		if err != nil {
			return persisted, err
		}
		current = radarcursor.Checkpoint{
			CursorKey:         cursorKey,
			NetworkID:         strings.ToLower(strings.TrimSpace(target.NetworkID)),
			StreamKind:        strings.ToLower(strings.TrimSpace(target.Kind)),
			Height:            bundle.height,
			BlockHash:         bundle.blockHash,
			ParentHash:        bundle.parentHash,
			SourceEventSHA256: bundle.events[0].EventSHA256,
			State:             radarcursor.StateCanonical,
			ObservedAt:        bundle.observedAt,
		}
	}
	return persisted, nil
}

func persistGlobalRadarHeadBundle(ctx context.Context, cfg GlobalRadarHeadIngestConfig, target GlobalRadarHeadIngestTarget, cursorKey string, bundle globalRadarHeadBundle, maxEvents int) (int, error) {
	if len(bundle.events) == 0 || bundle.events[0].Kind != radarevent.KindBlock {
		return 0, fmt.Errorf("head ingest bundle missing canonical block event")
	}
	if len(bundle.events) > maxEvents {
		return 0, fmt.Errorf("head ingest block event count %d exceeds configured limit %d", len(bundle.events), maxEvents)
	}
	if err := cfg.EventSink.InsertGlobalRadarEvents(ctx, bundle.events); err != nil {
		if cfg.Health != nil {
			cfg.Health.Failure("global-radar.head.event-sink", err)
		}
		return 0, fmt.Errorf("persist global radar head events: %w", err)
	}
	if cfg.Health != nil {
		cfg.Health.Success("global-radar.head.event-sink", len(bundle.events))
	}

	checkpoint := radarcursor.Checkpoint{
		CursorKey:         cursorKey,
		NetworkID:         strings.ToLower(strings.TrimSpace(target.NetworkID)),
		StreamKind:        strings.ToLower(strings.TrimSpace(target.Kind)),
		Height:            bundle.height,
		BlockHash:         bundle.blockHash,
		ParentHash:        bundle.parentHash,
		SourceEventSHA256: bundle.events[0].EventSHA256,
		State:             radarcursor.StateCanonical,
		ObservedAt:        bundle.observedAt,
	}
	if err := cfg.CursorStore.SaveGlobalRadarIngestCheckpoint(ctx, checkpoint); err != nil {
		if cfg.Health != nil {
			cfg.Health.Failure("global-radar.head.checkpoint-store", err)
		}
		return len(bundle.events), fmt.Errorf("persist durable head checkpoint after events: %w", err)
	}
	if cfg.Health != nil {
		cfg.Health.Success("global-radar.head.checkpoint-store", 1)
	}
	return len(bundle.events), nil
}

func freezeGlobalRadarHeadCheckpoint(ctx context.Context, cfg GlobalRadarHeadIngestConfig, checkpoint radarcursor.Checkpoint, observedAt time.Time) error {
	checkpoint.State = radarcursor.StateReorgObserved
	checkpoint.ObservedAt = observedAt.UTC()
	if err := cfg.CursorStore.SaveGlobalRadarIngestCheckpoint(ctx, checkpoint); err != nil {
		if cfg.Health != nil {
			cfg.Health.Failure("global-radar.head.checkpoint-store", err)
		}
		return err
	}
	if cfg.Health != nil {
		cfg.Health.Success("global-radar.head.checkpoint-store", 1)
	}
	return nil
}

func probeGlobalRadarHeadHeight(ctx context.Context, client *http.Client, target GlobalRadarHeadIngestTarget, observedAt time.Time) (uint64, error) {
	kind := strings.ToLower(strings.TrimSpace(target.Kind))
	networkID := strings.ToLower(strings.TrimSpace(target.NetworkID))
	endpoint := strings.TrimSpace(target.Endpoint)
	if endpoint == "" {
		return 0, fmt.Errorf("head ingest endpoint is required")
	}
	switch kind {
	case GlobalRadarHeadIngestEVM:
		result, err := networktarget.ProbeEVMHeadObservation(ctx, client, endpoint, networkID, observedAt)
		if err != nil {
			return 0, err
		}
		return result.HeadBlock, nil
	case GlobalRadarHeadIngestBitcoin:
		if networkID != "bitcoin-mainnet" {
			return 0, fmt.Errorf("bitcoin head ingest requires bitcoin-mainnet")
		}
		result, err := networktarget.ProbeBitcoinHeadObservation(ctx, client, endpoint, observedAt)
		if err != nil {
			return 0, err
		}
		return result.HeadBlock, nil
	default:
		return 0, fmt.Errorf("unsupported global radar head ingest kind %q", kind)
	}
}

func collectGlobalRadarHeadBundle(ctx context.Context, client *http.Client, target GlobalRadarHeadIngestTarget, height uint64, observedAt time.Time) (globalRadarHeadBundle, error) {
	kind := strings.ToLower(strings.TrimSpace(target.Kind))
	networkID := strings.ToLower(strings.TrimSpace(target.NetworkID))
	endpoint := strings.TrimSpace(target.Endpoint)
	if endpoint == "" {
		return globalRadarHeadBundle{}, fmt.Errorf("head ingest endpoint is required")
	}
	switch kind {
	case GlobalRadarHeadIngestEVM:
		result, err := networktarget.ProbeEVMBlockIngest(ctx, client, endpoint, networkID, height, observedAt)
		if err != nil {
			return globalRadarHeadBundle{}, err
		}
		events, err := radarevent.BuildEVMBlockIngestEvents("evm-block-ingest-adapter", result)
		if err != nil {
			return globalRadarHeadBundle{}, err
		}
		return globalRadarHeadBundle{
			events: events, height: result.Height, blockHash: result.Hash, parentHash: result.ParentHash, observedAt: result.ObservedAt,
		}, nil
	case GlobalRadarHeadIngestBitcoin:
		if networkID != "bitcoin-mainnet" {
			return globalRadarHeadBundle{}, fmt.Errorf("bitcoin head ingest requires bitcoin-mainnet")
		}
		result, err := networktarget.ProbeBitcoinBlockIngest(ctx, client, endpoint, height, observedAt)
		if err != nil {
			return globalRadarHeadBundle{}, err
		}
		events, err := radarevent.BuildBitcoinBlockIngestEvents("bitcoin-block-ingest-adapter", result)
		if err != nil {
			return globalRadarHeadBundle{}, err
		}
		return globalRadarHeadBundle{
			events: events, height: result.Height, blockHash: result.Hash, parentHash: result.PreviousHash, observedAt: result.ObservedAt,
		}, nil
	default:
		return globalRadarHeadBundle{}, fmt.Errorf("unsupported global radar head ingest kind %q", kind)
	}
}

func GlobalRadarHeadIngestConfirmationHealthID(target GlobalRadarHeadIngestTarget) string {
	return "global-radar.confirmation." + strings.ToLower(strings.TrimSpace(target.NetworkID)) + "." + strings.ToLower(strings.TrimSpace(target.Kind))
}

func confirmGlobalRadarHeadBundle(ctx context.Context, cfg GlobalRadarHeadIngestConfig, target GlobalRadarHeadIngestTarget, bundle globalRadarHeadBundle, observedAt time.Time) error {
	endpoint := strings.TrimSpace(target.ConfirmationEndpoint)
	healthID := GlobalRadarHeadIngestConfirmationHealthID(target)
	if endpoint == "" {
		if cfg.RequireConfirmation {
			err := fmt.Errorf("independent confirmation endpoint is required for %s", target.NetworkID)
			if cfg.Health != nil {
				cfg.Health.Failure(healthID, err)
			}
			return err
		}
		return nil
	}
	if cfg.Health != nil {
		cfg.Health.Register(healthID, "block_confirmation", target.NetworkID, true)
	}
	identity, err := probeGlobalRadarBlockIdentity(ctx, cfg.HTTPClient, target, endpoint, bundle.height, observedAt)
	if err != nil {
		if cfg.Health != nil {
			cfg.Health.Failure(healthID, err)
		}
		return fmt.Errorf("independent block confirmation failed: %w", err)
	}
	if identity.Hash != bundle.blockHash || identity.ParentHash != bundle.parentHash {
		err := fmt.Errorf("multi-provider block disagreement at height %d: primary=%s/%s confirmation=%s/%s", bundle.height, bundle.blockHash, bundle.parentHash, identity.Hash, identity.ParentHash)
		if cfg.Health != nil {
			cfg.Health.Failure(healthID, err)
		}
		return err
	}
	if cfg.Health != nil {
		cfg.Health.Success(healthID, 1)
	}
	return nil
}

func probeGlobalRadarBlockIdentity(ctx context.Context, client *http.Client, target GlobalRadarHeadIngestTarget, endpoint string, height uint64, observedAt time.Time) (networktarget.BlockIdentity, error) {
	kind := strings.ToLower(strings.TrimSpace(target.Kind))
	networkID := strings.ToLower(strings.TrimSpace(target.NetworkID))
	switch kind {
	case GlobalRadarHeadIngestEVM:
		return networktarget.ProbeEVMBlockIdentity(ctx, client, endpoint, networkID, height, observedAt)
	case GlobalRadarHeadIngestBitcoin:
		if networkID != "bitcoin-mainnet" {
			return networktarget.BlockIdentity{}, fmt.Errorf("bitcoin block identity requires bitcoin-mainnet")
		}
		return networktarget.ProbeBitcoinBlockIdentity(ctx, client, endpoint, height, observedAt)
	default:
		return networktarget.BlockIdentity{}, fmt.Errorf("unsupported global radar block identity kind %q", kind)
	}
}

func handleGlobalRadarReorg(ctx context.Context, cfg GlobalRadarHeadIngestConfig, target GlobalRadarHeadIngestTarget, checkpoint radarcursor.Checkpoint, observedAt time.Time, reason string) error {
	if checkpoint.State != radarcursor.StateReorgObserved {
		if err := freezeGlobalRadarHeadCheckpoint(ctx, cfg, checkpoint, observedAt); err != nil {
			return fmt.Errorf("%s; freeze checkpoint failed: %w", reason, err)
		}
		checkpoint.State = radarcursor.StateReorgObserved
		checkpoint.ObservedAt = observedAt.UTC()
	}
	if !cfg.AutoReorgRecovery {
		return fmt.Errorf("%s", reason)
	}
	ancestor, err := recoverGlobalRadarReorg(ctx, cfg, target, checkpoint, observedAt)
	if err != nil {
		return fmt.Errorf("%s; automatic recovery failed: %w", reason, err)
	}
	return fmt.Errorf("%s; automatic recovery rewound durable cursor to height %d; next cycle will resume", reason, ancestor.Height)
}

func recoverGlobalRadarReorg(ctx context.Context, cfg GlobalRadarHeadIngestConfig, target GlobalRadarHeadIngestTarget, checkpoint radarcursor.Checkpoint, observedAt time.Time) (radarcursor.Checkpoint, error) {
	healthID := "global-radar.reorg-recovery." + strings.ToLower(strings.TrimSpace(target.NetworkID))
	if strings.TrimSpace(target.ConfirmationEndpoint) == "" {
		err := fmt.Errorf("reorg recovery requires an independent confirmation endpoint")
		if cfg.Health != nil {
			cfg.Health.Failure(healthID, err)
		}
		return radarcursor.Checkpoint{}, err
	}
	if checkpoint.Height == 0 {
		err := fmt.Errorf("reorg recovery cannot rewind below genesis")
		if cfg.Health != nil {
			cfg.Health.Failure(healthID, err)
		}
		return radarcursor.Checkpoint{}, err
	}
	maxRewind := cfg.MaxReorgRewind
	if maxRewind == 0 {
		maxRewind = defaultGlobalRadarHeadIngestMaxReorgRewind
	}
	if maxRewind > maxGlobalRadarHeadIngestMaxReorgRewind {
		maxRewind = maxGlobalRadarHeadIngestMaxReorgRewind
	}
	floor := uint64(0)
	if checkpoint.Height > maxRewind {
		floor = checkpoint.Height - maxRewind
	}
	cursorKey := GlobalRadarHeadIngestCursorKey(target)

	for height := checkpoint.Height - 1; ; height-- {
		stored, found, err := cfg.CursorStore.LoadGlobalRadarCanonicalCheckpointAtHeight(ctx, cursorKey, height)
		if err != nil {
			if cfg.Health != nil {
				cfg.Health.Failure(healthID, err)
			}
			return radarcursor.Checkpoint{}, fmt.Errorf("load canonical checkpoint at height %d: %w", height, err)
		}
		if found {
			primary, err := probeGlobalRadarBlockIdentity(ctx, cfg.HTTPClient, target, target.Endpoint, height, observedAt)
			if err != nil {
				if cfg.Health != nil {
					cfg.Health.Failure(healthID, err)
				}
				return radarcursor.Checkpoint{}, fmt.Errorf("primary recovery probe at height %d: %w", height, err)
			}
			confirmation, err := probeGlobalRadarBlockIdentity(ctx, cfg.HTTPClient, target, target.ConfirmationEndpoint, height, observedAt)
			if err != nil {
				if cfg.Health != nil {
					cfg.Health.Failure(healthID, err)
				}
				return radarcursor.Checkpoint{}, fmt.Errorf("confirmation recovery probe at height %d: %w", height, err)
			}
			if primary.Hash != confirmation.Hash || primary.ParentHash != confirmation.ParentHash {
				err := fmt.Errorf("recovery providers disagree at height %d", height)
				if cfg.Health != nil {
					cfg.Health.Failure(healthID, err)
				}
				return radarcursor.Checkpoint{}, err
			}
			if stored.BlockHash == primary.Hash && stored.ParentHash == primary.ParentHash {
				rewind := stored
				rewind.State = radarcursor.StateRewind
				rewind.ObservedAt = observedAt.UTC()
				if err := cfg.CursorStore.SaveGlobalRadarIngestCheckpoint(ctx, rewind); err != nil {
					if cfg.Health != nil {
						cfg.Health.Failure(healthID, err)
					}
					return radarcursor.Checkpoint{}, fmt.Errorf("persist rewind checkpoint: %w", err)
				}
				if cfg.Health != nil {
					cfg.Health.Success(healthID, 1)
				}
				log.Printf("global radar reorg recovery rewound %s to height=%d hash=%s", target.NetworkID, rewind.Height, rewind.BlockHash)
				return rewind, nil
			}
		}
		if height == floor || height == 0 {
			break
		}
	}
	err := fmt.Errorf("no two-provider common ancestor found within rewind window=%d", maxRewind)
	if cfg.Health != nil {
		cfg.Health.Failure(healthID, err)
	}
	return radarcursor.Checkpoint{}, err
}

func StartGlobalRadarHeadIngest(ctx context.Context, cfg GlobalRadarHeadIngestConfig) func() {
	if cfg.EventSink == nil || cfg.CursorStore == nil || len(cfg.Targets) == 0 {
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
		cfg.Health.Register("global-radar.head.checkpoint-store", "storage", "", true)
		for _, target := range cfg.Targets {
			cfg.Health.Register(GlobalRadarHeadIngestTargetHealthID(target), "head_ingest", target.NetworkID, true)
			if strings.TrimSpace(target.ConfirmationEndpoint) != "" || cfg.RequireConfirmation {
				cfg.Health.Register(GlobalRadarHeadIngestConfirmationHealthID(target), "block_confirmation", target.NetworkID, strings.TrimSpace(target.ConfirmationEndpoint) != "")
			}
			if cfg.AutoReorgRecovery {
				cfg.Health.Register("global-radar.reorg-recovery."+strings.ToLower(strings.TrimSpace(target.NetworkID)), "reorg_recovery", target.NetworkID, strings.TrimSpace(target.ConfirmationEndpoint) != "")
			}
		}
	}

	workerCtx, cancel := context.WithCancel(ctx)
	go func() {
		run := func() {
			cycleCtx, cycleCancel := context.WithTimeout(workerCtx, 90*time.Second)
			defer cycleCancel()
			persisted, err := RunGlobalRadarHeadIngestCycle(cycleCtx, cfg)
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
