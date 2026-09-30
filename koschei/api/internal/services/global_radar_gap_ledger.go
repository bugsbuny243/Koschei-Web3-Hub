package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/radarcursor"
)

const globalRadarHeadGapStoreHealthID = "global-radar.head.gap-store"

func recordGlobalRadarIngestGap(ctx context.Context, cfg GlobalRadarHeadIngestConfig, target GlobalRadarHeadIngestTarget, cursorKey string, startHeight, endHeight uint64, reason string, observedAt time.Time) error {
	if cfg.GapStore == nil || startHeight > endHeight {
		return nil
	}
	gap, found, err := cfg.GapStore.LoadOpenGlobalRadarIngestGap(ctx, cursorKey)
	if err != nil {
		return fmt.Errorf("load open ingest gap: %w", err)
	}
	if !found {
		gap, err = radarcursor.NewGap(
			cursorKey,
			strings.ToLower(strings.TrimSpace(target.NetworkID)),
			strings.ToLower(strings.TrimSpace(target.Kind)),
			startHeight,
			endHeight,
			reason,
			observedAt,
		)
		if err != nil {
			return fmt.Errorf("build ingest gap: %w", err)
		}
	} else {
		if gap.NetworkID != strings.ToLower(strings.TrimSpace(target.NetworkID)) ||
			gap.StreamKind != strings.ToLower(strings.TrimSpace(target.Kind)) {
			return fmt.Errorf("open ingest gap identity mismatch")
		}
		if startHeight < gap.NextHeight {
			gap.NextHeight = startHeight
		}
		if endHeight > gap.EndHeight {
			gap.EndHeight = endHeight
		}
		gap.Reason = strings.ToLower(strings.TrimSpace(reason))
		gap.UpdatedAt = observedAt.UTC()
	}
	if err := cfg.GapStore.SaveGlobalRadarIngestGap(ctx, gap); err != nil {
		return fmt.Errorf("persist ingest gap: %w", err)
	}
	if cfg.Health != nil {
		cfg.Health.Success(globalRadarHeadGapStoreHealthID, 1)
	}
	return nil
}

func advanceGlobalRadarIngestGap(ctx context.Context, cfg GlobalRadarHeadIngestConfig, cursorKey string, height uint64, observedAt time.Time) error {
	if cfg.GapStore == nil {
		return nil
	}
	gap, found, err := cfg.GapStore.LoadOpenGlobalRadarIngestGap(ctx, cursorKey)
	if err != nil {
		return fmt.Errorf("load open ingest gap for replay: %w", err)
	}
	if !found || height < gap.NextHeight {
		return nil
	}
	if height > gap.EndHeight {
		height = gap.EndHeight
	}
	gap.NextHeight = height + 1
	gap.UpdatedAt = observedAt.UTC()
	if gap.NextHeight == gap.EndHeight+1 {
		resolved := observedAt.UTC()
		gap.State = radarcursor.GapStateResolved
		gap.ResolvedAt = &resolved
	}
	if err := cfg.GapStore.SaveGlobalRadarIngestGap(ctx, gap); err != nil {
		return fmt.Errorf("persist ingest gap replay progress: %w", err)
	}
	if cfg.Health != nil {
		cfg.Health.Success(globalRadarHeadGapStoreHealthID, 1)
	}
	return nil
}

func reconcileGlobalRadarIngestGapToCheckpoint(ctx context.Context, cfg GlobalRadarHeadIngestConfig, checkpoint radarcursor.Checkpoint, observedAt time.Time) error {
	if cfg.GapStore == nil {
		return nil
	}
	gap, found, err := cfg.GapStore.LoadOpenGlobalRadarIngestGap(ctx, checkpoint.CursorKey)
	if err != nil {
		return fmt.Errorf("load open ingest gap for checkpoint reconciliation: %w", err)
	}
	if !found || checkpoint.Height < gap.NextHeight {
		return nil
	}
	return advanceGlobalRadarIngestGap(ctx, cfg, checkpoint.CursorKey, checkpoint.Height, observedAt)
}
