package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

const riskDisclaimer = "This is informational only, not legal, financial, investment, or security advice."

type scanRiskRequest struct {
	Target string `json:"target"`
	Notes  string `json:"notes"`
}

type riskScanResult struct {
	OK               bool     `json:"ok"`
	RiskLevel        string   `json:"risk_level"`
	Score            int      `json:"score"`
	Checklist        []string `json:"checklist"`
	RecommendedFixes []string `json:"recommended_fixes"`
	Disclaimer       string   `json:"disclaimer"`
	UsedAI           bool     `json:"used_ai"`
	UsedFallback     bool     `json:"used_fallback"`
}

func (h *Handler) ScanRisk(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req scanRiskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(claims.Email))
	target := strings.TrimSpace(req.Target)
	notes := strings.TrimSpace(req.Notes)
	if email == "" || target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	creditOK, _, _, isPrivileged := h.RequireCredits(w, r, claims, "risk_scanner")
	if !creditOK {
		return
	}

	riskResult := buildRiskScanResult()
	resultJSON, err := json.Marshal(riskResult)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "risk_scan_failed"})
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer tx.Rollback()

	if err := saveRiskOutput(r.Context(), tx, email, nil, target, riskResult, string(resultJSON)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	attemptRiskReportSave(r.Context(), tx, email, target, notes, riskResult, string(resultJSON))

	if !isPrivileged {
		if err := h.ChargeCreditsTx(r.Context(), tx, email, "risk_scanner"); err != nil {
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse(ToolCreditCost("risk_scanner"), 0))
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	writeJSON(w, http.StatusOK, riskResult)
}

func buildRiskScanResult() riskScanResult {
	return riskScanResult{
		OK:        true,
		RiskLevel: "review_required",
		Score:     65,
		Checklist: []string{
			"Verify the official project source and the target address independently.",
			"Review contract verification, permissions, upgrade controls, and transaction history.",
			"Check for unexpected approvals, signer changes, or high-privilege operations.",
			"Confirm that public documentation and audit claims are current and authentic.",
		},
		RecommendedFixes: []string{
			"Use trusted explorers and official links to confirm addresses before interacting.",
			"Revoke unnecessary token approvals and limit wallet permissions where appropriate.",
			"Use a separate wallet for testing and request an independent security review for high-value activity.",
		},
		Disclaimer:   riskDisclaimer,
		UsedAI:       false,
		UsedFallback: true,
	}
}

func (result riskScanResult) summary(target string) string {
	return fmt.Sprintf("Risk scan for %s\nRisk level: %s\nScore: %d\n\nChecklist:\n- %s\n\nRecommended fixes:\n- %s\n\nDisclaimer: %s", target, result.RiskLevel, result.Score, strings.Join(result.Checklist, "\n- "), strings.Join(result.RecommendedFixes, "\n- "), result.Disclaimer)
}

func saveRiskOutput(ctx context.Context, tx *sql.Tx, email string, entitlementID any, target string, result riskScanResult, resultJSON string) error {
	var hasEntitlementID bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
				AND table_name = 'web3_outputs'
				AND column_name = 'entitlement_id'
		)`).Scan(&hasEntitlementID); err != nil {
		return err
	}

	if hasEntitlementID {
		if _, err := tx.ExecContext(ctx, "SAVEPOINT risk_output_entitlement_id"); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO web3_outputs (email, entitlement_id, output_type, title, ecosystem, content_json, content_text, used_ai, used_fallback)
			VALUES ($1, $2, 'risk', $3, 'web3', $4::jsonb, $5, false, true)`, email, entitlementID, target, resultJSON, result.summary(target))
		if err == nil {
			_, err = tx.ExecContext(ctx, "RELEASE SAVEPOINT risk_output_entitlement_id")
			return err
		}
		if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT risk_output_entitlement_id"); rollbackErr != nil {
			return rollbackErr
		}
		if _, releaseErr := tx.ExecContext(ctx, "RELEASE SAVEPOINT risk_output_entitlement_id"); releaseErr != nil {
			return releaseErr
		}
		log.Printf("risk scan: web3_outputs entitlement_id insert unavailable, retrying compatible insert: %v", err)
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO web3_outputs (email, output_type, title, ecosystem, content_json, content_text, used_ai, used_fallback)
		VALUES ($1, 'risk', $2, 'web3', $3::jsonb, $4, false, true)`, email, target, resultJSON, result.summary(target))
	return err
}

func attemptRiskReportSave(ctx context.Context, tx *sql.Tx, email, target, notes string, result riskScanResult, resultJSON string) {
	if _, err := tx.ExecContext(ctx, "SAVEPOINT optional_risk_report"); err != nil {
		log.Printf("risk scan: cannot prepare optional risk_reports insert: %v", err)
		return
	}
	if err := saveRiskReportIfSupported(ctx, tx, email, target, notes, result, resultJSON); err != nil {
		if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT optional_risk_report"); rollbackErr != nil {
			log.Printf("risk scan: cannot recover from optional risk_reports insert failure: %v (original error: %v)", rollbackErr, err)
			return
		}
		log.Printf("risk scan: optional risk_reports insert skipped: %v", err)
	}
	if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT optional_risk_report"); err != nil {
		log.Printf("risk scan: cannot release optional risk_reports savepoint: %v", err)
	}
}

// saveRiskReportIfSupported writes a report only when a compatible risk_reports
// table is present. Some deployments predate that optional table, while all
// deployments persist the complete report in web3_outputs.
func saveRiskReportIfSupported(ctx context.Context, tx *sql.Tx, email, target, notes string, result riskScanResult, resultJSON string) error {
	var tableExists bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('public.risk_reports') IS NOT NULL`).Scan(&tableExists); err != nil || !tableExists {
		return err
	}

	rows, err := tx.QueryContext(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'risk_reports'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	available := map[string]bool{}
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return err
		}
		available[column] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	fields := []struct {
		name  string
		value any
	}{
		{"email", email}, {"target", target}, {"notes", notes}, {"risk_level", result.RiskLevel}, {"score", result.Score},
		{"checklist", mustJSON(result.Checklist)}, {"recommended_fixes", mustJSON(result.RecommendedFixes)}, {"disclaimer", result.Disclaimer},
		{"content_json", resultJSON}, {"used_ai", result.UsedAI}, {"used_fallback", result.UsedFallback},
	}
	columns := make([]string, 0, len(fields))
	placeholders := make([]string, 0, len(fields))
	values := make([]any, 0, len(fields))
	for _, field := range fields {
		if available[field.name] {
			columns = append(columns, field.name)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, field.value)
		}
	}
	if len(columns) == 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO risk_reports (%s) VALUES (%s)", strings.Join(columns, ", "), strings.Join(placeholders, ", ")), values...)
	return err
}

func mustJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
