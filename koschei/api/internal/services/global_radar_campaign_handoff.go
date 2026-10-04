package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"koschei/api/internal/workerwake"
)

const globalCampaignWake = "global-campaign-runtime"
const globalRadarHandoffHealthID = "worker.global-radar-campaign-handoff"
const globalRadarHandoffCapacity = 10000

type GlobalRadarCampaignHandoffStatus struct {
	Pending             int64      `json:"pending"`
	Publishing          int64      `json:"publishing"`
	Failed              int64      `json:"failed"`
	Completed           int64      `json:"completed"`
	OldestOutstandingAt *time.Time `json:"oldest_outstanding_at,omitempty"`
}

type globalRadarHandoffClaim struct {
	source, owner string
	token         int64
	payload       []byte
	attempts      int
}

func canonicalGlobalRadarHandoff(snapshot GlobalRadarSnapshot) (GlobalRadarSnapshot, []byte, string, error) {
	if _, err := buildGlobalCampaignRuntimeJobs(snapshot); err != nil {
		return GlobalRadarSnapshot{}, nil, "", err
	}
	canonical, err := validateCanonicalGlobalCampaignRadarSnapshot(snapshot)
	if err != nil {
		return GlobalRadarSnapshot{}, nil, "", err
	}
	if canonical.Coverage != snapshot.Coverage {
		return GlobalRadarSnapshot{}, nil, "", ErrGlobalCampaignRuntimeInput
	}
	payload, err := json.Marshal(canonical)
	if err != nil || len(payload) > 512*1024 {
		return GlobalRadarSnapshot{}, nil, "", ErrGlobalCampaignRuntimeInput
	}
	digest := sha256.Sum256(payload)
	return canonical, payload, "sha256:" + hex.EncodeToString(digest[:]), nil
}

// StageGlobalRadarCampaignSnapshot commits before any external publication.
// Completed rows retain only a hash receipt; raw snapshot JSON is compacted.
func StageGlobalRadarCampaignSnapshot(ctx context.Context, db *sql.DB, snapshot GlobalRadarSnapshot) error {
	if db == nil {
		return ErrGlobalCampaignStoreUnavailable
	}
	_, payload, source, err := canonicalGlobalRadarHandoff(snapshot)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var ignored any
	if err := tx.QueryRowContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('global-radar-campaign-handoff',0))`).Scan(&ignored); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM global_radar_campaign_handoffs WHERE source_ref=$1)`, source).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		var outstanding int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT 1 FROM global_radar_campaign_handoffs WHERE status <> 'complete' LIMIT 10000) q`).Scan(&outstanding); err != nil {
			return err
		}
		if outstanding >= globalRadarHandoffCapacity {
			return errors.New("radar campaign handoff capacity reached")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO global_radar_campaign_handoffs(source_ref,snapshot_payload) VALUES($1,$2)`, source, string(payload)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	workerwake.Signal(globalCampaignWake)
	return nil
}

func claimGlobalRadarCampaignHandoff(ctx context.Context, db *sql.DB, owner string) (globalRadarHandoffClaim, bool, error) {
	claim := globalRadarHandoffClaim{owner: owner}
	if owner == "" || len(owner) > 160 {
		return claim, false, ErrGlobalCampaignRuntimeInput
	}
	// One statement locks, claims and commits. No SQL transaction spans HTTP IO.
	err := db.QueryRowContext(ctx, `WITH next AS (
        SELECT source_ref FROM global_radar_campaign_handoffs
        WHERE (status='pending' AND next_attempt_at <= clock_timestamp())
           OR (status='publishing' AND lease_until <= clock_timestamp())
        ORDER BY created_at,source_ref LIMIT 1 FOR UPDATE SKIP LOCKED
    ) UPDATE global_radar_campaign_handoffs h SET status='publishing',lease_owner=$1,
        fencing_token=h.fencing_token+1,lease_until=clock_timestamp()+interval '5 minutes',
        attempts=LEAST(h.attempts,2147483646)+1,updated_at=clock_timestamp()
        FROM next WHERE h.source_ref=next.source_ref
        RETURNING h.source_ref,h.snapshot_payload,h.fencing_token,h.attempts`, owner).
		Scan(&claim.source, &claim.payload, &claim.token, &claim.attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return claim, false, nil
	}
	return claim, err == nil, err
}

func retryGlobalRadarCampaignHandoff(ctx context.Context, db *sql.DB, claim globalRadarHandoffClaim, invalid bool) error {
	status, code := "pending", "publication_retry"
	if invalid {
		status, code = "failed", "invalid_canonical_payload"
	}
	shift := claim.attempts
	if shift > 6 {
		shift = 6
	}
	delay := time.Duration(1<<uint(shift)) * 10 * time.Second
	result, err := db.ExecContext(ctx, `UPDATE global_radar_campaign_handoffs SET status=$4,
        lease_owner='',lease_until=NULL,last_error_code=$5,next_attempt_at=clock_timestamp()+($6 * interval '1 second'),updated_at=clock_timestamp()
        WHERE source_ref=$1 AND lease_owner=$2 AND fencing_token=$3 AND status='publishing' AND lease_until>clock_timestamp()`, claim.source, claim.owner, claim.token, status, code, delay.Seconds())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrGlobalCampaignWorkerLeaseFenced
	}
	return nil
}

func completeGlobalRadarCampaignHandoff(ctx context.Context, db *sql.DB, claim globalRadarHandoffClaim, snapshot GlobalRadarSnapshot) error {
	jobs, err := buildGlobalCampaignRuntimeJobs(snapshot)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var source string
	if err := tx.QueryRowContext(ctx, `SELECT source_ref FROM global_radar_campaign_handoffs WHERE source_ref=$1
        AND lease_owner=$2 AND fencing_token=$3 AND status='publishing' AND lease_until>clock_timestamp() FOR UPDATE`, claim.source, claim.owner, claim.token).Scan(&source); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrGlobalCampaignWorkerLeaseFenced
		}
		return err
	}
	if _, err := enqueueGlobalCampaignRuntimeJobsTx(ctx, tx, jobs); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE global_radar_campaign_handoffs SET status='complete',snapshot_payload=NULL,
        lease_owner='',lease_until=NULL,last_error_code='',completed_at=clock_timestamp(),updated_at=clock_timestamp()
        WHERE source_ref=$1 AND lease_owner=$2 AND fencing_token=$3 AND lease_until>clock_timestamp()`, claim.source, claim.owner, claim.token)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrGlobalCampaignWorkerLeaseFenced
	}
	return tx.Commit()
}

// ProcessGlobalRadarCampaignHandoff replays an expired publication with stable
// evidence IDs. ClickHouse ReplacingMergeTree reads deduplicate replayed rows.
func ProcessGlobalRadarCampaignHandoff(ctx context.Context, db *sql.DB, sink GlobalRadarSnapshotSink, owner string) (bool, error) {
	if db == nil || sink == nil {
		return false, ErrGlobalCampaignStoreUnavailable
	}
	claim, found, err := claimGlobalRadarCampaignHandoff(ctx, db, owner)
	if err != nil || !found {
		return false, err
	}
	var snapshot GlobalRadarSnapshot
	var source string
	decoder := json.NewDecoder(bytes.NewReader(claim.payload))
	decoder.UseNumber()
	if err = decoder.Decode(&snapshot); err == nil {
		snapshot, _, source, err = canonicalGlobalRadarHandoff(snapshot)
	}
	invalid := err != nil || source != claim.source
	if invalid {
		err = ErrGlobalCampaignRuntimeInput
	} else {
		err = sink.InsertGlobalRadarSnapshot(ctx, snapshot)
		if err == nil {
			err = completeGlobalRadarCampaignHandoff(ctx, db, claim, snapshot)
		}
	}
	if err != nil {
		// A canceled request must not prevent releasing its durable retry lease.
		retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		retryErr := retryGlobalRadarCampaignHandoff(retryCtx, db, claim, invalid)
		if retryErr != nil {
			return true, errors.Join(err, retryErr)
		}
		return true, err
	}
	return true, nil
}

func globalCampaignRuntimeSleep(ctx context.Context, db *sql.DB) time.Duration {
	var pending bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM global_campaign_runtime_queue WHERE status='pending')`).Scan(&pending); err != nil {
		return time.Minute
	}
	if pending {
		return 100 * time.Millisecond
	}
	var next sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT min(due) FROM (
        SELECT min(next_attempt_at) AS due FROM global_radar_campaign_handoffs WHERE status='pending'
        UNION ALL SELECT min(lease_until) FROM global_radar_campaign_handoffs WHERE status='publishing') q`).Scan(&next); err != nil {
		return time.Minute
	}
	if !next.Valid {
		return workerwake.RecoveryCeiling()
	}
	wait := time.Until(next.Time)
	if wait < 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	return wait
}

func readGlobalRadarCampaignHandoffStatus(ctx context.Context, db *sql.DB) (GlobalRadarCampaignHandoffStatus, error) {
	var s GlobalRadarCampaignHandoffStatus
	err := db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE status='pending'),count(*) FILTER (WHERE status='publishing'),
        count(*) FILTER (WHERE status='failed'),count(*) FILTER (WHERE status='complete'),min(created_at) FILTER (WHERE status IN ('pending','publishing'))
        FROM global_radar_campaign_handoffs`).Scan(&s.Pending, &s.Publishing, &s.Failed, &s.Completed, &s.OldestOutstandingAt)
	return s, err
}
