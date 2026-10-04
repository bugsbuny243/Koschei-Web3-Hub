package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/runtimehealth"
)

const GlobalCampaignReconcilerHealthID = "worker.global-campaign-reconciler"
const globalCampaignReconciliationCheckpointKey = "clickhouse-global-radar"

// GlobalCampaignReplayCursor is the durable ordered position in ClickHouse graph
// memory. SnapshotSHA256 breaks ties when several snapshots share a timestamp.
type GlobalCampaignReplayCursor struct {
	RecordedAt     time.Time
	SnapshotSHA256 string
}

// GlobalCampaignReplayItem is a canonical graph snapshot reconstructed from the
// durable ClickHouse ledger. It carries no new verdict or execution authority.
type GlobalCampaignReplayItem struct {
	Cursor   GlobalCampaignReplayCursor
	Snapshot GlobalRadarSnapshot
}

// GlobalCampaignReplaySource is implemented by durable graph stores that can
// reconstruct canonical snapshots in stable oldest-first order.
type GlobalCampaignReplaySource interface {
	ReadGlobalCampaignReplay(context.Context, GlobalCampaignReplayCursor, int) ([]GlobalCampaignReplayItem, error)
}

func VerifyGlobalCampaignReconciliationSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrGlobalCampaignStoreUnavailable
	}
	var ok bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('global_campaign_reconciliation_checkpoint') IS NOT NULL`).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("campaign reconciliation migration is required")
	}
	return nil
}

func loadGlobalCampaignReplayCursor(ctx context.Context, db *sql.DB) (GlobalCampaignReplayCursor, error) {
	var cursor GlobalCampaignReplayCursor
	if db == nil {
		return cursor, ErrGlobalCampaignStoreUnavailable
	}
	err := db.QueryRowContext(ctx, `SELECT last_recorded_at,last_snapshot_sha256
        FROM global_campaign_reconciliation_checkpoint WHERE checkpoint_key=$1`, globalCampaignReconciliationCheckpointKey).
		Scan(&cursor.RecordedAt, &cursor.SnapshotSHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return cursor, errors.New("campaign reconciliation checkpoint is missing")
	}
	if err != nil {
		return cursor, err
	}
	cursor.RecordedAt = cursor.RecordedAt.UTC()
	cursor.SnapshotSHA256 = strings.TrimSpace(cursor.SnapshotSHA256)
	return cursor, nil
}

func advanceGlobalCampaignReplayCursor(ctx context.Context, db *sql.DB, cursor GlobalCampaignReplayCursor) error {
	if db == nil {
		return ErrGlobalCampaignStoreUnavailable
	}
	cursor.RecordedAt = cursor.RecordedAt.UTC()
	cursor.SnapshotSHA256 = strings.TrimSpace(cursor.SnapshotSHA256)
	if cursor.RecordedAt.IsZero() || len(cursor.SnapshotSHA256) != 64 {
		return errors.New("invalid campaign reconciliation cursor")
	}
	result, err := db.ExecContext(ctx, `UPDATE global_campaign_reconciliation_checkpoint
        SET last_recorded_at=$2,last_snapshot_sha256=$3,updated_at=clock_timestamp()
        WHERE checkpoint_key=$1
          AND (last_recorded_at,last_snapshot_sha256) < ($2,$3)`,
		globalCampaignReconciliationCheckpointKey, cursor.RecordedAt, cursor.SnapshotSHA256)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}
	// Another reconciler may have advanced the same singleton cursor first.
	// Re-read and accept only a cursor that is already at or beyond this item.
	current, err := loadGlobalCampaignReplayCursor(ctx, db)
	if err != nil {
		return err
	}
	if current.RecordedAt.After(cursor.RecordedAt) ||
		(current.RecordedAt.Equal(cursor.RecordedAt) && current.SnapshotSHA256 >= cursor.SnapshotSHA256) {
		return nil
	}
	return errors.New("campaign reconciliation cursor did not advance")
}

// ReconcileGlobalCampaignRuntime repairs the ClickHouse -> PostgreSQL handoff.
// EnqueueGlobalCampaignRadarSnapshot is already source-digest idempotent, so a
// replay after a crash cannot create a duplicate queue identity.
func ReconcileGlobalCampaignRuntime(ctx context.Context, db *sql.DB, source GlobalCampaignReplaySource, limit int) (int, error) {
	if db == nil {
		return 0, ErrGlobalCampaignStoreUnavailable
	}
	if source == nil {
		return 0, errors.New("campaign replay source is unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	cursor, err := loadGlobalCampaignReplayCursor(ctx, db)
	if err != nil {
		return 0, err
	}
	items, err := source.ReadGlobalCampaignReplay(ctx, cursor, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	previous := cursor
	for _, item := range items {
		item.Cursor.RecordedAt = item.Cursor.RecordedAt.UTC()
		item.Cursor.SnapshotSHA256 = strings.TrimSpace(item.Cursor.SnapshotSHA256)
		if item.Cursor.RecordedAt.IsZero() || len(item.Cursor.SnapshotSHA256) != 64 ||
			item.Cursor.RecordedAt.Before(previous.RecordedAt) ||
			(item.Cursor.RecordedAt.Equal(previous.RecordedAt) && item.Cursor.SnapshotSHA256 <= previous.SnapshotSHA256) {
			return processed, fmt.Errorf("campaign replay source returned a non-monotonic cursor")
		}
		if _, err := EnqueueGlobalCampaignRadarSnapshot(ctx, db, item.Snapshot); err != nil {
			return processed, fmt.Errorf("re-enqueue ClickHouse campaign snapshot %s: %w", item.Cursor.SnapshotSHA256, err)
		}
		if err := advanceGlobalCampaignReplayCursor(ctx, db, item.Cursor); err != nil {
			return processed, fmt.Errorf("advance campaign replay cursor: %w", err)
		}
		previous = item.Cursor
		processed++
	}
	return processed, nil
}

func StartGlobalCampaignReconciler(parent context.Context, db *sql.DB, source GlobalCampaignReplaySource, health *runtimehealth.Registry) func() {
	if db == nil || source == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	if health != nil {
		health.RegisterPeriodic(GlobalCampaignReconcilerHealthID, "worker", "", true, time.Minute)
	}
	go func() {
		defer close(done)
		if health != nil {
			defer health.Stop(GlobalCampaignReconcilerHealthID)
		}
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			cycle, cycleCancel := context.WithTimeout(ctx, 25*time.Second)
			processed, err := ReconcileGlobalCampaignRuntime(cycle, db, source, 25)
			cycleCancel()
			if health != nil {
				if err != nil {
					health.Failure(GlobalCampaignReconcilerHealthID, err)
				} else {
					health.Success(GlobalCampaignReconcilerHealthID, processed)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }
}
