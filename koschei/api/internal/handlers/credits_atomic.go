package handlers

import (
	"database/sql"
	"errors"
	"strings"
)

func (h *Handler) userCreditsAndRole(authSubject string) (bool, int, error) {
	var available int
	err := h.DB.QueryRow(`
		SELECT COALESCE((
			SELECT SUM(e.outputs_remaining)::int
			FROM entitlements e
			JOIN app_user_profiles p ON lower(p.email) = lower(e.email)
			WHERE p.auth_subject = $1
			  AND e.status = 'active'
			  AND COALESCE(e.plan_id, '') <> 'free'
			  AND COALESCE(e.outputs_remaining, 0) > 0
		), 0) + COALESCE((
			SELECT p.credits
			FROM app_user_profiles p
			WHERE p.auth_subject = $1
		), 0)`, authSubject).Scan(&available)
	if err != nil {
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
			SET outputs_remaining = outputs_remaining - 1,
			    updated_at = now()
			WHERE id = (
				SELECT id
				FROM entitlements
				WHERE lower(email) = lower($1)
				  AND status = 'active'
				  AND COALESCE(plan_id, '') <> 'free'
				  AND outputs_remaining > 0
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

	var res sql.Result
	var err error
	if authSubject != "" {
		res, err = tx.Exec(`UPDATE app_user_profiles SET credits = credits - 1, updated_at = now() WHERE auth_subject=$1 AND credits > 0`, authSubject)
	} else if email != "" {
		res, err = tx.Exec(`UPDATE app_user_profiles SET credits = credits - 1, updated_at = now() WHERE lower(email)=lower($1) AND credits > 0`, email)
	} else {
		return errors.New("insufficient outputs")
	}
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("insufficient outputs")
	}
	_, err = tx.Exec(`INSERT INTO credit_events (email, amount, reason, event_type) VALUES (lower($1), -1, $2, $3)`, email, reason, reason)
	return err
}
