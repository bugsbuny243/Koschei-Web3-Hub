package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const koschTokenPaymentProvider = "kosch_token"
const koschDefaultTokenMint = "HHPpU9u56Bwxov12nf7DXUCuv6h1q5j1xgGS3yukpump"

var koschPackageAmounts = map[string]string{
	"starter":      "250000",
	"professional": "750000",
	"enterprise":   "2000000",
}

type koschPaymentRequestInput struct {
	FullName             string `json:"full_name"`
	ProductID            string `json:"product_id"`
	RegisteredEmail      string `json:"registered_email"`
	CustomerEmail        string `json:"customer_email"`
	WalletAddress        string `json:"wallet_address"`
	PayerWallet          string `json:"payer_wallet"`
	TransactionSignature string `json:"transaction_signature"`
	TxSignature          string `json:"tx_signature"`
	PaymentReference     string `json:"payment_reference"`
	Note                 string `json:"note"`
}

func ensureKoschPaymentSchema(r *http.Request, db *sql.DB) error {
	if db == nil {
		return errors.New("db nil")
	}
	if err := ensurePaymentSchema(r.Context(), db); err != nil {
		return err
	}
	statements := []string{
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS amount_kosch text NOT NULL DEFAULT ''`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS transaction_signature text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS payer_wallet text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS treasury_wallet text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS token_mint text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS chain text NOT NULL DEFAULT 'solana'`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS price_kosch text NOT NULL DEFAULT ''`,
		`UPDATE products SET price_try_cents=0, price_usd_cents=0, price_kosch='250000', shopier_url='', updated_at=now() WHERE slug='starter'`,
		`UPDATE products SET price_try_cents=0, price_usd_cents=0, price_kosch='750000', shopier_url='', updated_at=now() WHERE slug IN ('professional','builder')`,
		`UPDATE products SET price_try_cents=0, price_usd_cents=0, price_kosch='2000000', shopier_url='', updated_at=now() WHERE slug IN ('enterprise','studio')`,
		`CREATE UNIQUE INDEX IF NOT EXISTS payment_requests_kosch_tx_sig_idx ON payment_requests (lower(transaction_signature)) WHERE transaction_signature IS NOT NULL AND transaction_signature <> ''`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(r.Context(), statement); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) KoschPaymentRequest(w http.ResponseWriter, r *http.Request) {
	if err := ensureKoschPaymentSchema(r, h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "kosch payment schema unavailable", "message": err.Error()})
		return
	}
	if h.Limiter != nil && !h.Limiter.allow("kosch-billing:"+clientIP(r), 10, 10_000_000_000) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limited"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req koschPaymentRequestInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	productID := normalizePackageID(req.ProductID)
	amountKOSCH := koschPackageAmounts[productID]
	signature := sanitizeKoschPaymentSignature(firstNonEmpty(req.TransactionSignature, req.TxSignature, req.PaymentReference))
	payerWallet := normalizeWallet(firstNonEmpty(req.PayerWallet, req.WalletAddress))
	fullName := strings.TrimSpace(req.FullName)
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	registeredEmail := strings.ToLower(strings.TrimSpace(req.RegisteredEmail))
	customerEmail := strings.ToLower(strings.TrimSpace(req.CustomerEmail))
	treasuryWallet := normalizeWallet(firstEnv("KOSCH_TREASURY_WALLET", "KOSCHEI_TREASURY_WALLET", "OWNER_WALLET", "KOSCHEI_OWNER_WALLET"))
	tokenMint := strings.TrimSpace(firstEnv("KOSCH_TOKEN_MINT", "KOSCHEI_TOKEN_MINT"))
	if tokenMint == "" {
		tokenMint = koschDefaultTokenMint
	}
	if email == "" || fullName == "" || productID == "" || amountKOSCH == "" || signature == "" || payerWallet == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body", "message": "Ad soyad, paket, ödeme cüzdanı ve KOSCH transaction signature zorunlu."})
		return
	}
	if treasuryWallet == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosch treasury wallet not configured"})
		return
	}
	rawPayload, err := json.Marshal(map[string]string{
		"payment_provider":      koschTokenPaymentProvider,
		"product_id":            productID,
		"amount_kosch":          amountKOSCH,
		"registered_email":      email,
		"declared_email":        firstNonEmpty(registeredEmail, customerEmail, email),
		"payer_wallet":          payerWallet,
		"treasury_wallet":       treasuryWallet,
		"token_mint":            tokenMint,
		"transaction_signature": signature,
		"chain":                 "solana",
		"note":                  strings.TrimSpace(req.Note),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payload encoding failed"})
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `
		INSERT INTO payment_requests (email, full_name, product_slug, plan, amount_try, amount_kosch, currency, status, raw_payload, payment_provider, external_payment_id, transaction_signature, payer_wallet, treasury_wallet, token_mint, chain, created_at)
		VALUES ($1, $2, $3, $3, 0, $4, 'KOSCH', 'pending', $5::jsonb, $6, $7, $7, $8, $9, $10, 'solana', now())`,
		email, fullName, productID, amountKOSCH, string(rawPayload), koschTokenPaymentProvider, signature, payerWallet, treasuryWallet, tokenMint)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "transaction signature already submitted"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db insert failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "status": "pending", "provider": koschTokenPaymentProvider, "currency": "KOSCH", "amount_kosch": amountKOSCH, "message": "KOSCH ödeme bildirimi owner paneline gönderildi."})
}

func sanitizeKoschPaymentSignature(signature string) string {
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
