package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type adminSummary struct {
	UsersCount                  int64      `json:"users_count"`
	TotalRegisteredUsers        int64      `json:"total_registered_users"`
	ActiveUsersToday            int64      `json:"active_users_today"`
	ActiveUsers7Days            int64      `json:"active_users_7d"`
	ActiveUsers30Days           int64      `json:"active_users_30d"`
	NewUsersToday               int64      `json:"new_users_today"`
	NewUsers7Days               int64      `json:"new_users_7d"`
	FreeMembers                 int64      `json:"free_members"`
	PaidMembers                 int64      `json:"paid_members"`
	UsersWithZeroOutputs        int64      `json:"users_with_zero_outputs"`
	UsersWithOutputsRemaining   int64      `json:"users_with_outputs_remaining"`
	OutputsUsed24h              int64      `json:"outputs_used_24h"`
	ChainChecks24h              int64      `json:"chain_checks_24h"`
	FailedChecks24h             int64      `json:"failed_checks_24h"`
	ActiveEntitlementsCount     int64      `json:"active_entitlements_count"`
	TotalOutputsRemaining       int64      `json:"total_outputs_remaining"`
	Web3OutputsCount            int64      `json:"web3_outputs_count"`
	PendingPaymentRequestsCount int64      `json:"pending_payment_requests_count"`
	WatchlistSourcesCount       int64      `json:"watchlist_sources_count"`
	Web3EventsCount             int64      `json:"web3_events_count"`
	ChainHealthLogsCount        int64      `json:"chain_health_logs_count"`
	AnalyticsEventsCount        int64      `json:"analytics_events_count"`
	LatestLoginTime             *time.Time `json:"latest_login_time"`
	LatestOutputTime            *time.Time `json:"latest_output_time"`
	LatestChainCheckTime        *time.Time `json:"latest_chain_check_time"`
}

type adminCheck struct {
	Name    string         `json:"name"`
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

type adminScan struct {
	OK     bool         `json:"ok"`
	Status string       `json:"status"`
	Checks []adminCheck `json:"checks"`
}

var adminTables = map[string][]string{
	"users":             {"auth_subject", "email", "role", "credits", "created_at", "updated_at", "last_login_at"},
	"payments":          {"id", "email", "full_name", "product_id", "amount_try", "currency", "status", "created_at", "reviewed_at"},
	"entitlements":      {"id", "customer_id", "email", "plan_id", "payment_request_id", "outputs_total", "outputs_remaining", "status", "created_at", "updated_at"},
	"outputs":           {"id", "email", "entitlement_id", "output_type", "title", "ecosystem", "used_ai", "used_fallback", "created_at"},
	"watchlist-sources": {"id", "user_id", "email", "name", "label", "provider", "chain", "network", "address", "source_type", "status", "provider_setup_status", "is_active", "last_event_at", "created_at", "updated_at"},
	"web3-events":       {"id", "source_id", "user_id", "email", "provider", "chain", "network", "event_type", "address", "tx_hash", "block_number", "direction", "asset_type", "amount", "verification_status", "status", "created_at"},
	"chain-health":      {"id", "chain", "network", "provider", "ok", "healthy", "result", "status", "error", "error_message", "checked_at", "created_at"},
	"analytics":         {"id", "event_name", "email", "path", "referrer", "created_at"},
}

var adminTableNames = map[string]string{
	"users": "app_user_profiles", "payments": "payment_requests", "entitlements": "entitlements", "outputs": "web3_outputs",
	"watchlist-sources": "web3_event_sources", "web3-events": "web3_events", "chain-health": "chain_health_logs", "analytics": "analytics_events",
}

func (h *Handler) AdminSummary(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	summary, err := h.adminSummary(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "summary unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) AdminTable(w http.ResponseWriter, r *http.Request, name string) {
	if !h.ownerAuth(w, r) {
		return
	}
	rows, err := h.adminRows(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "admin data unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": rows})
}

func (h *Handler) AdminSystemScan(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, h.adminSystemScan(r.Context()))
}

func (h *Handler) AdminChat(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Message) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message required"})
		return
	}
	message := strings.TrimSpace(req.Message)
	summary, summaryErr := h.adminSummary(r.Context())
	scan := h.adminSystemScan(r.Context())
	answer := adminChatAnswer(message, summary, summaryErr, scan)
	// Admin chat is deliberately read-only: it summarizes in-memory results and never writes to the database.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "answer": answer, "actions": []any{}, "used_context": map[string]bool{"summary": summaryErr == nil, "system_scan": true}})
}

func (h *Handler) adminSummary(ctx context.Context) (adminSummary, error) {
	var s adminSummary
	queries := []struct {
		dst   *int64
		query string
	}{
		{&s.UsersCount, `SELECT count(*) FROM app_user_profiles`},
		{&s.TotalRegisteredUsers, `SELECT count(DISTINCT identity) FROM (SELECT lower(email) AS identity FROM app_user_profiles WHERE COALESCE(email, '') <> '' UNION SELECT lower(email) AS identity FROM entitlements WHERE COALESCE(email, '') <> '') u`},
		{&s.ActiveUsersToday, `SELECT count(DISTINCT COALESCE(NULLIF(lower(email), ''), path)) FROM analytics_events WHERE created_at >= CURRENT_DATE`},
		{&s.ActiveUsers7Days, `SELECT count(DISTINCT COALESCE(NULLIF(lower(email), ''), path)) FROM analytics_events WHERE created_at >= now()-interval '7 days'`},
		{&s.ActiveUsers30Days, `SELECT count(DISTINCT COALESCE(NULLIF(lower(email), ''), path)) FROM analytics_events WHERE created_at >= now()-interval '30 days'`},
		{&s.NewUsersToday, `SELECT count(*) FROM app_user_profiles WHERE created_at >= CURRENT_DATE`},
		{&s.NewUsers7Days, `SELECT count(*) FROM app_user_profiles WHERE created_at >= now()-interval '7 days'`},
		{&s.FreeMembers, `SELECT count(*) FROM entitlements WHERE status='active' AND (plan_id IS NULL OR plan_id='free')`},
		{&s.PaidMembers, `SELECT count(*) FROM entitlements WHERE status='active' AND plan_id IS NOT NULL AND plan_id<>'free'`},
		{&s.UsersWithZeroOutputs, `SELECT count(DISTINCT lower(email)) FROM entitlements WHERE outputs_remaining <= 0`},
		{&s.UsersWithOutputsRemaining, `SELECT count(DISTINCT lower(email)) FROM entitlements WHERE outputs_remaining > 0`},
		{&s.ActiveEntitlementsCount, `SELECT count(*) FROM entitlements WHERE status='active'`},
		{&s.TotalOutputsRemaining, `SELECT COALESCE(sum(outputs_remaining),0)::bigint FROM entitlements WHERE status='active'`},
		{&s.Web3OutputsCount, `SELECT count(*) FROM web3_outputs`},
		{&s.OutputsUsed24h, `SELECT count(*) FROM web3_outputs WHERE created_at >= now()-interval '24 hours'`},
		{&s.PendingPaymentRequestsCount, `SELECT count(*) FROM payment_requests WHERE status='pending'`},
		{&s.WatchlistSourcesCount, `SELECT count(*) FROM web3_event_sources`},
		{&s.Web3EventsCount, `SELECT count(*) FROM web3_events`},
		{&s.ChainHealthLogsCount, `SELECT count(*) FROM chain_health_logs`},
		{&s.AnalyticsEventsCount, `SELECT count(*) FROM analytics_events`},
	}
	for _, q := range queries {
		if exists, _ := h.tableExists(ctx, tableFromCountQuery(q.query)); exists {
			if err := h.DB.QueryRowContext(ctx, q.query).Scan(q.dst); err != nil {
				return s, err
			}
		}
	}
	s.LatestLoginTime = h.latestTime(ctx, `SELECT max(created_at) FROM analytics_events WHERE event_name='login_success'`)
	s.LatestOutputTime = h.latestTime(ctx, `SELECT max(created_at) FROM web3_outputs`)
	columns, _ := h.tableColumns(ctx, "chain_health_logs")
	if columns["checked_at"] {
		s.LatestChainCheckTime = h.latestTime(ctx, `SELECT max(checked_at) FROM chain_health_logs`)
	} else if columns["created_at"] {
		s.LatestChainCheckTime = h.latestTime(ctx, `SELECT max(created_at) FROM chain_health_logs`)
	}
	okCol := ""
	if columns["ok"] {
		okCol = "ok"
	} else if columns["healthy"] {
		okCol = "healthy"
	}
	timeCol := ""
	if columns["checked_at"] {
		timeCol = "checked_at"
	} else if columns["created_at"] {
		timeCol = "created_at"
	}
	if okCol != "" && timeCol != "" {
		_ = h.DB.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM chain_health_logs WHERE %s >= now()-interval '24 hours'`, timeCol)).Scan(&s.ChainChecks24h)
		_ = h.DB.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM chain_health_logs WHERE %s=false AND %s >= now()-interval '24 hours'`, okCol, timeCol)).Scan(&s.FailedChecks24h)
	}
	return s, nil
}

func tableFromCountQuery(query string) string {
	parts := strings.Fields(query)
	for i, part := range parts {
		if strings.EqualFold(part, "FROM") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func (h *Handler) latestTime(ctx context.Context, query string) *time.Time {
	var value sql.NullTime
	if err := h.DB.QueryRowContext(ctx, query).Scan(&value); err != nil || !value.Valid {
		return nil
	}
	return &value.Time
}

func (h *Handler) adminRows(ctx context.Context, name string) ([]map[string]any, error) {
	table := adminTableNames[name]
	columns, err := h.tableColumns(ctx, table)
	if err != nil {
		return nil, err
	}
	selected := make([]string, 0, len(adminTables[name]))
	for _, column := range adminTables[name] {
		if columns[column] {
			selected = append(selected, column)
		}
	}
	if len(selected) == 0 {
		return []map[string]any{}, nil
	}
	order := selected[0]
	for _, candidate := range []string{"created_at", "checked_at", "updated_at", "id"} {
		if columns[candidate] {
			order = candidate
			break
		}
	}
	query := fmt.Sprintf(`SELECT COALESCE(jsonb_agg(row_to_json(t)), '[]'::jsonb) FROM (SELECT %s FROM %s ORDER BY %s DESC LIMIT 200) t`, strings.Join(selected, ","), table, order)
	var raw []byte
	if err := h.DB.QueryRowContext(ctx, query).Scan(&raw); err != nil {
		return nil, err
	}
	rows := []map[string]any{}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (h *Handler) tableExists(ctx context.Context, table string) (bool, error) {
	var exists bool
	err := h.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=ANY(current_schemas(false)) AND table_name=$1)`, table).Scan(&exists)
	return exists, err
}

func (h *Handler) tableColumns(ctx context.Context, table string) (map[string]bool, error) {
	rows, err := h.DB.QueryContext(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema=ANY(current_schemas(false)) AND table_name=$1`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		columns[column] = true
	}
	return columns, rows.Err()
}

func (h *Handler) adminSystemScan(ctx context.Context) adminScan {
	checks := make([]adminCheck, 0, 20)
	add := func(name, status, message string, details map[string]any) {
		if details == nil {
			details = map[string]any{}
		}
		checks = append(checks, adminCheck{Name: name, Status: status, Message: message, Details: details})
	}
	required := []string{"app_user_profiles", "entitlements", "web3_outputs", "chain_health_logs", "web3_event_sources", "web3_events", "payment_requests", "analytics_events"}
	for _, table := range required {
		exists, err := h.tableExists(ctx, table)
		if err != nil || !exists {
			add("table: "+table, "critical", "Required database table is missing or unavailable.", map[string]any{"exists": false})
		} else {
			add("table: "+table, "ok", "Required database table exists.", map[string]any{"exists": true})
		}
	}
	envs := []struct {
		name     string
		present  bool
		severity string
	}{
		{"DATABASE_URL", strings.TrimSpace(os.Getenv("DATABASE_URL")) != "", "critical"}, {"ALCHEMY_API_KEY", strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")) != "", "warning"},
		{"ADMIN_PASSWORD", strings.TrimSpace(h.AdminPassword) != "", "critical"}, {"TOGETHER_API_KEY", strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) != "", "warning"},
	}
	for _, env := range envs {
		status, message := "ok", "Environment setting is present."
		if !env.present {
			status, message = env.severity, "Environment setting is missing."
		}
		add("env: "+env.name, status, message, map[string]any{"present": env.present})
	}

	metric := func(name, query string, warnAbove int64, criticalBelowZero bool) {
		var count int64
		if err := h.DB.QueryRowContext(ctx, query).Scan(&count); err != nil {
			add(name, "warning", "Check could not be completed.", map[string]any{})
			return
		}
		status := "ok"
		if (warnAbove >= 0 && count > warnAbove) || (criticalBelowZero && count > 0) {
			status = "warning"
		}
		add(name, status, fmt.Sprintf("Count: %d", count), map[string]any{"count": count})
	}
	metric("pending payments", `SELECT count(*) FROM payment_requests WHERE status='pending'`, 0, false)
	metric("negative output balances", `SELECT count(*) FROM entitlements WHERE outputs_remaining < 0`, 0, true)
	metric("entitlements missing email", `SELECT count(*) FROM entitlements WHERE email IS NULL OR btrim(email)=''`, 0, false)
	metric("watchlist sources missing identity", `SELECT count(*) FROM web3_event_sources WHERE email IS NULL OR btrim(email)='' OR address IS NULL OR btrim(address)=''`, 0, false)
	metric("demo/test web3 events", `SELECT count(*) FROM web3_events WHERE lower(COALESCE(event_type,'')) LIKE '%test%' OR lower(COALESCE(event_type,'')) LIKE '%demo%'`, 0, false)
	metric("analytics events in last 24h", `SELECT count(*) FROM analytics_events WHERE created_at >= now()-interval '24 hours'`, -1, false)
	metric("outputs in last 24h", `SELECT count(*) FROM web3_outputs WHERE created_at >= now()-interval '24 hours'`, -1, false)
	columns, _ := h.tableColumns(ctx, "chain_health_logs")
	okCol, timeCol := "", ""
	if columns["ok"] {
		okCol = "ok"
	} else if columns["healthy"] {
		okCol = "healthy"
	}
	if columns["checked_at"] {
		timeCol = "checked_at"
	} else if columns["created_at"] {
		timeCol = "created_at"
	}
	if okCol != "" && timeCol != "" {
		metric("successful chain checks in last 24h", fmt.Sprintf(`SELECT count(*) FROM chain_health_logs WHERE %s=true AND %s >= now()-interval '24 hours'`, okCol, timeCol), -1, false)
		metric("failed chain checks in last 24h", fmt.Sprintf(`SELECT count(*) FROM chain_health_logs WHERE %s=false AND %s >= now()-interval '24 hours'`, okCol, timeCol), 0, false)
	} else {
		add("recent chain health", "warning", "Compatible chain health status/time columns were not found.", map[string]any{})
	}
	status := "healthy"
	for _, check := range checks {
		if check.Status == "critical" {
			status = "critical"
			break
		}
		if check.Status == "warning" {
			status = "warning"
		}
	}
	return adminScan{OK: true, Status: status, Checks: checks}
}

func adminChatAnswer(message string, summary adminSummary, summaryErr error, scan adminScan) string {
	m := strings.ToLower(message)
	severity := "✅ OK"
	if scan.Status == "warning" {
		severity = "⚠️ Warning"
	} else if scan.Status == "critical" {
		severity = "❌ Critical"
	}
	problems := make([]string, 0)
	for _, check := range scan.Checks {
		if check.Status != "ok" {
			problems = append(problems, check.Name+": "+check.Message)
		}
	}
	sort.Strings(problems)
	next := "Next action: review Members, Packages, or Koschei Chat in the simplified admin panel."
	if summaryErr != nil {
		return "❌ Critical — Summary data could not be read. " + next
	}
	switch {
	case strings.Contains(m, "modül") || strings.Contains(m, "module"):
		return "✅ OK — The public product is locked to six Pro production modules: TX Decoder, Token Scanner / Rug Checker, Wallet Score / Reputation, Risk Scanner, Portfolio Tracker, and Project Radar. Admin module screens are hidden."
	case strings.Contains(m, "grant başvuru") || strings.Contains(m, "grant application"):
		return "✅ OK — Grant Autopilot is no longer exposed in the active product surface."
	case strings.Contains(m, "impact"):
		return "✅ OK — Public impact is no longer an active module page in the simplified product surface."
	case strings.Contains(m, "agent api"):
		return "✅ OK — Agent API is not exposed in the active six-module product surface."
	case strings.Contains(m, "x402"):
		return "✅ OK — Credit-based Pro access is active; x402 remains disabled."
	case strings.Contains(m, "cross-chain") || strings.Contains(m, "cross chain"):
		return "✅ OK — Cross-chain Risk Monitor is not exposed; use Risk Scanner for general risk review."
	case strings.Contains(m, "sybil"):
		return "✅ OK — Sybil Check is not exposed in the active product surface."
	case strings.Contains(m, "risk engine") || strings.Contains(m, "risk v2"):
		return "✅ OK — Risk Scanner is the active risk module; Risk Engine v2 is not exposed."
	case strings.Contains(m, "ödeme") || strings.Contains(m, "payment") || strings.Contains(m, "gelir"):
		return fmt.Sprintf("%s — %d pending payment request(s). Approved/rejected history is available in Payments. %s", severity, summary.PendingPaymentRequestsCount, next)
	case strings.Contains(m, "kredi") || strings.Contains(m, "credit"):
		return fmt.Sprintf("%s — %d active entitlement(s), %d total outputs remaining. %s", severity, summary.ActiveEntitlementsCount, summary.TotalOutputsRemaining, next)
	case strings.Contains(m, "wallet") || strings.Contains(m, "token") || strings.Contains(m, "transaction"):
		return fmt.Sprintf("%s — Six Pro modules are active and credit-gated. Total generated outputs: %d. %s", severity, summary.Web3OutputsCount, next)
	case strings.Contains(m, "alchemy") || strings.Contains(m, "chain"):
		return fmt.Sprintf("%s — %d chain health log(s); latest check: %s. %s", severity, summary.ChainHealthLogsCount, formatAdminTime(summary.LatestChainCheckTime), next)
	case strings.Contains(m, "bugün") || strings.Contains(m, "today") || strings.Contains(m, "aktiv") || strings.Contains(m, "activity"):
		return fmt.Sprintf("%s — Totals: %d analytics events, %d outputs, %d web3 events. Latest login: %s; latest output: %s. %s", severity, summary.AnalyticsEventsCount, summary.Web3OutputsCount, summary.Web3EventsCount, formatAdminTime(summary.LatestLoginTime), formatAdminTime(summary.LatestOutputTime), next)
	case strings.Contains(m, "hata") || strings.Contains(m, "error") || strings.Contains(m, "tara") || strings.Contains(m, "scan"):
		if len(problems) == 0 {
			return "✅ OK — System scan found no warnings or critical issues. Next action: continue monitoring."
		}
		if len(problems) > 5 {
			problems = problems[:5]
		}
		return severity + " — " + strings.Join(problems, " | ") + ". " + next
	default:
		return fmt.Sprintf("%s — %d users, %d active entitlements, %d pending payments, %d outputs remaining. Ask me to scan the system or summarize payments, credits, watchlist, chains, activity, or errors. %s", severity, summary.UsersCount, summary.ActiveEntitlementsCount, summary.PendingPaymentRequestsCount, summary.TotalOutputsRemaining, next)
	}
}

func formatAdminTime(value *time.Time) string {
	if value == nil {
		return "not available"
	}
	return value.UTC().Format(time.RFC3339)
}
