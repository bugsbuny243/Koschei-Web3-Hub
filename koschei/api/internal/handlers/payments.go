package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const paymentRequestMessage = "KOSCH payment proof received. Owner review required."
const koschPaymentProvider = "kosch_token"
const koschDefaultTokenMint = "HHPpU9u56Bwxov12nf7DXUCuv6h1q5j1xgGS3yukpump"

type koscheiPackage struct {
	ID          string
	Name        string
	AmountKOSCH string
	Outputs     int
}

var koscheiPackages = map[string]koscheiPackage{
	"starter":      {ID: "starter", Name: "Koschei Starter", AmountKOSCH: "250000", Outputs: 25},
	"professional": {ID: "professional", Name: "Koschei Professional", AmountKOSCH: "750000", Outputs: 100},
	"enterprise":   {ID: "enterprise", Name: "Koschei Enterprise", AmountKOSCH: "2000000", Outputs: 300},
}

var koschPacks = map[string]koscheiPackage{
	"starter":      koscheiPackages["starter"],
	"professional": koscheiPackages["professional"],
	"enterprise":   koscheiPackages["enterprise"],
	"builder":      koscheiPackages["professional"],
	"studio":       koscheiPackages["enterprise"],
}

type paymentRequestInput struct {
	FullName             string `json:"full_name"`
	ProductID            string `json:"product_id"`
	PaymentReference     string `json:"payment_reference"`
	RegisteredEmail      string `json:"registered_email"`
	CustomerEmail        string `json:"customer_email"`
	Note                 string `json:"note"`
	WalletAddress        string `json:"wallet_address"`
	PayerWallet          string `json:"payer_wallet"`
	TransactionSignature string `json:"transaction_signature"`
	TxSignature          string `json:"tx_signature"`
}

type paymentRequestReviewInput struct {
	PaymentRequestID string `json:"payment_request_id"`
	ID               string `json:"id"`
	Reason           string `json:"reason"`
}

func ensurePaymentSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("db nil")
	}
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE IF NOT EXISTS payment_requests (id uuid PRIMARY KEY DEFAULT gen_random_uuid())`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS email text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS full_name text NOT NULL DEFAULT ''`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS product_id text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS product_slug text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS plan text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS amount_try integer`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS amount_kosch text NOT NULL DEFAULT ''`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'KOSCH'`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending'`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now()`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS reviewed_at timestamptz`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS payment_provider text NOT NULL DEFAULT 'kosch_token'`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS external_payment_id text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS transaction_signature text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS payer_wallet text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS treasury_wallet text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS token_mint text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS chain text NOT NULL DEFAULT 'solana'`,
		`ALTER TABLE payment_requests ALTER COLUMN payment_provider SET DEFAULT 'kosch_token'`,
		`ALTER TABLE payment_requests ALTER COLUMN currency SET DEFAULT 'KOSCH'`,
		`UPDATE payment_requests SET plan = COALESCE(NULLIF(plan, ''), NULLIF(product_slug, ''), raw_payload->>'product_id', 'starter') WHERE plan IS NULL OR plan = ''`,
		`ALTER TABLE payment_requests ALTER COLUMN plan SET DEFAULT 'starter'`,
		`ALTER TABLE payment_requests ALTER COLUMN plan SET NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS payment_requests_kosch_tx_sig_idx ON payment_requests (lower(transaction_signature)) WHERE transaction_signature IS NOT NULL AND transaction_signature <> ''`,
		`CREATE TABLE IF NOT EXISTS products (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), slug text NOT NULL, name text NOT NULL, pack_type text NOT NULL, description text, price_try_cents integer NOT NULL DEFAULT 0, price_usd_cents integer NOT NULL DEFAULT 0, price_kosch text NOT NULL DEFAULT '', output_quota integer NOT NULL DEFAULT 0, shopier_url text NOT NULL DEFAULT '', image_url text, is_active boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now())`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS slug text`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS name text`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS pack_type text`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS description text`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS price_try_cents integer NOT NULL DEFAULT 0`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS price_usd_cents integer NOT NULL DEFAULT 0`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS price_kosch text NOT NULL DEFAULT ''`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS output_quota integer NOT NULL DEFAULT 0`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS shopier_url text NOT NULL DEFAULT ''`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS image_url text`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS is_active boolean NOT NULL DEFAULT true`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now()`,
		`CREATE TABLE IF NOT EXISTS customers (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text, full_name text, company_name text, country text, source text, notes text, created_at timestamptz NOT NULL DEFAULT now())`,
		`CREATE TABLE IF NOT EXISTS orders (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), customer_id uuid, product_id uuid, provider text NOT NULL DEFAULT 'kosch_token', provider_order_id text, provider_payment_id text, amount_try_cents integer NOT NULL DEFAULT 0, currency text NOT NULL DEFAULT 'KOSCH', status text NOT NULL DEFAULT 'pending', raw_payload jsonb, purchased_at timestamptz, created_at timestamptz NOT NULL DEFAULT now())`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS customer_id uuid`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS product_id uuid`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT 'kosch_token'`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS provider_order_id text`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS provider_payment_id text`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS amount_try_cents integer NOT NULL DEFAULT 0`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'KOSCH'`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending'`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS raw_payload jsonb`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS purchased_at timestamptz`,
		`CREATE TABLE IF NOT EXISTS entitlements (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text, plan_id text, outputs_total integer NOT NULL DEFAULT 0, outputs_remaining integer NOT NULL DEFAULT 0, status text NOT NULL DEFAULT 'active', starts_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz, notes text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz)`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS email text`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS plan_id text`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS payment_request_id text`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS payment_provider text`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS external_payment_id text`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS starts_at timestamptz`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS expires_at timestamptz`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS order_id uuid`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS updated_at timestamptz`,
		`CREATE INDEX IF NOT EXISTS orders_provider_order_idx ON orders (provider, provider_order_id)`,
		`CREATE INDEX IF NOT EXISTS orders_provider_payment_idx ON orders (provider, provider_payment_id)`,
		`CREATE INDEX IF NOT EXISTS entitlements_email_status_expires_idx ON entitlements (lower(email), status, expires_at)`,
		`UPDATE products SET name='Starter', pack_type='starter', description='Koschei Starter package - paid with KOSCH', price_try_cents=0, price_usd_cents=0, price_kosch='250000', output_quota=25, shopier_url='', is_active=true, updated_at=now() WHERE slug='starter'`,
		`UPDATE products SET name='Professional', pack_type='professional', description='Koschei Professional package - paid with KOSCH', price_try_cents=0, price_usd_cents=0, price_kosch='750000', output_quota=100, shopier_url='', is_active=true, updated_at=now() WHERE slug='professional'`,
		`UPDATE products SET name='Enterprise', pack_type='enterprise', description='Koschei Enterprise package - paid with KOSCH', price_try_cents=0, price_usd_cents=0, price_kosch='2000000', output_quota=300, shopier_url='', is_active=true, updated_at=now() WHERE slug='enterprise'`,
		`INSERT INTO products (slug, name, pack_type, description, price_try_cents, price_usd_cents, price_kosch, output_quota, shopier_url, is_active) SELECT 'starter','Starter','starter','Koschei Starter package - paid with KOSCH',0,0,'250000',25,'',true WHERE NOT EXISTS (SELECT 1 FROM products WHERE slug='starter')`,
		`INSERT INTO products (slug, name, pack_type, description, price_try_cents, price_usd_cents, price_kosch, output_quota, shopier_url, is_active) SELECT 'professional','Professional','professional','Koschei Professional package - paid with KOSCH',0,0,'750000',100,'',true WHERE NOT EXISTS (SELECT 1 FROM products WHERE slug='professional')`,
		`INSERT INTO products (slug, name, pack_type, description, price_try_cents, price_usd_cents, price_kosch, output_quota, shopier_url, is_active) SELECT 'enterprise','Enterprise','enterprise','Koschei Enterprise package - paid with KOSCH',0,0,'2000000',300,'',true WHERE NOT EXISTS (SELECT 1 FROM products WHERE slug='enterprise')`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

type paymentRequestRecord struct {
	ID              string         `json:"id"`
	Email           string         `json:"email"`
	FullName        string         `json:"full_name"`
	ProductID       string         `json:"product_id"`
	AmountTRY       int            `json:"amount_try"`
	AmountKOSCH     string         `json:"amount_kosch"`
	Currency        string         `json:"currency"`
	Status          string         `json:"status"`
	PaymentProvider string         `json:"payment_provider"`
	RawPayload      map[string]any `json:"raw_payload"`
	CreatedAt       time.Time      `json:"created_at"`
	ReviewedAt      *time.Time     `json:"reviewed_at,omitempty"`
}

func (h *Handler) PaymentRequest(w http.ResponseWriter, r *http.Request) {
	if err := ensurePaymentSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment schema unavailable", "message": err.Error()})
		return
	}
	if h.Limiter != nil && !h.Limiter.allow("billing:"+clientIP(r), 10, 10_000_000_000) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limited"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req paymentRequestInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	fullName := strings.TrimSpace(req.FullName)
	productID := normalizePackageID(req.ProductID)
	pack, ok := koschPacks[productID]
	signature := sanitizePaymentSignature(firstNonEmpty(req.TransactionSignature, req.TxSignature, req.PaymentReference))
	payerWallet := normalizeWallet(firstNonEmpty(req.PayerWallet, req.WalletAddress))
	treasuryWallet := normalizeWallet(firstEnv("KOSCH_TREASURY_WALLET", "KOSCHEI_TREASURY_WALLET", "OWNER_WALLET", "KOSCHEI_OWNER_WALLET"))
	tokenMint := strings.TrimSpace(firstEnv("KOSCH_TOKEN_MINT", "KOSCHEI_TOKEN_MINT"))
	if tokenMint == "" {
		tokenMint = koschDefaultTokenMint
	}
	if email == "" || fullName == "" || !ok || signature == "" || payerWallet == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body", "message": "full_name, product_id, payer_wallet and transaction_signature are required"})
		return
	}
	if treasuryWallet == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosch treasury wallet not configured"})
		return
	}
	rawPayload, err := json.Marshal(map[string]string{
		"payment_provider":      koschPaymentProvider,
		"registered_email":      email,
		"declared_email":        firstNonEmpty(strings.ToLower(strings.TrimSpace(req.RegisteredEmail)), strings.ToLower(strings.TrimSpace(req.CustomerEmail)), email),
		"product_id":            productID,
		"amount_kosch":          pack.AmountKOSCH,
		"note":                  strings.TrimSpace(req.Note),
		"transaction_signature": signature,
		"payer_wallet":          payerWallet,
		"treasury_wallet":       treasuryWallet,
		"token_mint":            tokenMint,
		"chain":                 "solana",
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payload encoding failed"})
		return
	}
	if _, err := h.DB.ExecContext(r.Context(), `
		INSERT INTO payment_requests (email, full_name, product_slug, plan, amount_try, amount_kosch, currency, status, raw_payload, payment_provider, external_payment_id, transaction_signature, payer_wallet, treasury_wallet, token_mint, chain, created_at)
		VALUES ($1, $2, $3, $3, 0, $4, 'KOSCH', 'pending', $5::jsonb, $6, $7, $7, $8, $9, $10, 'solana', now())`, email, fullName, productID, pack.AmountKOSCH, string(rawPayload), koschPaymentProvider, signature, payerWallet, treasuryWallet, tokenMint); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "transaction signature already submitted"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db insert failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "status": "pending", "provider": koschPaymentProvider, "currency": "KOSCH", "amount_kosch": pack.AmountKOSCH, "message": paymentRequestMessage})
}

func (h *Handler) OwnerPaymentRequestsList(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensurePaymentSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "payment_requests": []paymentRequestRecord{}, "warning": "payment schema unavailable"})
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id::text, COALESCE(email,''), COALESCE(full_name, ''), COALESCE(NULLIF(product_slug,''), raw_payload->>'product_id', product_id::text, plan, ''), COALESCE(amount_try, 0), COALESCE(amount_kosch, raw_payload->>'amount_kosch', ''), COALESCE(currency, 'KOSCH'), status, COALESCE(payment_provider, raw_payload->>'payment_provider', 'kosch_token'), COALESCE(raw_payload, '{}'::jsonb), created_at, reviewed_at
		FROM payment_requests
		ORDER BY created_at DESC
		LIMIT 200`)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "payment_requests": []paymentRequestRecord{}, "warning": "payment requests unavailable", "message": err.Error()})
		return
	}
	defer rows.Close()

	requests := make([]paymentRequestRecord, 0)
	for rows.Next() {
		var request paymentRequestRecord
		var rawPayload []byte
		if err := rows.Scan(&request.ID, &request.Email, &request.FullName, &request.ProductID, &request.AmountTRY, &request.AmountKOSCH, &request.Currency, &request.Status, &request.PaymentProvider, &rawPayload, &request.CreatedAt, &request.ReviewedAt); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "payment_requests": requests, "warning": "payment request scan failed"})
			return
		}
		if err := json.Unmarshal(rawPayload, &request.RawPayload); err != nil {
			request.RawPayload = map[string]any{}
		}
		requests = append(requests, request)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "payment_requests": requests})
}

func (h *Handler) OwnerApprovePaymentRequest(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensurePaymentSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment schema unavailable"})
		return
	}
	var req paymentRequestReviewInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	paymentRequestID := strings.TrimSpace(firstNonEmpty(req.PaymentRequestID, req.ID))
	if paymentRequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db transaction failed"})
		return
	}
	defer tx.Rollback()

	var email, productID, status, provider, externalPaymentID string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT lower(email), COALESCE(NULLIF(product_slug,''), raw_payload->>'product_id', product_id::text, plan, ''), status, COALESCE(payment_provider, raw_payload->>'payment_provider', 'kosch_token'), COALESCE(transaction_signature, raw_payload->>'transaction_signature', external_payment_id, '')
		FROM payment_requests
		WHERE id = $1
		FOR UPDATE`, paymentRequestID).Scan(&email, &productID, &status, &provider, &externalPaymentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "payment request not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed", "message": err.Error()})
		return
	}
	productID = normalizePackageID(productID)
	if _, ok := koschPacks[productID]; !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown product_id"})
		return
	}
	if status != "pending" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "payment request already reviewed"})
		return
	}

	if _, err := activatePackageEntitlementTx(r.Context(), tx, email, productID, normalizePaymentProvider(provider), externalPaymentID, paymentRequestID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "entitlement activation failed"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE payment_requests SET status = 'approved', reviewed_at = now() WHERE id = $1`, paymentRequestID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment request update failed"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db commit failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "approved", "provider": normalizePaymentProvider(provider)})
}

func (h *Handler) OwnerRejectPaymentRequest(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensurePaymentSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment schema unavailable"})
		return
	}
	var req paymentRequestReviewInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	paymentRequestID := strings.TrimSpace(firstNonEmpty(req.PaymentRequestID, req.ID))
	if paymentRequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	reason := strings.TrimSpace(req.Reason)
	result, err := h.DB.ExecContext(r.Context(), `
		UPDATE payment_requests
		SET status = 'rejected', reviewed_at = now(),
		    raw_payload = COALESCE(raw_payload, '{}'::jsonb) || jsonb_build_object('reason', $2::text)
		WHERE id = $1 AND status = 'pending'`, paymentRequestID, reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment request update failed"})
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment request update failed"})
		return
	}
	if rowsAffected == 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "payment request not found or already reviewed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "rejected"})
}

type entitlementActivationResult struct {
	Activated        bool
	PackageID        string
	OutputsTotal     int
	OutputsRemaining int
}

func normalizePackageID(packageID string) string {
	switch strings.ToLower(strings.TrimSpace(packageID)) {
	case "starter":
		return "starter"
	case "builder", "pro", "professional":
		return "professional"
	case "studio", "enterprise":
		return "enterprise"
	default:
		return ""
	}
}

func packageOutputCount(packageID string) (int, bool) {
	pack, ok := koscheiPackages[normalizePackageID(packageID)]
	return pack.Outputs, ok
}

func packageName(packageID string) string {
	pack, ok := koscheiPackages[normalizePackageID(packageID)]
	if !ok {
		return ""
	}
	return pack.Name
}

func normalizePaymentProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "kosch", "kosch_token", "koschei_token":
		return koschPaymentProvider
	case "shopier", "shopier_manual", "paddle", "owner_manual":
		return strings.ToLower(strings.TrimSpace(provider))
	default:
		return "owner_manual"
	}
}

func sanitizePaymentSignature(signature string) string {
	signature = strings.TrimSpace(signature)
	if len(signature) < 40 || len(signature) > 120 {
		return ""
	}
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, char := range signature {
		if !strings.ContainsRune(alphabet, char) {
			return ""
		}
	}
	return signature
}

func (h *Handler) activatePackageEntitlement(ctx context.Context, email, packageID, paymentProvider, externalPaymentID string) (entitlementActivationResult, error) {
	if err := ensurePaymentSchema(ctx, h.DB); err != nil {
		return entitlementActivationResult{}, err
	}
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return entitlementActivationResult{}, err
	}
	defer tx.Rollback()
	result, err := activatePackageEntitlementTx(ctx, tx, email, packageID, paymentProvider, externalPaymentID, "")
	if err != nil {
		return entitlementActivationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return entitlementActivationResult{}, err
	}
	return result, nil
}

func activatePackageEntitlementTx(ctx context.Context, tx *sql.Tx, email, packageID, paymentProvider, externalPaymentID, paymentRequestID string) (entitlementActivationResult, error) {
	return activatePackageEntitlementDetailedTx(ctx, tx, email, packageID, paymentProvider, externalPaymentID, paymentRequestID, "", "", nil)
}

func activatePackageEntitlementDetailedTx(ctx context.Context, tx *sql.Tx, email, packageID, paymentProvider, externalPaymentID, paymentRequestID, orderID, productID string, expiresAt any) (entitlementActivationResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	packageID = normalizePackageID(packageID)
	provider := normalizePaymentProvider(paymentProvider)
	externalPaymentID = strings.TrimSpace(externalPaymentID)
	paymentRequestID = strings.TrimSpace(paymentRequestID)
	orderID = strings.TrimSpace(orderID)
	outputs, ok := packageOutputCount(packageID)
	if email == "" || !ok || outputs <= 0 {
		return entitlementActivationResult{}, errors.New("invalid entitlement activation input")
	}

	if externalPaymentID != "" {
		var existingID, existingPackage string
		var existingTotal, existingRemaining int
		err := tx.QueryRowContext(ctx, `
			SELECT id::text, COALESCE(plan_id,''), COALESCE(outputs_total,0), COALESCE(outputs_remaining,0)
			FROM entitlements
			WHERE payment_provider = $1 AND external_payment_id = $2
			ORDER BY created_at DESC
			LIMIT 1`, provider, externalPaymentID).Scan(&existingID, &existingPackage, &existingTotal, &existingRemaining)
		if err == nil {
			_, err = tx.ExecContext(ctx, `
				UPDATE entitlements
				SET email=lower($1), plan_id=$2, status='active', starts_at=COALESCE(starts_at, now()), expires_at=$3,
				    order_id=COALESCE(NULLIF($4,'')::uuid, order_id), outputs_total=GREATEST(COALESCE(outputs_total,0), $5),
				    outputs_remaining=GREATEST(COALESCE(outputs_remaining,0), $5), updated_at=now()
				WHERE id=$6::uuid`, email, packageID, expiresAt, orderID, outputs, existingID)
			if err != nil {
				return entitlementActivationResult{}, err
			}
			return entitlementActivationResult{Activated: false, PackageID: normalizePackageID(existingPackage), OutputsTotal: maxInt(existingTotal, outputs), OutputsRemaining: maxInt(existingRemaining, outputs)}, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return entitlementActivationResult{}, err
		}
	}

	var activeID string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM entitlements
		WHERE lower(email)=lower($1)
		  AND status='active'
		  AND COALESCE(plan_id, '') <> ''
		  AND COALESCE(plan_id, '') <> 'free'
		ORDER BY updated_at DESC NULLS LAST, created_at DESC
		LIMIT 1
		FOR UPDATE`, email).Scan(&activeID)
	if err == nil {
		_, err = tx.ExecContext(ctx, `
			UPDATE entitlements
			SET plan_id=$2, payment_request_id=COALESCE(NULLIF($3,''), payment_request_id), payment_provider=$4,
			    external_payment_id=NULLIF($5,''), outputs_total=GREATEST(COALESCE(outputs_total,0), $6),
			    outputs_remaining=GREATEST(COALESCE(outputs_remaining,0), $6), starts_at=COALESCE(starts_at, now()),
			    expires_at=$7, order_id=COALESCE(NULLIF($8,'')::uuid, order_id), updated_at=now()
			WHERE id=$1::uuid`, activeID, packageID, paymentRequestID, provider, externalPaymentID, outputs, expiresAt, orderID)
		if err != nil {
			return entitlementActivationResult{}, err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		if paymentRequestID != "" {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO entitlements (email, plan_id, payment_request_id, payment_provider, external_payment_id, outputs_total, outputs_remaining, status, starts_at, expires_at, order_id, created_at, updated_at)
				VALUES (lower($1), $2, $3, $4, NULLIF($5, ''), $6, $6, 'active', now(), $7, NULLIF($8,'')::uuid, now(), now())`, email, packageID, paymentRequestID, provider, externalPaymentID, outputs, expiresAt, orderID)
		} else {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO entitlements (email, plan_id, payment_provider, external_payment_id, outputs_total, outputs_remaining, status, starts_at, expires_at, order_id, created_at, updated_at)
				VALUES (lower($1), $2, $3, NULLIF($4, ''), $5, $5, 'active', now(), $6, NULLIF($7,'')::uuid, now(), now())`, email, packageID, provider, externalPaymentID, outputs, expiresAt, orderID)
		}
		if err != nil {
			return entitlementActivationResult{}, err
		}
	} else {
		return entitlementActivationResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE app_user_profiles
		SET plan_id = CASE
			WHEN CASE $2 WHEN 'enterprise' THEN 3 WHEN 'professional' THEN 2 WHEN 'starter' THEN 1 ELSE 0 END >=
			     CASE COALESCE(plan_id, 'free') WHEN 'enterprise' THEN 3 WHEN 'studio' THEN 3 WHEN 'professional' THEN 2 WHEN 'builder' THEN 2 WHEN 'starter' THEN 1 ELSE 0 END
			THEN $2
			ELSE plan_id
		END,
		updated_at = now()
		WHERE lower(email) = lower($1)`, email, packageID); err != nil {
		return entitlementActivationResult{}, err
	}

	return entitlementActivationResult{Activated: true, PackageID: packageID, OutputsTotal: outputs, OutputsRemaining: outputs}, nil
}
