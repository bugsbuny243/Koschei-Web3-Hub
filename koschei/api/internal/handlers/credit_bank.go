package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type insufficientCreditsError struct {
	Tool      string
	Cost      int
	Available int
}

func (e insufficientCreditsError) Error() string {
	return fmt.Sprintf("insufficient outputs for %s: required=%d available=%d", e.Tool, e.Cost, e.Available)
}

func ToolCreditCost(tool string) int {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "chain_health":
		return 0
	case "watchlist_add_source", "watchlist_sync", "wallet_score":
		return 1
	case "token_scanner", "token_scan", "tx_decoder", "tx_decode", "metadata_studio", "metadata", "risk_scanner", "risk_scan", "portfolio_tracker", "airdrop_checker":
		return 2
	case "program_scanner", "program_scan", "smart_money", "rug_radar", "project_radar":
		return 3
	case "risk_v2", "cross_chain_risk", "sybil_check", "tx_decoder_pro", "tx_decode_pro":
		return 4
	case "intelligence_graph", "funding_assistant", "artifact_generate", "artifact_generation", "runtime_project":
		return 5
	case "ai_generate", "ai_generation", "chat", "code", "reason", "build_analyzer", "game_design", "game_code":
		return 4
	default:
		return 1
	}
}

func insufficientOutputsResponse(values ...int) map[string]any {
	cost := 1
	available := 0
	if len(values) > 0 {
		cost = values[0]
	}
	if len(values) > 1 {
		available = values[1]
	}
	return map[string]any{
		"error":             "insufficient_outputs",
		"message":           "This tool requires credits. Please buy credits to continue.",
		"required_credits":  cost,
		"available_credits": available,
	}
}

func (h *Handler) CheckCredits(ctx context.Context, email, tool string) (int, int, error) {
	cost := ToolCreditCost(tool)
	if cost <= 0 {
		return 0, 0, nil
	}
	var available int
	err := h.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(outputs_remaining),0)::int FROM entitlements WHERE lower(email)=lower($1) AND status='active'`, strings.TrimSpace(email)).Scan(&available)
	return cost, available, err
}

func (h *Handler) RequireCredits(w http.ResponseWriter, r *http.Request, claims neonJWTClaims, tool string) (bool, int, int, bool) {
	isPrivileged, _, err := h.userCreditsAndRole(claims.Sub)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return false, 0, 0, false
	}
	cost, available, err := h.CheckCredits(r.Context(), claims.Email, tool)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return false, cost, available, isPrivileged
	}
	if !isPrivileged && available < cost {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse(cost, available))
		return false, cost, available, isPrivileged
	}
	return true, cost, available, isPrivileged
}

func (h *Handler) ChargeCreditsTx(ctx context.Context, tx *sql.Tx, email, tool string) error {
	cost := ToolCreditCost(tool)
	if cost <= 0 {
		return nil
	}
	type entitlementBalance struct {
		ID        any
		Remaining int
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, outputs_remaining
		FROM entitlements
		WHERE lower(email)=lower($1) AND status='active' AND outputs_remaining > 0
		ORDER BY outputs_remaining DESC, created_at DESC
		FOR UPDATE`, strings.TrimSpace(email))
	if err != nil {
		return err
	}
	defer rows.Close()
	balances := []entitlementBalance{}
	available := 0
	for rows.Next() {
		var item entitlementBalance
		if err := rows.Scan(&item.ID, &item.Remaining); err != nil {
			return err
		}
		available += item.Remaining
		balances = append(balances, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if available < cost {
		return insufficientCreditsError{Tool: tool, Cost: cost, Available: available}
	}
	remainingToCharge := cost
	for _, balance := range balances {
		if remainingToCharge <= 0 {
			break
		}
		deduct := balance.Remaining
		if deduct > remainingToCharge {
			deduct = remainingToCharge
		}
		if _, err := tx.ExecContext(ctx, `UPDATE entitlements SET outputs_remaining=outputs_remaining-$2, updated_at=now() WHERE id=$1`, balance.ID, deduct); err != nil {
			return err
		}
		remainingToCharge -= deduct
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO credit_events (email, amount, reason, created_at) VALUES (lower($1), $2, $3, now())`, email, -cost, strings.ToLower(strings.TrimSpace(tool)))
	return err
}

func (h *Handler) userCreditsAndRole(authSub string) (bool, int, error) {
	var role, email string
	if err := h.DB.QueryRow(`SELECT COALESCE(role,''), COALESCE(email,'') FROM app_user_profiles WHERE auth_subject=$1`, authSub).Scan(&role, &email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, nil
		}
		return false, 0, err
	}
	role = strings.ToLower(strings.TrimSpace(role))
	email = strings.ToLower(strings.TrimSpace(email))
	var outputs int
	if email != "" {
		if err := h.DB.QueryRow(`SELECT COALESCE(SUM(outputs_remaining),0)::int FROM entitlements WHERE lower(email)=lower($1) AND status='active'`, email).Scan(&outputs); err != nil {
			return role == "owner", 0, err
		}
	}
	return role == "owner", outputs, nil
}

func (h *Handler) isPrivilegedEmail(email string) bool {
	var role string
	if err := h.DB.QueryRow(`SELECT COALESCE(role,'') FROM app_user_profiles WHERE lower(email)=lower($1) ORDER BY updated_at DESC LIMIT 1`, email).Scan(&role); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(role), "owner")
}

func (h *Handler) spendOutput(email, tool string) error {
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := h.ChargeCreditsTx(context.Background(), tx, email, tool); err != nil {
		return err
	}
	return tx.Commit()
}

func (h *Handler) useOutput(w http.ResponseWriter, email, tool string) bool {
	if h.isPrivilegedEmail(email) {
		return true
	}
	if err := h.spendOutput(email, tool); err != nil {
		var insufficient insufficientCreditsError
		if errors.As(err, &insufficient) {
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse(insufficient.Cost, insufficient.Available))
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		}
		return false
	}
	return true
}
