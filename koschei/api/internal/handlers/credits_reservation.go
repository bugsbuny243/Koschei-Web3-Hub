package handlers

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type premiumOutputReservation struct {
	EntitlementID   string
	Email           string
	Reason          string
	QuotaDayKey     string
	QuotaEventReason string
	QuotaTier       string
	QuotaLimit      int
	QuotaResetAt    time.Time
}

func (h *Handler) reservePremiumOutput(ctx context.Context, authSubject, email, reason string) (premiumOutputReservation, error) {
	if h == nil || h.DB == nil {
		return premiumOutputReservation{}, errors.New("database unavailable")
	}
	authSubject = strings.TrimSpace(authSubject)
	email = strings.ToLower(strings.TrimSpace(email))
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "premium_output"
	}
	if email == "" {
		email = entitlementEmailFromSubject(authSubject)
	}
	if email == "" && authSubject != "" {
		_ = h.DB.QueryRowContext(ctx, `SELECT lower(email) FROM app_user_profiles WHERE auth_subject=$1`, authSubject).Scan(&email)
	}
	if email == "" {
		return premiumOutputReservation{}, errors.New("entitlement email unavailable")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return premiumOutputReservation{}, err
	}
	defer tx.Rollback()

	var entitlementID, normalizedEmail string
	err = tx.QueryRowContext(ctx, `
		UPDATE entitlements
		SET outputs_remaining = outputs_remaining - 1,
		    updated_at = now()
		WHERE id = (
			SELECT id
			FROM entitlements
			WHERE lower(email)=lower($1)
			  AND status='active'
			  AND COALESCE(plan_id,'') <> ''
			  AND COALESCE(plan_id,'') <> 'free'
			  AND COALESCE(outputs_remaining,0) > 0
			  AND (expires_at IS NULL OR expires_at > now())
			ORDER BY outputs_remaining DESC, created_at DESC
			LIMIT 1
			FOR UPDATE
		)
		RETURNING id::text, lower(email)
	`, email).Scan(&entitlementID, &normalizedEmail)
	if errors.Is(err, sql.ErrNoRows) {
		return premiumOutputReservation{}, errors.New("active package output required")
	}
	if err != nil {
		return premiumOutputReservation{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credit_events(email,amount,reason,event_type) VALUES(lower($1),-1,$2,$3)`, normalizedEmail, reason+"_reserved", "reserve"); err != nil {
		return premiumOutputReservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return premiumOutputReservation{}, err
	}
	return premiumOutputReservation{EntitlementID: entitlementID, Email: normalizedEmail, Reason: reason}, nil
}

// reserveKOSCHScanQuota uses the existing credit_events reservation ledger.
// Eligibility is established before this method; the tier controls the daily
// UTC limit. An advisory transaction lock makes concurrent reservations for the
// same account/day atomic.
func (h *Handler) reserveKOSCHScanQuota(ctx context.Context, authSubject, email, tier string, limit int, now time.Time) (premiumOutputReservation, scanQuotaStatus, error) {
	status := newScanQuotaStatus(tier, limit, now)
	if h == nil || h.DB == nil {
		return premiumOutputReservation{}, status, errors.New("database unavailable")
	}
	authSubject = strings.TrimSpace(authSubject)
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		email = entitlementEmailFromSubject(authSubject)
	}
	if email == "" && authSubject != "" {
		_ = h.DB.QueryRowContext(ctx, `SELECT lower(email) FROM app_user_profiles WHERE auth_subject=$1`, authSubject).Scan(&email)
	}
	if email == "" || limit <= 0 {
		return premiumOutputReservation{}, status, errors.New("quota identity unavailable")
	}

	start, reset, dayKey := utcQuotaWindow(now)
	status.ResetsAt = reset
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return premiumOutputReservation{}, status, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1)||':'||$2,0))`, email, dayKey); err != nil {
		return premiumOutputReservation{}, status, err
	}
	used, err := quotaUsedTx(ctx, tx, email, dayKey, start, reset)
	if err != nil {
		return premiumOutputReservation{}, status, err
	}
	status.Used = used
	status.Remaining = maxInt(limit-used, 0)
	if used >= limit {
		return premiumOutputReservation{}, status, errScanQuotaExceeded
	}
	id, err := quotaReservationID()
	if err != nil {
		return premiumOutputReservation{}, status, err
	}
	eventReason := dayKey + ":" + id
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credit_events(email,amount,reason,event_type)
		VALUES(lower($1),-1,$2,'kosch_quota_reserve')`, email, eventReason); err != nil {
		return premiumOutputReservation{}, status, err
	}
	if err := tx.Commit(); err != nil {
		return premiumOutputReservation{}, status, err
	}
	status.Used++
	status.Remaining = maxInt(limit-status.Used, 0)
	return premiumOutputReservation{
		Email: email, Reason: "kosch_daily_scan", QuotaDayKey: dayKey,
		QuotaEventReason: eventReason, QuotaTier: tier, QuotaLimit: limit, QuotaResetAt: reset,
	}, status, nil
}

func (h *Handler) koschScanQuotaStatus(ctx context.Context, email, tier string, limit int, now time.Time) (scanQuotaStatus, error) {
	status := newScanQuotaStatus(tier, limit, now)
	if h == nil || h.DB == nil || strings.TrimSpace(email) == "" || limit <= 0 {
		return status, nil
	}
	start, reset, dayKey := utcQuotaWindow(now)
	status.ResetsAt = reset
	var used int
	err := h.DB.QueryRowContext(ctx, `
		SELECT GREATEST(COALESCE(-SUM(amount),0),0)::int
		FROM credit_events
		WHERE lower(email)=lower($1)
		  AND reason LIKE $2 || ':%'
		  AND created_at >= $3 AND created_at < $4
		  AND event_type IN ('kosch_quota_reserve','kosch_quota_refund')`, email, dayKey, start, reset).Scan(&used)
	if err != nil {
		return status, err
	}
	status.Used = used
	status.Remaining = maxInt(limit-used, 0)
	return status, nil
}

func quotaUsedTx(ctx context.Context, tx *sql.Tx, email, dayKey string, start, reset time.Time) (int, error) {
	var used int
	err := tx.QueryRowContext(ctx, `
		SELECT GREATEST(COALESCE(-SUM(amount),0),0)::int
		FROM credit_events
		WHERE lower(email)=lower($1)
		  AND reason LIKE $2 || ':%'
		  AND created_at >= $3 AND created_at < $4
		  AND event_type IN ('kosch_quota_reserve','kosch_quota_refund')`, email, dayKey, start, reset).Scan(&used)
	return used, err
}

func (h *Handler) refundPremiumOutputReservation(ctx context.Context, reservation premiumOutputReservation, refundReason string) error {
	if h == nil || h.DB == nil {
		return nil
	}
	if strings.TrimSpace(reservation.QuotaEventReason) != "" {
		return h.refundKOSCHScanQuota(ctx, reservation)
	}
	if strings.TrimSpace(reservation.EntitlementID) == "" {
		return nil
	}
	refundReason = strings.TrimSpace(refundReason)
	if refundReason == "" {
		refundReason = reservation.Reason + "_refund"
	}
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE entitlements
		SET outputs_remaining = LEAST(outputs_remaining + 1, outputs_total),
		    updated_at = now()
		WHERE id=$1::uuid
	`, reservation.EntitlementID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("reserved entitlement not found")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credit_events(email,amount,reason,event_type) VALUES(lower($1),1,$2,$3)`, reservation.Email, refundReason, "refund"); err != nil {
		return err
	}
	return tx.Commit()
}

func (h *Handler) refundKOSCHScanQuota(ctx context.Context, reservation premiumOutputReservation) error {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1)||':'||$2,0))`, reservation.Email, reservation.QuotaDayKey); err != nil {
		return err
	}
	var refunded bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM credit_events
			WHERE lower(email)=lower($1) AND reason=$2 AND event_type='kosch_quota_refund'
		)`, reservation.Email, reservation.QuotaEventReason).Scan(&refunded); err != nil {
		return err
	}
	if refunded {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credit_events(email,amount,reason,event_type)
		VALUES(lower($1),1,$2,'kosch_quota_refund')`, reservation.Email, reservation.QuotaEventReason); err != nil {
		return err
	}
	return tx.Commit()
}
