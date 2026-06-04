package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

type adminSummary struct {
	Users                  int64 `json:"users"`
	Entitlements           int64 `json:"entitlements"`
	ActiveOutputsRemaining int64 `json:"active_outputs_remaining"`
	Web3Outputs            int64 `json:"web3_outputs"`
	ChainHealthLogs        int64 `json:"chain_health_logs"`
	Web3EventSources       int64 `json:"web3_event_sources"`
	Web3Events             int64 `json:"web3_events"`
	PendingPaymentRequests int64 `json:"pending_payment_requests"`
	AnalyticsEvents        int64 `json:"analytics_events"`
}

func (h *Handler) AdminSummary(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	var summary adminSummary
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT
			(SELECT count(*) FROM app_user_profiles),
			(SELECT count(*) FROM entitlements),
			(SELECT COALESCE(sum(outputs_remaining), 0) FROM entitlements WHERE status = 'active'),
			(SELECT count(*) FROM web3_outputs),
			(SELECT count(*) FROM chain_health_logs),
			(SELECT count(*) FROM web3_event_sources),
			(SELECT count(*) FROM web3_events),
			(SELECT count(*) FROM payment_requests WHERE status = 'pending'),
			(SELECT count(*) FROM analytics_events)`).Scan(
		&summary.Users, &summary.Entitlements, &summary.ActiveOutputsRemaining,
		&summary.Web3Outputs, &summary.ChainHealthLogs, &summary.Web3EventSources,
		&summary.Web3Events, &summary.PendingPaymentRequests, &summary.AnalyticsEvents,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary})
}

type adminUser struct {
	Email            string     `json:"email"`
	CreatedAt        time.Time  `json:"created_at"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty"`
	PlanSummary      string     `json:"plan_summary"`
	OutputsRemaining int64      `json:"outputs_remaining"`
}

func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	columns, err := tableColumns(r.Context(), h.DB, "app_user_profiles")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	lastLoginExpression := "NULL::timestamptz"
	for _, candidate := range []string{"last_login_at", "last_signed_in_at", "last_sign_in_at"} {
		if columns[candidate] {
			lastLoginExpression = "u." + candidate
			break
		}
	}
	query := fmt.Sprintf(`
		SELECT COALESCE(u.email, ''), u.created_at, %s,
		       COALESCE((SELECT string_agg(DISTINCT COALESCE(e.plan_id, 'free'), ', ' ORDER BY COALESCE(e.plan_id, 'free')) FROM entitlements e WHERE lower(e.email)=lower(u.email) AND e.status='active'), 'none'),
		       COALESCE((SELECT sum(e.outputs_remaining) FROM entitlements e WHERE lower(e.email)=lower(u.email) AND e.status='active'), 0)
		FROM app_user_profiles u
		ORDER BY u.created_at DESC
		LIMIT 500`, lastLoginExpression)
	rows, err := h.DB.QueryContext(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()
	users := make([]adminUser, 0)
	for rows.Next() {
		var user adminUser
		var lastLogin sql.NullTime
		if err := rows.Scan(&user.Email, &user.CreatedAt, &lastLogin, &user.PlanSummary, &user.OutputsRemaining); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		if lastLogin.Valid {
			user.LastLoginAt = &lastLogin.Time
		}
		users = append(users, user)
	}
	if rows.Err() != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "users": users})
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
