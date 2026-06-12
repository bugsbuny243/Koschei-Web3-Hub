package handlers

import (
	"database/sql"
	"errors"
	"strings"
)

func (h *Handler) userCreditsAndRole(authSubject string) (bool, int, error) {
	var email, role string
	var ownerCredits int
	err := h.DB.QueryRow(`
		SELECT lower(COALESCE(email, '')), COALESCE(role, ''), COALESCE(credits, 0)
		FROM app_user_profiles
		WHERE auth_subject = $1`, authSubject).Scan(&email, &role, &ownerCredits)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, nil
		}
		return false, 0, err
	}

	isPrivileged := strings.EqualFold(role, "owner") || strings.EqualFold(role, "admin")
	paidOutputs := 0
	if email != "" {
		if err := h.DB.QueryRow(`
			SELECT COALESCE(SUM(outputs_remaining), 0)::int
			FROM entitlements
			WHERE lower(email) = lower($1)
			  AND status = 'active'
			  AND COALESCE(plan_id, 'free') <> 'free'
			  AND outputs_remaining > 0`, email).Scan(&paidOutputs); err != nil {
			return false, 0, err
		}
	}
	return isPrivileged, paidOutputs + ownerCredits, nil
}

func (h *Handler) applyCreditChargeTxWithReason(tx *sql.Tx, authSubject, email, reason string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" && strings.TrimSpace(authSubject) != "" {
		_ = tx.QueryRow(`SELECT lower(COALESCE(email, '')) FROM app_user_profiles WHERE auth_subject=$1`, authSubject).Scan(&normalizedEmail)
	}
	if normalizedEmail != "" {
		res, err := tx.Exec(`
			UPDATE entitlements
			SET outputs_remaining = GREATEST(outputs_remaining - 1, 0),
				updated_at = now()
			WHERE id = (
				SELECT id
				FROM entitlements
				WHERE lower(email) = lower($1)
				  AND status = 'active'
				  AND COALESCE(plan_id, 'free') <> 'free'
				  AND outputs_remaining > 0
				ORDER BY outputs_remaining DESC, created_at DESC
				LIMIT 1
				FOR UPDATE
			)`, normalizedEmail)
		if err != nil {
			return err
		}
		if rows, err := res.RowsAffected(); err != nil {
			return err
		} else if rows == 1 {
			_, err = tx.Exec(`INSERT INTO credit_events (email, amount, reason, event_type) VALUES (lower($1), -1, $2, $3)`, normalizedEmail, reason, reason)
			return err
		}
	}

	var res sql.Result
	var err error
	if strings.TrimSpace(authSubject) != "" {
		res, err = tx.Exec(`UPDATE app_user_profiles SET credits = credits - 1, updated_at = now() WHERE auth_subject=$1 AND credits > 0`, authSubject)
	} else {
		res, err = tx.Exec(`UPDATE app_user_profiles SET credits = credits - 1, updated_at = now() WHERE lower(email)=lower($1) AND credits > 0`, normalizedEmail)
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
	_, err = tx.Exec(`INSERT INTO credit_events (email, amount, reason, event_type) VALUES (lower($1), -1, $2, $3)`, normalizedEmail, reason, reason)
	return err
}
