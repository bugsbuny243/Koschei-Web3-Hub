package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type chainHealthLog struct {
	Chain     string    `json:"chain"`
	Network   string    `json:"network"`
	Provider  string    `json:"provider"`
	Healthy   bool      `json:"healthy"`
	Result    string    `json:"result"`
	Error     string    `json:"error"`
	CheckedAt time.Time `json:"checked_at"`
}

func (h *Handler) recordChainHealth(logEntry chainHealthLog) {
	if h.DB == nil {
		log.Printf("chain health log insert skipped: database unavailable")
		return
	}

	columns, columnsErr := chainHealthLogColumns(context.Background(), h.DB)
	if columnsErr != nil {
		log.Printf("chain health log insert failed: %v", columnsErr)
		return
	}
	okColumn := firstAvailableColumn(columns, "ok", "healthy")
	resultColumn := firstAvailableColumn(columns, "result", "status")
	errorColumn := firstAvailableColumn(columns, "error", "error_message")
	checkedAtColumn := firstAvailableColumn(columns, "checked_at", "created_at")
	if okColumn == "" || resultColumn == "" || errorColumn == "" || checkedAtColumn == "" {
		log.Printf("chain health log insert failed: unsupported chain_health_logs columns")
		return
	}
	if h.hasRecentChainHealthLog(logEntry, checkedAtColumn) {
		return
	}

	query := fmt.Sprintf(`
		INSERT INTO chain_health_logs (chain, network, provider, %s, %s, %s, %s)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, okColumn, resultColumn, errorColumn, checkedAtColumn)
	if _, err := h.DB.Exec(query, logEntry.Chain, logEntry.Network, logEntry.Provider, logEntry.Healthy, logEntry.Result, logEntry.Error, logEntry.CheckedAt); err != nil {
		log.Printf("chain health log insert failed: %v", err)
	}
}

func (h *Handler) hasRecentChainHealthLog(logEntry chainHealthLog, checkedAtColumn string) bool {
	query := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM chain_health_logs
			WHERE lower(chain) = lower($1)
			  AND lower(network) = lower($2)
			  AND lower(provider) = lower($3)
			  AND %s >= now() - interval '5 minutes'
		)`, checkedAtColumn)
	var exists bool
	if err := h.DB.QueryRow(query, logEntry.Chain, logEntry.Network, logEntry.Provider).Scan(&exists); err != nil {
		log.Printf("chain health recent check lookup failed: %v", err)
		return false
	}
	return exists
}

func (h *Handler) Web3HealthLogs(w http.ResponseWriter, r *http.Request) {
	columns, err := chainHealthLogColumns(r.Context(), h.DB)
	if err != nil {
		log.Printf("chain health logs query failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	okColumn := firstAvailableColumn(columns, "ok", "healthy")
	resultColumn := firstAvailableColumn(columns, "result", "status")
	errorColumn := firstAvailableColumn(columns, "error", "error_message")
	checkedAtColumn := firstAvailableColumn(columns, "checked_at", "created_at")
	if okColumn == "" || resultColumn == "" || errorColumn == "" || checkedAtColumn == "" {
		log.Printf("chain health logs query failed: unsupported chain_health_logs columns")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	query := fmt.Sprintf(`
		SELECT chain, network, provider, %s, COALESCE(%s, ''), COALESCE(%s, ''), %s
		FROM chain_health_logs
		ORDER BY %s DESC
		LIMIT 50`, okColumn, resultColumn, errorColumn, checkedAtColumn, checkedAtColumn)
	rows, err := h.DB.QueryContext(r.Context(), query)
	if err != nil {
		log.Printf("chain health logs query failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()

	logs := make([]chainHealthLog, 0, 50)
	for rows.Next() {
		var logEntry chainHealthLog
		if err := rows.Scan(&logEntry.Chain, &logEntry.Network, &logEntry.Provider, &logEntry.Healthy, &logEntry.Result, &logEntry.Error, &logEntry.CheckedAt); err != nil {
			log.Printf("chain health logs scan failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		logs = append(logs, logEntry)
	}
	if err := rows.Err(); err != nil {
		log.Printf("chain health logs iteration failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "logs": logs})
}

func chainHealthLogColumns(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = ANY(current_schemas(false))
		  AND table_name = 'chain_health_logs'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		columns[strings.ToLower(column)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("chain_health_logs table unavailable")
	}
	for _, required := range []string{"chain", "network", "provider"} {
		if !columns[required] {
			return nil, fmt.Errorf("chain_health_logs missing %s column", required)
		}
	}
	return columns, nil
}

func firstAvailableColumn(columns map[string]bool, candidates ...string) string {
	for _, candidate := range candidates {
		if columns[candidate] {
			return candidate
		}
	}
	return ""
}
