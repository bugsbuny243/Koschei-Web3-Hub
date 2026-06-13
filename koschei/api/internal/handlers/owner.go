package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type ownerUserRecord struct {
	ID            string     `json:"id"`
	AuthSubject   string     `json:"auth_subject"`
	Email         string     `json:"email"`
	WalletAddress string     `json:"wallet_address,omitempty"`
	Credits       int        `json:"credits"`
	Status        string     `json:"status"`
	PlanID        string     `json:"plan_id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	BannedAt      *time.Time `json:"banned_at,omitempty"`
}

type ownerCreditInput struct {
	Email         string `json:"email"`
	AuthSubject   string `json:"auth_subject"`
	WalletAddress string `json:"wallet_address"`
	Credits       int    `json:"credits"`
	Reason        string `json:"reason"`
}

type ownerBanInput struct {
	Email       string `json:"email"`
	AuthSubject string `json:"auth_subject"`
	Ban         bool   `json:"ban"`
	Reason      string `json:"reason"`
}

type ownerCommandInput struct {
	Command string `json:"command"`
}

type ownerBrainInput struct {
	Message string `json:"message"`
}

type ownerLoginInput struct {
	Wallet string `json:"wallet"`
	Secret string `json:"secret"`
}

type ownerRemoveInput struct {
	Email         string `json:"email"`
	AuthSubject   string `json:"auth_subject"`
	WalletAddress string `json:"wallet_address"`
	Reason        string `json:"reason"`
}

func (h *Handler) OwnerLogin(w http.ResponseWriter, r *http.Request) {
	ownerWallet := normalizeWallet(firstEnv("OWNER_WALLET", "KOSCHEI_OWNER_WALLET"))
	ownerSecret := strings.TrimSpace(firstEnv("OWNER_SECRET", "KOSCHEI_OWNER_SECRET"))
	if ownerSecret == "" {
		http.NotFound(w, r)
		return
	}
	var req ownerLoginInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if !constantTimeStringEqual(strings.TrimSpace(req.Secret), ownerSecret) {
		http.NotFound(w, r)
		return
	}
	loginWallet := normalizeWallet(req.Wallet)
	if ownerWallet != "" && loginWallet != "" && loginWallet != ownerWallet {
		http.NotFound(w, r)
		return
	}
	secure := strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	http.SetCookie(w, &http.Cookie{Name: "koschei_owner_secret", Value: ownerSecret, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, Expires: time.Now().Add(12 * time.Hour)})
	http.SetCookie(w, &http.Cookie{Name: "koschei_owner_wallet", Value: firstNonEmpty(ownerWallet, loginWallet), Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, Expires: time.Now().Add(12 * time.Hour)})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Owner oturumu açıldı."})
}

func (h *Handler) OwnerUsers(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "owner schema unavailable"})
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	where := ""
	args := []any{}
	if q != "" {
		where = `WHERE lower(email) LIKE $1 OR lower(COALESCE(wallet_address,'')) LIKE $1 OR lower(COALESCE(auth_subject,'')) LIKE $1`
		args = append(args, "%"+q+"%")
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id::text, COALESCE(auth_subject,''), email, COALESCE(wallet_address,''), COALESCE(credits,0), COALESCE(status,'active'), COALESCE(plan_id,''), created_at, updated_at, banned_at FROM app_user_profiles `+where+` ORDER BY created_at DESC LIMIT 500`, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()
	users := []ownerUserRecord{}
	for rows.Next() {
		var u ownerUserRecord
		if err := rows.Scan(&u.ID, &u.AuthSubject, &u.Email, &u.WalletAddress, &u.Credits, &u.Status, &u.PlanID, &u.CreatedAt, &u.UpdatedAt, &u.BannedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "users": users})
}

func (h *Handler) OwnerAddCredits(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "owner schema unavailable"})
		return
	}
	var req ownerCreditInput
	if err := decodeJSON(r, &req); err != nil || req.Credits <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "positive credits required"})
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "owner_manual_credit"
	}
	where, args := ownerIdentityWhere(req.Email, req.AuthSubject, req.WalletAddress)
	if where == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, auth_subject, or wallet_address required"})
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db transaction failed"})
		return
	}
	defer tx.Rollback()
	args = append(args, req.Credits)
	res, err := tx.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits=COALESCE(credits,0)+$`+fmt.Sprint(len(args))+`, updated_at=now() `+where, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "credit update failed"})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	identityArgs := args[:len(args)-1]
	var eventEmail string
	if err := tx.QueryRowContext(r.Context(), `SELECT email FROM app_user_profiles `+where+` LIMIT 1`, identityArgs...).Scan(&eventEmail); err == nil {
		_, _ = tx.ExecContext(r.Context(), `INSERT INTO credit_events (email, amount, reason, event_type) VALUES (lower($1), $2, $3, 'owner_manual_credit')`, eventEmail, req.Credits, reason)
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db commit failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "credits_added": req.Credits})
}

func (h *Handler) OwnerBanUser(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "owner schema unavailable"})
		return
	}
	var req ownerBanInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	where, args := ownerIdentityWhere(req.Email, req.AuthSubject, "")
	if where == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email or auth_subject required"})
		return
	}
	status := "active"
	bannedAt := "NULL"
	if req.Ban {
		status = "banned"
		bannedAt = "now()"
	}
	res, err := h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET status=$`+fmt.Sprint(len(args)+1)+`, banned_at=`+bannedAt+`, ban_reason=$`+fmt.Sprint(len(args)+2)+`, updated_at=now() `+where, append(args, status, strings.TrimSpace(req.Reason))...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user update failed"})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}

func (h *Handler) OwnerRemoveUser(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "owner schema unavailable"})
		return
	}
	var req ownerRemoveInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	where, args := ownerIdentityWhere(req.Email, req.AuthSubject, req.WalletAddress)
	if where == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, auth_subject, or wallet_address required"})
		return
	}
	res, err := h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET status='removed', banned_at=now(), ban_reason=$`+fmt.Sprint(len(args)+1)+`, updated_at=now() `+where, append(args, firstNonEmpty(req.Reason, "owner_removed"))...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user remove failed"})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "removed"})
}

func (h *Handler) OwnerPaymentRequests(w http.ResponseWriter, r *http.Request) {
	h.OwnerPaymentRequestsList(w, r)
}
func (h *Handler) OwnerApprovePayment(w http.ResponseWriter, r *http.Request) {
	h.OwnerApprovePaymentRequest(w, r)
}
func (h *Handler) OwnerRejectPayment(w http.ResponseWriter, r *http.Request) {
	h.OwnerRejectPaymentRequest(w, r)
}

func (h *Handler) OwnerCommand(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "owner schema unavailable"})
		return
	}
	var req ownerCommandInput
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Command) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "command required"})
		return
	}
	command := strings.TrimSpace(req.Command)
	status := "queued"
	output := "Komut kaydedildi. GitHub/Together AI otomasyonu yapılandırıldığında bu kayıt iş kuyruğuna yönlendirilecek."
	if _, err := h.DB.ExecContext(r.Context(), `INSERT INTO ai_command_logs (command, output, status, created_at) VALUES ($1,$2,$3,now())`, command, output, status); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "command log failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "status": status, "output": output})
}

func (h *Handler) OwnerBrain(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "code": "OWNER_SCHEMA_UNAVAILABLE", "message": "Owner schema unavailable.", "data": nil})
		return
	}
	var req ownerBrainInput
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Message) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "code": "MESSAGE_REQUIRED", "message": "Komut mesajı gerekli.", "data": nil})
		return
	}
	message := strings.TrimSpace(req.Message)
	intent, result, humanMessage, ok := h.routeOwnerBrainCommand(r.Context(), message)
	status := "completed"
	code := "OK"
	if !ok {
		status = "unsupported"
		code = "COMMAND_UNSUPPORTED"
		humanMessage = "Bu komut henüz desteklenmiyor."
	}
	_, _ = h.DB.ExecContext(r.Context(), `INSERT INTO ai_command_logs (command, output, status, created_at) VALUES ($1,$2,$3,now())`, message, humanMessage, status)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "code": code, "message": humanMessage, "data": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "code": code, "message": humanMessage, "data": map[string]any{"intent": intent, "result": result}})
}

func (h *Handler) OwnerHealth(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	dbStatus := "connected"
	if err := h.DB.PingContext(r.Context()); err != nil {
		dbStatus = "error"
	}
	services := map[string]any{
		"database": map[string]string{"status": dbStatus},
		"openai":   map[string]string{"status": configuredStatus("OPENAI_API_KEY")},
		"paddle":   map[string]string{"status": configuredStatus("PADDLE_API_KEY", "PADDLE_WEBHOOK_SECRET", "PADDLE_ENV")},
		"alchemy":  map[string]string{"status": configuredStatusAny("ALCHEMY_API_KEY", "SOLANA_RPC_URL")},
		"github":   map[string]string{"status": configuredStatus("GITHUB_TOKEN", "GITHUB_OWNER", "GITHUB_REPO")},
		"neon":     map[string]string{"status": configuredStatus("DATABASE_URL")},
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "services": services})
}

func (h *Handler) routeOwnerBrainCommand(ctx context.Context, message string) (string, map[string]any, string, bool) {
	cmd := strings.ToLower(strings.TrimSpace(message))
	cmd = strings.Join(strings.Fields(cmd), " ")
	switch {
	case strings.Contains(cmd, "son 24 saat") && strings.Contains(cmd, "hata"):
		result := h.ownerRecentErrors(ctx)
		return "recent_errors_24h", result, "Son 24 saat hata özeti hazır.", true
	case strings.HasPrefix(cmd, "kullanıcı ara") || strings.HasPrefix(cmd, "kullanici ara"):
		email := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(cmd, "kullanıcı ara"), "kullanici ara"))
		result := h.ownerSearchUser(ctx, email)
		return "user_search", result, "Kullanıcı arama sonucu hazır.", true
	case strings.Contains(cmd, "bekleyen ödeme") || strings.Contains(cmd, "bekleyen odeme"):
		result := h.ownerPendingPayments(ctx)
		return "pending_payments", result, "Bekleyen ödemeler listelendi.", true
	case strings.Contains(cmd, "paddle") && strings.Contains(cmd, "durum"):
		return "paddle_status", envConfiguredResult([]string{"PADDLE_API_KEY", "PADDLE_WEBHOOK_SECRET", "PADDLE_ENV"}, false), "Paddle yapılandırma durumu hazır.", true
	case strings.Contains(cmd, "openai") && strings.Contains(cmd, "durum"):
		return "openai_status", envConfiguredResult([]string{"OPENAI_API_KEY"}, false), "OpenAI yapılandırma durumu hazır.", true
	case strings.Contains(cmd, "alchemy") && strings.Contains(cmd, "durum"):
		return "alchemy_status", envConfiguredResult([]string{"ALCHEMY_API_KEY", "SOLANA_RPC_URL"}, true), "Alchemy / Solana RPC yapılandırma durumu hazır.", true
	case strings.Contains(cmd, "github") && strings.Contains(cmd, "durum"):
		return "github_status", envConfiguredResult([]string{"GITHUB_TOKEN", "GITHUB_OWNER", "GITHUB_REPO"}, false), "GitHub yapılandırma durumu hazır.", true
	case strings.Contains(cmd, "neon") && strings.Contains(cmd, "durum"):
		return "neon_status", envConfiguredResult([]string{"DATABASE_URL"}, false), "Neon veritabanı yapılandırma durumu hazır.", true
	default:
		return "unsupported", nil, "", false
	}
}

func (h *Handler) ownerRecentErrors(ctx context.Context) map[string]any {
	result := map[string]any{}
	if ownerTableExists(ctx, h.DB, "runtime_logs") {
		result["runtime_logs"] = ownerQueryRows(ctx, h.DB, `SELECT created_at, level, message FROM runtime_logs WHERE created_at >= now() - interval '24 hours' AND lower(level) IN ('error','fatal','warn','warning') ORDER BY created_at DESC LIMIT 50`, []string{"created_at", "level", "message"})
	}
	if ownerTableExists(ctx, h.DB, "model_route_logs") {
		result["api_logs"] = ownerQueryRows(ctx, h.DB, `SELECT created_at, provider, model, status, tool FROM model_route_logs WHERE created_at >= now() - interval '24 hours' AND lower(COALESCE(status,'')) NOT IN ('ok','success','completed') ORDER BY created_at DESC LIMIT 50`, []string{"created_at", "provider", "model", "status", "tool"})
	}
	if ownerTableExists(ctx, h.DB, "generation_jobs") {
		result["failed_jobs"] = ownerQueryRows(ctx, h.DB, `SELECT created_at, updated_at, email, tool, provider, status FROM generation_jobs WHERE updated_at >= now() - interval '24 hours' AND lower(status) IN ('failed','error') ORDER BY updated_at DESC LIMIT 50`, []string{"created_at", "updated_at", "email", "tool", "provider", "status"})
	}
	if ownerTableExists(ctx, h.DB, "web3_jobs") {
		result["failed_web3_jobs"] = ownerQueryRows(ctx, h.DB, `SELECT queued_at, updated_at, email, job_type, status, error_code, error_message FROM web3_jobs WHERE updated_at >= now() - interval '24 hours' AND lower(status) IN ('failed','error') ORDER BY updated_at DESC LIMIT 50`, []string{"queued_at", "updated_at", "email", "job_type", "status", "error_code", "error_message"})
	}
	if ownerTableExists(ctx, h.DB, "payment_requests") {
		result["failed_payments"] = ownerQueryRows(ctx, h.DB, `SELECT created_at, reviewed_at, email, product_id, amount_try, currency, status FROM payment_requests WHERE created_at >= now() - interval '24 hours' AND lower(status) IN ('failed','rejected','error') ORDER BY created_at DESC LIMIT 50`, []string{"created_at", "reviewed_at", "email", "product_id", "amount_try", "currency", "status"})
	}
	return result
}

func (h *Handler) ownerSearchUser(ctx context.Context, email string) map[string]any {
	result := map[string]any{"email": email, "user": nil, "entitlement": map[string]any{"active": false}}
	if email == "" || !ownerTableExists(ctx, h.DB, "app_user_profiles") {
		return result
	}
	rows := ownerQueryRows(ctx, h.DB, `SELECT email, COALESCE(auth_subject,''), COALESCE(status,'active'), COALESCE(plan_id,''), COALESCE(credits,0), created_at, updated_at FROM app_user_profiles WHERE lower(email)=lower($1) LIMIT 1`, []string{"email", "auth_subject", "status", "plan_id", "credits_legacy", "created_at", "updated_at"}, email)
	if len(rows) > 0 {
		result["user"] = rows[0]
	}
	if ownerTableExists(ctx, h.DB, "entitlements") {
		rows = ownerQueryRows(ctx, h.DB, `SELECT plan_id, status, created_at AS starts_at, NULL::timestamptz AS expires_at FROM entitlements WHERE lower(email)=lower($1) AND status='active' ORDER BY updated_at DESC, created_at DESC LIMIT 1`, []string{"plan_id", "status", "starts_at", "expires_at"}, email)
		if len(rows) > 0 {
			rows[0]["active"] = true
			result["entitlement"] = rows[0]
		}
	}
	return result
}

func (h *Handler) ownerPendingPayments(ctx context.Context) map[string]any {
	if !ownerTableExists(ctx, h.DB, "payment_requests") {
		return map[string]any{"payment_requests": []map[string]any{}, "table_exists": false}
	}
	rows := ownerQueryRows(ctx, h.DB, `SELECT id::text, email, COALESCE(full_name,''), COALESCE(product_id,''), COALESCE(amount_try,0), COALESCE(currency,'TRY'), status, created_at FROM payment_requests WHERE status='pending' ORDER BY created_at DESC LIMIT 100`, []string{"id", "email", "full_name", "product_id", "amount_try", "currency", "status", "created_at"})
	return map[string]any{"payment_requests": rows, "count": len(rows), "table_exists": true}
}

func configuredStatus(keys ...string) string {
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return "missing"
		}
	}
	return "configured"
}

func configuredStatusAny(keys ...string) string {
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return "configured"
		}
	}
	return "missing"
}

func envConfiguredResult(keys []string, anyMode bool) map[string]any {
	items := map[string]bool{}
	configuredCount := 0
	for _, key := range keys {
		ok := strings.TrimSpace(os.Getenv(key)) != ""
		items[key] = ok
		if ok {
			configuredCount++
		}
	}
	status := "missing"
	if (!anyMode && configuredCount == len(keys)) || (anyMode && configuredCount > 0) {
		status = "configured"
	}
	return map[string]any{"status": status, "configured": items}
}

func ownerTableExists(ctx context.Context, db *sql.DB, table string) bool {
	var exists bool
	_ = db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = ANY (current_schemas(false)) AND table_name = $1)`, table).Scan(&exists)
	return exists
}

func ownerQueryRows(ctx context.Context, db *sql.DB, query string, keys []string, args ...any) []map[string]any {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	items := []map[string]any{}
	vals := make([]any, len(keys))
	ptrs := make([]any, len(keys))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		item := map[string]any{}
		for i, key := range keys {
			item[key] = ownerJSONValue(vals[i])
		}
		items = append(items, item)
	}
	return items
}

func ownerJSONValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case time.Time:
		return t
	default:
		return t
	}
}

func (h *Handler) OwnerStatus(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "owner schema unavailable"})
		return
	}
	metrics := map[string]any{}
	count := func(key, query string) {
		var v int64
		if err := h.DB.QueryRowContext(r.Context(), query).Scan(&v); err == nil {
			metrics[key] = v
		}
	}
	money := func(key, query string) {
		var v float64
		if err := h.DB.QueryRowContext(r.Context(), query).Scan(&v); err == nil {
			metrics[key] = v
		}
	}
	count("total_users", `SELECT count(*) FROM app_user_profiles`)
	count("active_users_today", `SELECT count(DISTINCT COALESCE(NULLIF(lower(email),''), path)) FROM analytics_events WHERE created_at >= CURRENT_DATE`)
	count("pending_payments", `SELECT count(*) FROM payment_requests WHERE status='pending'`)
	count("approved_payments", `SELECT count(*) FROM payment_requests WHERE status='approved'`)
	money("daily_revenue_try", `SELECT COALESCE(sum(amount_try),0)::float FROM payment_requests WHERE status='approved' AND reviewed_at >= CURRENT_DATE`)
	money("monthly_revenue_try", `SELECT COALESCE(sum(amount_try),0)::float FROM payment_requests WHERE status='approved' AND reviewed_at >= date_trunc('month', now())`)
	money("total_saved_usd", `SELECT COALESCE((SELECT sum(mev_saved_usd) FROM mev_protection_events),0)::float + COALESCE((SELECT sum(loss_prevented_usd) FROM liquidity_drain_alerts),0)::float + COALESCE((SELECT sum(estimated_outflow_usd) FROM proposal_risks WHERE risk_score >= 40),0)::float`)
	count("pending_prs", `SELECT count(*) FROM ai_command_logs WHERE status IN ('queued','running')`)
	var best string
	_ = h.DB.QueryRowContext(r.Context(), `SELECT COALESCE(product_id,'') FROM payment_requests WHERE status='approved' GROUP BY product_id ORDER BY count(*) DESC LIMIT 1`).Scan(&best)
	metrics["best_selling_package"] = best
	logs := []map[string]any{}
	if rows, err := h.DB.QueryContext(r.Context(), `SELECT command, status, output, created_at FROM ai_command_logs ORDER BY created_at DESC LIMIT 100`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var c, s, o string
			var t time.Time
			_ = rows.Scan(&c, &s, &o, &t)
			logs = append(logs, map[string]any{"event": "ai_command", "message": c, "status": s, "output": o, "created_at": t})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "metrics": metrics, "logs": logs})
}

func (h *Handler) OwnerGrants(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT title, deadline, status, COALESCE(notes, '')
		FROM grant_opportunities
		WHERE status <> 'archived'
		ORDER BY fit_score DESC, updated_at DESC
		LIMIT 100`)
	if isMissingRelation(err) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "grants": []map[string]any{}})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "grants unavailable"})
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var program, deadline, status, focus string
		if err := rows.Scan(&program, &deadline, &status, &focus); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "grants scan failed"})
			return
		}
		items = append(items, map[string]any{"program": program, "deadline": deadline, "status": status, "focus": focus})
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "grants unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "grants": items})
}

func (h *Handler) ShopierWebhook(w http.ResponseWriter, r *http.Request) {
	secret := strings.TrimSpace(firstEnv("SHOPIER_WEBHOOK_SECRET", "OWNER_SECRET"))
	if secret != "" && !constantTimeStringEqual(strings.TrimSpace(r.Header.Get("x-shopier-secret")), secret) {
		http.NotFound(w, r)
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment schema unavailable"})
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	email := lowerStringFromAny(payload["email"])
	productID := strings.ToLower(strings.TrimSpace(fmt.Sprint(payload["product_id"])))
	if email == "" || productID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and product_id required"})
		return
	}
	pack, ok := shopierPacks[productID]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown product_id"})
		return
	}
	externalPaymentID := ""
	for _, key := range []string{"payment_id", "order_id", "id"} {
		if value, ok := payload[key]; ok {
			if candidate := strings.TrimSpace(fmt.Sprint(value)); candidate != "" && candidate != "<nil>" {
				externalPaymentID = candidate
				break
			}
		}
	}
	if externalPaymentID == "" {
		externalPaymentID = "shopier:" + email + ":" + productID + ":" + fmt.Sprint(payload["created_at"])
	}
	raw, _ := json.Marshal(payload)
	_, err := h.DB.ExecContext(r.Context(), `INSERT INTO payment_requests (email, full_name, product_id, amount_try, currency, status, raw_payload, payment_provider, external_payment_id, reviewed_at, created_at) VALUES ($1,$2,$3,$4,'TRY','approved',$5::jsonb,'shopier',$6,now(),now())`, email, strings.TrimSpace(fmt.Sprint(payload["full_name"])), productID, pack.AmountTRY, string(raw), externalPaymentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "webhook insert failed"})
		return
	}
	result, err := h.activatePackageEntitlement(r.Context(), email, productID, "shopier", externalPaymentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "entitlement activation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "approved", "activated": result.Activated})
}

func ensureOwnerSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("db nil")
	}
	stmts := []string{
		`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS wallet_address text`,
		`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active'`,
		`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS banned_at timestamptz`,
		`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS ban_reason text`,
		`CREATE TABLE IF NOT EXISTS credit_events (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text, amount integer NOT NULL, reason text, event_type text, created_at timestamptz NOT NULL DEFAULT now())`,
		`CREATE TABLE IF NOT EXISTS ai_command_logs (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), command text NOT NULL, output text NOT NULL DEFAULT '', status text NOT NULL DEFAULT 'queued', created_at timestamptz NOT NULL DEFAULT now())`,
		`CREATE TABLE IF NOT EXISTS system_analytics (day date PRIMARY KEY DEFAULT CURRENT_DATE, active_users integer NOT NULL DEFAULT 0, revenue_try numeric NOT NULL DEFAULT 0, credits_consumed integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now())`,
		`CREATE TABLE IF NOT EXISTS mev_protection_events (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_wallet text NOT NULL DEFAULT '', tx_signature text NOT NULL DEFAULT '', estimated_loss_usd numeric NOT NULL DEFAULT 0, mev_saved_usd numeric NOT NULL DEFAULT 0, jito_tip_used boolean NOT NULL DEFAULT false, risk_score integer NOT NULL DEFAULT 0, risk_level text NOT NULL DEFAULT 'DÜŞÜK', route text NOT NULL DEFAULT '', raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now())`,
		`ALTER TABLE mev_protection_events ADD COLUMN IF NOT EXISTS mev_saved_usd numeric NOT NULL DEFAULT 0`,
		`ALTER TABLE mev_protection_events ADD COLUMN IF NOT EXISTS raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb`,
		`CREATE TABLE IF NOT EXISTS liquidity_drain_alerts (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), pool_address text NOT NULL DEFAULT '', token_mint text NOT NULL DEFAULT '', severity text NOT NULL DEFAULT 'DÜŞÜK', risk_score integer NOT NULL DEFAULT 0, removed_liquidity_usd numeric NOT NULL DEFAULT 0, loss_prevented_usd numeric NOT NULL DEFAULT 0, telegram_queued boolean NOT NULL DEFAULT false, sms_queued boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now())`,
		`CREATE TABLE IF NOT EXISTS proposal_risks (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), dao_id text NOT NULL DEFAULT '', treasury_address text NOT NULL DEFAULT '', proposal_id text NOT NULL DEFAULT '', risk_score integer NOT NULL DEFAULT 0, risk_level text NOT NULL DEFAULT 'DÜŞÜK', estimated_outflow_usd numeric NOT NULL DEFAULT 0, instruction_count integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now())`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return ensurePaymentSchema(ctx, db)
}

func ownerIdentityWhere(email, sub, wallet string) (string, []any) {
	if strings.TrimSpace(email) != "" {
		return "WHERE lower(email)=lower($1)", []any{strings.TrimSpace(email)}
	}
	if strings.TrimSpace(sub) != "" {
		return "WHERE auth_subject=$1", []any{strings.TrimSpace(sub)}
	}
	if strings.TrimSpace(wallet) != "" {
		return "WHERE lower(wallet_address)=lower($1)", []any{strings.TrimSpace(wallet)}
	}
	return "", nil
}

func lowerStringFromAny(v any) string { return strings.ToLower(strings.TrimSpace(fmt.Sprint(v))) }
