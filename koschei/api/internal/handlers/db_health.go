package handlers

import "net/http"

func (h *Handler) OwnerDBHealth(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}

	requiredTables := []string{
		"schema_migrations",
		"plans",
		"payment_requests",
		"credit_events",
		"generation_jobs",
		"runtime_projects",
		"runtime_tasks",
		"runtime_logs",
		"model_route_logs",
	}

	missing := []string{}
	present := []string{}
	for _, table := range requiredTables {
		var ok bool
		if err := h.DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`, table).Scan(&ok); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		if ok {
			present = append(present, table)
		} else {
			missing = append(missing, table)
		}
	}

	planIDs := []string{}
	rows, err := h.DB.Query(`SELECT id FROM plans ORDER BY id ASC`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		planIDs = append(planIDs, id)
	}
	rows.Close()

	paymentByStatus := map[string]int{}
	rows, err = h.DB.Query(`SELECT status, COUNT(*) FROM payment_requests GROUP BY status ORDER BY status`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		paymentByStatus[status] = count
	}
	rows.Close()

	var runtimeProjectsCount int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM runtime_projects`).Scan(&runtimeProjectsCount); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	runtimeTasksByStatus := map[string]int{}
	rows, err = h.DB.Query(`SELECT status, COUNT(*) FROM runtime_tasks GROUP BY status ORDER BY status`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		runtimeTasksByStatus[status] = count
	}
	rows.Close()

	var creditEventsRowCount int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM credit_events`).Scan(&creditEventsRowCount); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	legacyTables := []string{"credit_ledger", "app_users", "users", "user_projects", "auth_accounts"}
	legacyPresence := map[string]bool{}
	for _, table := range legacyTables {
		var ok bool
		if err := h.DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`, table).Scan(&ok); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		legacyPresence[table] = ok
	}

	writeJSON(w, 200, map[string]any{
		"required_active_tables": map[string]any{
			"present": present,
			"missing": missing,
		},
		"plan_ids":                      planIDs,
		"payment_requests_by_status":    paymentByStatus,
		"runtime_projects_count":        runtimeProjectsCount,
		"runtime_tasks_by_status":       runtimeTasksByStatus,
		"credit_events_row_count":       creditEventsRowCount,
		"legacy_removed_tables_present": legacyPresence,
	})
}
