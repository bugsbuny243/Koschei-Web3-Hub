package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	ErrGlobalCampaignWorkerLeaseInvalid = errors.New("global campaign worker lease is invalid")
	ErrGlobalCampaignWorkerLeaseBusy    = errors.New("global campaign worker lease is busy")
	ErrGlobalCampaignWorkerLeaseFenced  = errors.New("global campaign worker has been fenced")
	ErrGlobalCampaignWorkerLeaseExpired = errors.New("global campaign worker lease is expired")
)

type GlobalCampaignWorkerLease struct {
	LeaseKey       string    `json:"lease_key"`
	LeaseOwner     string    `json:"lease_owner"`
	FencingToken   int64     `json:"fencing_token"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
	AcquiredAt     time.Time `json:"acquired_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func AcquireGlobalCampaignWorkerLease(ctx context.Context, db *sql.DB, leaseKey, owner string, now time.Time, ttl time.Duration) (GlobalCampaignWorkerLease, error) {
	leaseKey = strings.TrimSpace(leaseKey)
	owner = strings.TrimSpace(owner)
	now = now.UTC()
	if db == nil || leaseKey == "" || owner == "" || now.IsZero() || ttl <= 0 {
		return GlobalCampaignWorkerLease{}, ErrGlobalCampaignWorkerLeaseInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return GlobalCampaignWorkerLease{}, fmt.Errorf("begin worker lease acquisition: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lockResult any
	if err := tx.QueryRowContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "global-campaign-worker:"+leaseKey).Scan(&lockResult); err != nil {
		return GlobalCampaignWorkerLease{}, fmt.Errorf("lock worker lease %q: %w", leaseKey, err)
	}

	current, found, err := loadGlobalCampaignWorkerLeaseTx(ctx, tx, leaseKey, true)
	if err != nil {
		return GlobalCampaignWorkerLease{}, err
	}
	expiresAt := now.Add(ttl)
	if !found {
		lease := GlobalCampaignWorkerLease{LeaseKey: leaseKey, LeaseOwner: owner, FencingToken: 1, LeaseExpiresAt: expiresAt, AcquiredAt: now, UpdatedAt: now}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO global_campaign_worker_leases
			(lease_key, lease_owner, fencing_token, lease_expires_at, acquired_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6)`, lease.LeaseKey, lease.LeaseOwner, lease.FencingToken, lease.LeaseExpiresAt, lease.AcquiredAt, lease.UpdatedAt); err != nil {
			return GlobalCampaignWorkerLease{}, fmt.Errorf("insert worker lease: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return GlobalCampaignWorkerLease{}, fmt.Errorf("commit worker lease acquisition: %w", err)
		}
		return lease, nil
	}

	if current.LeaseExpiresAt.After(now) {
		if current.LeaseOwner != owner {
			return GlobalCampaignWorkerLease{}, ErrGlobalCampaignWorkerLeaseBusy
		}
		if err := tx.Commit(); err != nil {
			return GlobalCampaignWorkerLease{}, fmt.Errorf("commit idempotent worker lease acquisition: %w", err)
		}
		return current, nil
	}
	if current.FencingToken == math.MaxInt64 {
		return GlobalCampaignWorkerLease{}, ErrGlobalCampaignWorkerLeaseFenced
	}

	next := GlobalCampaignWorkerLease{
		LeaseKey:       leaseKey,
		LeaseOwner:     owner,
		FencingToken:   current.FencingToken + 1,
		LeaseExpiresAt: expiresAt,
		AcquiredAt:     now,
		UpdatedAt:      now,
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE global_campaign_worker_leases
		SET lease_owner=$2, fencing_token=$3, lease_expires_at=$4, acquired_at=$5, updated_at=$6
		WHERE lease_key=$1`, next.LeaseKey, next.LeaseOwner, next.FencingToken, next.LeaseExpiresAt, next.AcquiredAt, next.UpdatedAt); err != nil {
		return GlobalCampaignWorkerLease{}, fmt.Errorf("take over worker lease: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return GlobalCampaignWorkerLease{}, fmt.Errorf("commit worker lease takeover: %w", err)
	}
	return next, nil
}

func RenewGlobalCampaignWorkerLease(ctx context.Context, db *sql.DB, lease GlobalCampaignWorkerLease, now time.Time, ttl time.Duration) (GlobalCampaignWorkerLease, error) {
	now = now.UTC()
	if db == nil || strings.TrimSpace(lease.LeaseKey) == "" || strings.TrimSpace(lease.LeaseOwner) == "" || lease.FencingToken <= 0 || now.IsZero() || ttl <= 0 {
		return GlobalCampaignWorkerLease{}, ErrGlobalCampaignWorkerLeaseInvalid
	}
	result, err := db.ExecContext(ctx, `
		UPDATE global_campaign_worker_leases
		SET lease_expires_at=$4, updated_at=$3
		WHERE lease_key=$1 AND lease_owner=$2 AND fencing_token=$5 AND lease_expires_at > $3`,
		lease.LeaseKey, lease.LeaseOwner, now, now.Add(ttl), lease.FencingToken)
	if err != nil {
		return GlobalCampaignWorkerLease{}, fmt.Errorf("renew worker lease: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return GlobalCampaignWorkerLease{}, fmt.Errorf("inspect worker lease renewal: %w", err)
	}
	if rows != 1 {
		return GlobalCampaignWorkerLease{}, ErrGlobalCampaignWorkerLeaseFenced
	}
	lease.LeaseExpiresAt = now.Add(ttl)
	lease.UpdatedAt = now
	return lease, nil
}

func AssertGlobalCampaignWorkerLease(ctx context.Context, db *sql.DB, lease GlobalCampaignWorkerLease, now time.Time) error {
	now = now.UTC()
	if db == nil || strings.TrimSpace(lease.LeaseKey) == "" || strings.TrimSpace(lease.LeaseOwner) == "" || lease.FencingToken <= 0 || now.IsZero() {
		return ErrGlobalCampaignWorkerLeaseInvalid
	}
	var expiresAt time.Time
	err := db.QueryRowContext(ctx, `
		SELECT lease_expires_at FROM global_campaign_worker_leases
		WHERE lease_key=$1 AND lease_owner=$2 AND fencing_token=$3`, lease.LeaseKey, lease.LeaseOwner, lease.FencingToken).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGlobalCampaignWorkerLeaseFenced
	}
	if err != nil {
		return fmt.Errorf("assert worker lease: %w", err)
	}
	if !expiresAt.After(now) {
		return ErrGlobalCampaignWorkerLeaseExpired
	}
	return nil
}

func ReleaseGlobalCampaignWorkerLease(ctx context.Context, db *sql.DB, lease GlobalCampaignWorkerLease, now time.Time) error {
	now = now.UTC()
	if err := AssertGlobalCampaignWorkerLease(ctx, db, lease, now); err != nil {
		return err
	}
	// Keep the lease row as a fencing tombstone. Deleting it would allow the
	// next acquisition to restart at token=1, so a stale worker from an older
	// process lifetime could accidentally become valid again after a release.
	// Expiring the row preserves the last token and forces every subsequent
	// acquisition to advance it monotonically.
	result, err := db.ExecContext(ctx, `
		UPDATE global_campaign_worker_leases
		SET lease_expires_at=$4, updated_at=$4
		WHERE lease_key=$1 AND lease_owner=$2 AND fencing_token=$3 AND lease_expires_at > $4`,
		lease.LeaseKey, lease.LeaseOwner, lease.FencingToken, now)
	if err != nil {
		return fmt.Errorf("release worker lease: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect worker lease release: %w", err)
	}
	if rows != 1 {
		return ErrGlobalCampaignWorkerLeaseFenced
	}
	return nil
}

func loadGlobalCampaignWorkerLeaseTx(ctx context.Context, tx *sql.Tx, leaseKey string, forUpdate bool) (GlobalCampaignWorkerLease, bool, error) {
	query := `SELECT lease_key,lease_owner,fencing_token,lease_expires_at,acquired_at,updated_at FROM global_campaign_worker_leases WHERE lease_key=$1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var lease GlobalCampaignWorkerLease
	err := tx.QueryRowContext(ctx, query, leaseKey).Scan(&lease.LeaseKey, &lease.LeaseOwner, &lease.FencingToken, &lease.LeaseExpiresAt, &lease.AcquiredAt, &lease.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return GlobalCampaignWorkerLease{}, false, nil
	}
	if err != nil {
		return GlobalCampaignWorkerLease{}, false, fmt.Errorf("load worker lease: %w", err)
	}
	return lease, true, nil
}
