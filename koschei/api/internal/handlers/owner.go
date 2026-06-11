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
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id::text, COALESCE(auth_subject,''), email, COALESCE(wallet_address,''), COALESCE(credits,0), COALESCE(status,'active'), created_at, updated_at, banned_at FROM app_user_profiles `+where+` ORDER BY created_at DESC LIMIT 500`, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()
	users := []ownerUserRecord{}
	for rows.Next() {
		var u ownerUserRecord
		if err := rows.Scan(&u.ID, &u.AuthSubject, &u.Email, &u.WalletAddress, &u.Credits, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.BannedAt); err != nil {
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
	h.AdminPaymentRequests(w, r)
}
func (h *Handler) OwnerApprovePayment(w http.ResponseWriter, r *http.Request) {
	h.ApprovePaymentRequest(w, r)
}
func (h *Handler) OwnerRejectPayment(w http.ResponseWriter, r *http.Request) {
	h.RejectPaymentRequest(w, r)
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
	items := []map[string]any{
		{"program": "Solana Foundation", "deadline": "2026-07-15", "status": "Hazırlanıyor", "focus": "MEV Shield ve Liquidity Drain public-good metrikleri"},
		{"program": "Ethereum Grants", "deadline": "2026-08-01", "status": "Taslak", "focus": "Bridge/PoR Monitor ve DAO Guardian risk raporları"},
		{"program": "Protocol Labs / FIL RetroPGF", "deadline": "2026-09-10", "status": "Araştırma", "focus": "AI Exploit Simulator geliştirici güvenliği"},
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
	raw, _ := json.Marshal(payload)
	_, err := h.DB.ExecContext(r.Context(), `INSERT INTO payment_requests (email, full_name, product_id, amount_try, currency, status, raw_payload, created_at) VALUES ($1,$2,$3,$4,'TRY','pending',$5::jsonb,now())`, email, strings.TrimSpace(fmt.Sprint(payload["full_name"])), productID, pack.AmountTRY, string(raw))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "webhook insert failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "status": "pending"})
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
