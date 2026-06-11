package handlers

import (
	"database/sql"
	"errors"
	"strings"
)

func (h *Handler) userCreditsAndRole(authSubject string) (bool, int, error) {
	var role string
	var credits int
	err := h.DB.QueryRow(`SELECT COALESCE(role,''), COALESCE(credits,0) FROM app_user_profiles WHERE auth_subject=$1`, authSubject).Scan(&role, &credits)
	if err != nil {
		return false, 0, err
	}
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "admin" || role == "owner" || role == "enterprise", credits, nil
}

func (h *Handler) applyCreditChargeTxWithReason(tx *sql.Tx, authSubject, email, reason string) error {
	var res sql.Result
	var err error
	if strings.TrimSpace(authSubject) != "" {
		res, err = tx.Exec(`UPDATE app_user_profiles SET credits = credits - 1, updated_at = now() WHERE auth_subject=$1 AND credits > 0`, authSubject)
	} else {
		res, err = tx.Exec(`UPDATE app_user_profiles SET credits = credits - 1, updated_at = now() WHERE lower(email)=lower($1) AND credits > 0`, email)
	}
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("insufficient credits")
	}
	_, err = tx.Exec(`INSERT INTO credit_events (email, amount, reason, event_type) VALUES (lower($1), -1, $2, $3)`, email, reason, reason)
	return err
}
