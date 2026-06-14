package handlers

import (
	"database/sql"
	"errors"
	"strings"
)

func (h *Handler) userCreditsAndRole(authSubject string, emails ...string) (bool, int, error) {
	authSubject = strings.TrimSpace(authSubject)
	email := ""
	if len(emails) > 0 {
		email = strings.ToLower(strings.TrimSpace(emails[0]))
	}

	var available int
	err := h.DB.QueryRow(`
		SELECT COALESCE((
			SELECT SUM(e.outputs_remaining)::int
			FROM entitlements e
			LEFT JOIN app_user_profiles p ON lower(p.email) = lower(e.email)
			WHERE e.status = 'active'
			  AND COALESCE(e.plan_id, '') <> 'free'
			  AND COALESCE(e.outputs_remaining, 0) > 0
			  AND (e.expires_at IS NULL OR e.expires_at > now())
			  AND (
				($1 <> '' AND p.auth_subject = $1)
				OR ($2 <> '' AND lower(e.email) = lower($2))
			  )
		), 0)`, authSubject, email).Scan(&available)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, nil
		}
		return false, 0, err
	}
	return false, available, nil
}

func (h *Handler) applyCreditChargeTxWithReason(tx *sql.Tx, authSubject, email, reason string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	authSubject = strings.TrimSpace(authSubject)

	if email == "" && authSubject != "" {
		_ = tx.QueryRow(`SELECT lower(email) FROM app_user_profiles WHERE auth_subject=$1`, authSubject).Scan(&email)
	}

	if email != "" {
		res, err := tx.Exec(`
			UPDATE entitlements
			SET outputs_remaining = GREATEST(outputs_remaining - 1, 0),
			    updated_at = now()
			WHERE id = (
				SELECT id
				FROM entitlements
				WHERE lower(email) = lower($1)
				  AND status = 'active'
				  AND COALESCE(plan_id, '') <> 'free'
				  AND COALESCE(outputs_remaining, 0) > 0
				  AND (expires_at IS NULL OR expires_at > now())
				ORDER BY outputs_remaining DESC, created_at DESC
				LIMIT 1
				FOR UPDATE
			)`, email)
		if err != nil {
			return err
		}
		if rows, err := res.RowsAffected(); err != nil {
			return err
		} else if rows == 1 {
			_, err = tx.Exec(`INSERT INTO credit_events (email, amount, reason, event_type) VALUES (lower($1), -1, $2, $3)`, email, reason, reason)
			return err
		}
	}

	return errors.New("active package output required")
}

func (h *Handler) consumePremiumOutput(authSubject, email, reason string) error {
	if h.DB == nil {
		return errors.New("database unavailable")
	}
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := h.applyCreditChargeTxWithReason(tx, authSubject, email, reason); err != nil {
		return err
	}
	return tx.Commit()
}

func (h *Handler) hasActivePaidPackage(authSubject, email string) (bool, error) {
	if h.DB == nil {
		return false, errors.New("database unavailable")
	}
	authSubject = strings.TrimSpace(authSubject)
	email = strings.ToLower(strings.TrimSpace(email))
	var active bool
	err := h.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM entitlements e
			LEFT JOIN app_user_profiles p ON lower(p.email) = lower(e.email)
			WHERE e.status = 'active'
			  AND COALESCE(e.plan_id, '') <> 'free'
			  AND (e.expires_at IS NULL OR e.expires_at > now())
			  AND (
				($1 <> '' AND p.auth_subject = $1)
				OR ($2 <> '' AND lower(e.email) = lower($2))
			  )
		)`, authSubject, email).Scan(&active)
	return active, err
}

func (h *Handler) requirePremiumOutput(authSubject string, emails ...string) (int, error) {
	if h.DB == nil {
		return 0, errors.New("database unavailable")
	}
	email := ""
	if len(emails) > 0 {
		email = strings.ToLower(strings.TrimSpace(emails[0]))
	}
	active, err := h.hasActivePaidPackage(authSubject, email)
	if err != nil {
		return 0, err
	}
	if !active {
		return 0, errors.New("active package required")
	}
	_, available, err := h.userCreditsAndRole(authSubject, email)
	if err != nil {
		return 0, err
	}
	if available <= 0 {
		return 0, errors.New("active package output required")
	}
	return available, nil
}
