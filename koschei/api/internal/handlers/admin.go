package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type adminUser struct {
	Email                string     `json:"email"`
	Plan                 string     `json:"plan"`
	OutputsTotal         int64      `json:"outputs_total"`
	OutputsRemaining     int64      `json:"outputs_remaining"`
	Status               string     `json:"status"`
	CreatedAt            *time.Time `json:"created_at,omitempty"`
	LastLoginAt          *time.Time `json:"last_login_at,omitempty"`
	LastActivityAt       *time.Time `json:"last_activity_at,omitempty"`
	OutputsUsedCount     int64      `json:"outputs_used_count"`
	WatchlistSourceCount int64      `json:"watchlist_source_count"`
}

func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	users, err := h.adminUsers(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")), strings.TrimSpace(r.URL.Query().Get("filter")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "users": users})
}

func (h *Handler) adminUsers(ctx context.Context, search, filter string) ([]adminUser, error) {
	profileColumns, err := tableColumns(ctx, h.DB, "app_user_profiles")
	if err != nil {
		return nil, err
	}
	lastLoginExpression := "NULL::timestamptz"
	for _, candidate := range []string{"last_login_at", "last_signed_in_at", "last_sign_in_at"} {
		if profileColumns[candidate] {
			lastLoginExpression = "u." + candidate
			break
		}
	}
	where := []string{"e.email <> ''"}
	args := []any{}
	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		where = append(where, fmt.Sprintf("e.email LIKE $%d", len(args)))
	}
	switch filter {
	case "free":
		where = append(where, "COALESCE(es.has_paid,false)=false")
	case "paid":
		where = append(where, "COALESCE(es.has_paid,false)=true")
	case "no_outputs":
		where = append(where, "COALESCE(es.outputs_remaining,0) <= 0")
	case "active_30d":
		where = append(where, "la.last_activity_at >= now()-interval '30 days'")
	}
	query := fmt.Sprintf(`
		WITH emails AS (
			SELECT lower(email) AS email FROM app_user_profiles WHERE COALESCE(email,'') <> ''
			UNION
			SELECT lower(email) AS email FROM entitlements WHERE COALESCE(email,'') <> ''
		), entitlement_summary AS (
			SELECT lower(email) AS email,
			       bool_or(status='active' AND plan_id IS NOT NULL AND plan_id <> 'free') AS has_paid,
			       COALESCE(string_agg(DISTINCT COALESCE(plan_id,'free'), ', ' ORDER BY COALESCE(plan_id,'free')) FILTER (WHERE status='active'), 'free') AS plan,
			       COALESCE(sum(outputs_total) FILTER (WHERE status='active'), 0)::bigint AS outputs_total,
			       COALESCE(sum(outputs_remaining) FILTER (WHERE status='active'), 0)::bigint AS outputs_remaining,
			       COALESCE(string_agg(DISTINCT status, ', ' ORDER BY status), 'none') AS status
			FROM entitlements GROUP BY lower(email)
		), latest_activity AS (
			SELECT lower(email) AS email, max(created_at) AS last_activity_at FROM analytics_events WHERE COALESCE(email,'') <> '' GROUP BY lower(email)
		), latest_login AS (
			SELECT lower(email) AS email, max(created_at) AS last_login_at FROM analytics_events WHERE event_name='login_success' AND COALESCE(email,'') <> '' GROUP BY lower(email)
		)
		SELECT e.email,
		       COALESCE(es.plan,'free'), COALESCE(es.outputs_total,0), COALESCE(es.outputs_remaining,0), COALESCE(es.status,'none'),
		       u.created_at, COALESCE(%s, ll.last_login_at), la.last_activity_at,
		       COALESCE((SELECT count(*) FROM web3_outputs o WHERE lower(o.email)=e.email),0)::bigint,
		       COALESCE((SELECT count(*) FROM web3_event_sources s WHERE lower(s.email)=e.email),0)::bigint
		FROM emails e
		LEFT JOIN app_user_profiles u ON lower(u.email)=e.email
		LEFT JOIN entitlement_summary es ON es.email=e.email
		LEFT JOIN latest_activity la ON la.email=e.email
		LEFT JOIN latest_login ll ON ll.email=e.email
		WHERE %s
		ORDER BY COALESCE(la.last_activity_at, u.created_at, now()-interval '100 years') DESC
		LIMIT 500`, lastLoginExpression, strings.Join(where, " AND "))
	rows, err := h.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]adminUser, 0)
	for rows.Next() {
		var user adminUser
		var created, login, activity sql.NullTime
		if err := rows.Scan(&user.Email, &user.Plan, &user.OutputsTotal, &user.OutputsRemaining, &user.Status, &created, &login, &activity, &user.OutputsUsedCount, &user.WatchlistSourceCount); err != nil {
			return nil, err
		}
		if created.Valid {
			user.CreatedAt = &created.Time
		}
		if login.Valid {
			user.LastLoginAt = &login.Time
		}
		if activity.Valid {
			user.LastActivityAt = &activity.Time
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

type adminUserActionInput struct {
	Email  string `json:"email"`
	Action string `json:"action"`
	Amount int64  `json:"amount"`
	PlanID string `json:"plan_id"`
	Status string `json:"status"`
}

func (h *Handler) AdminUserAction(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	var req adminUserActionInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	action := strings.TrimSpace(req.Action)
	if email == "" || action == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and action required"})
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db transaction failed"})
		return
	}
	defer tx.Rollback()
	switch action {
	case "add_outputs":
		if req.Amount <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "positive amount required"})
			return
		}
		if err := ensureActiveEntitlement(r.Context(), tx, email, strings.TrimSpace(req.PlanID)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "entitlement unavailable"})
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE entitlements SET outputs_total=outputs_total+$2, outputs_remaining=outputs_remaining+$2, updated_at=now() WHERE id=(SELECT id FROM entitlements WHERE lower(email)=$1 AND status='active' ORDER BY outputs_remaining DESC, created_at DESC LIMIT 1)`, email, req.Amount)
	case "reduce_outputs":
		if req.Amount <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "positive amount required"})
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE entitlements SET outputs_remaining=GREATEST(outputs_remaining-$2,0), updated_at=now() WHERE id=(SELECT id FROM entitlements WHERE lower(email)=$1 AND status='active' ORDER BY outputs_remaining DESC, created_at DESC LIMIT 1)`, email, req.Amount)
	case "set_plan":
		plan := strings.TrimSpace(req.PlanID)
		if plan == "" {
			plan = "free"
		}
		if err := ensureActiveEntitlement(r.Context(), tx, email, plan); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "entitlement unavailable"})
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE entitlements SET plan_id=$2, updated_at=now() WHERE id=(SELECT id FROM entitlements WHERE lower(email)=$1 AND status='active' ORDER BY created_at DESC LIMIT 1)`, email, plan)
	case "set_status":
		status := strings.ToLower(strings.TrimSpace(req.Status))
		if status != "active" && status != "inactive" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be active or inactive"})
			return
		}
		if status == "active" {
			if err := ensureActiveEntitlement(r.Context(), tx, email, strings.TrimSpace(req.PlanID)); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "entitlement unavailable"})
				return
			}
		} else {
			_, err = tx.ExecContext(r.Context(), `UPDATE entitlements SET status='inactive', updated_at=now() WHERE lower(email)=$1 AND status='active'`, email)
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported action"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "entitlement update failed"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db commit failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func ensureActiveEntitlement(ctx context.Context, tx *sql.Tx, email, plan string) error {
	if plan == "" {
		plan = "free"
	}
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id::text FROM entitlements WHERE lower(email)=$1 AND status='active' ORDER BY created_at DESC LIMIT 1`, email).Scan(&id)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO entitlements (email, plan_id, outputs_total, outputs_remaining, status) VALUES ($1, $2, 0, 0, 'active')`, email, plan)
	return err
}

type adminEntitlement struct {
	Email            string    `json:"email"`
	PlanID           string    `json:"plan_id"`
	OutputsTotal     int64     `json:"outputs_total"`
	OutputsRemaining int64     `json:"outputs_remaining"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	PaymentRequestID *string   `json:"payment_request_id,omitempty"`
}

func (h *Handler) AdminEntitlements(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensurePaymentSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment schema unavailable"})
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT COALESCE(email, ''), COALESCE(plan_id, 'free'), COALESCE(outputs_total, 0), COALESCE(outputs_remaining, 0), COALESCE(status, ''), created_at, updated_at, payment_request_id
		FROM entitlements ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()
	items := make([]adminEntitlement, 0)
	for rows.Next() {
		var item adminEntitlement
		if err := rows.Scan(&item.Email, &item.PlanID, &item.OutputsTotal, &item.OutputsRemaining, &item.Status, &item.CreatedAt, &item.UpdatedAt, &item.PaymentRequestID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "entitlements": items})
}

type adminOutput struct {
	Email        string    `json:"email"`
	OutputType   string    `json:"output_type"`
	Title        string    `json:"title"`
	UsedAI       bool      `json:"used_ai"`
	UsedFallback bool      `json:"used_fallback"`
	CreatedAt    time.Time `json:"created_at"`
}

func (h *Handler) AdminOutputs(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT COALESCE(email,''), COALESCE(output_type,''), COALESCE(title,''), COALESCE(used_ai,false), COALESCE(used_fallback,false), created_at FROM web3_outputs ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()
	items := make([]adminOutput, 0, 50)
	for rows.Next() {
		var item adminOutput
		if err := rows.Scan(&item.Email, &item.OutputType, &item.Title, &item.UsedAI, &item.UsedFallback, &item.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "outputs": items})
}

type adminWatchlistSource struct {
	Email               string    `json:"email"`
	Label               string    `json:"label"`
	Chain               string    `json:"chain"`
	Network             string    `json:"network"`
	Address             string    `json:"address"`
	SourceType          string    `json:"source_type"`
	Status              string    `json:"status"`
	ProviderSetupStatus string    `json:"provider_setup_status"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (h *Handler) AdminWatchlistSources(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT COALESCE(email,''), COALESCE(label,name,''), COALESCE(chain,''), COALESCE(network,''), COALESCE(address,''), COALESCE(source_type,''), COALESCE(status,''), COALESCE(provider_setup_status,''), created_at, updated_at FROM web3_event_sources ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()
	items := make([]adminWatchlistSource, 0)
	for rows.Next() {
		var item adminWatchlistSource
		if err := rows.Scan(&item.Email, &item.Label, &item.Chain, &item.Network, &item.Address, &item.SourceType, &item.Status, &item.ProviderSetupStatus, &item.CreatedAt, &item.UpdatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sources": items})
}

type adminWeb3Event struct {
	Email     string    `json:"email"`
	EventType string    `json:"event_type"`
	Chain     string    `json:"chain"`
	Network   string    `json:"network"`
	Address   string    `json:"address"`
	TxHash    string    `json:"tx_hash"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) AdminWeb3Events(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT COALESCE(email,''), COALESCE(event_type,''), COALESCE(chain,''), COALESCE(network,''), COALESCE(address,''), COALESCE(tx_hash,''), created_at FROM web3_events ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()
	items := make([]adminWeb3Event, 0, 50)
	for rows.Next() {
		var item adminWeb3Event
		if err := rows.Scan(&item.Email, &item.EventType, &item.Chain, &item.Network, &item.Address, &item.TxHash, &item.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "events": items})
}

func (h *Handler) AdminChainHealth(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	columns, err := chainHealthLogColumns(r.Context(), h.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	okCol := firstAvailableColumn(columns, "ok", "healthy")
	resultCol := firstAvailableColumn(columns, "result", "status")
	errorCol := firstAvailableColumn(columns, "error", "error_message")
	checkedCol := firstAvailableColumn(columns, "checked_at", "created_at")
	if okCol == "" || resultCol == "" || errorCol == "" || checkedCol == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unsupported chain health schema"})
		return
	}
	query := fmt.Sprintf(`SELECT COALESCE(chain,''), COALESCE(network,''), COALESCE(provider,''), %s, COALESCE(%s,''), COALESCE(%s,''), %s FROM chain_health_logs ORDER BY %s DESC LIMIT 50`, okCol, resultCol, errorCol, checkedCol, checkedCol)
	rows, err := h.DB.QueryContext(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 50)
	for rows.Next() {
		var chain, network, provider, result, errorText string
		var ok bool
		var checkedAt time.Time
		if err := rows.Scan(&chain, &network, &provider, &ok, &result, &errorText, &checkedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		items = append(items, map[string]any{"chain": chain, "network": network, "provider": provider, "ok": ok, "result": result, "error": errorText, "checked_at": checkedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "logs": items})
}

func (h *Handler) AdminSettings(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	present := func(name string) bool { return strings.TrimSpace(getenvForAdmin(name)) != "" }
	settings := []map[string]any{
		{"name": "APP_ENV", "present": present("APP_ENV")},
		{"name": "CORS_ALLOWED_ORIGIN", "present": present("CORS_ALLOWED_ORIGIN")},
		{"name": "X402_ENABLED", "present": present("X402_ENABLED"), "enabled": parseAdminBool(getenvForAdmin("X402_ENABLED"))},
		{"name": "PAY_PER_TOOL_ENABLED", "present": present("PAY_PER_TOOL_ENABLED"), "enabled": parseAdminBool(getenvForAdmin("PAY_PER_TOOL_ENABLED"))},
		{"name": "PUBLIC_DOMAIN", "present": present("PUBLIC_DOMAIN")},
		{"name": "VERSION", "present": present("VERSION")},
		{"name": "BUILD_LABEL", "present": present("BUILD_LABEL")},
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": settings})
}

func getenvForAdmin(name string) string { return strings.TrimSpace(os.Getenv(name)) }

func parseAdminBool(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = ANY(current_schemas(false)) AND table_name=$1`, table)
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
