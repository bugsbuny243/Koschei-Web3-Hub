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

const paymentRequestMessage = "Payment request received. Owner review required."

type shopierPack struct {
	AmountTRY int
	Outputs   int
}

var shopierPacks = map[string]shopierPack{
	"starter": {AmountTRY: 899, Outputs: 25},
	"builder": {AmountTRY: 2299, Outputs: 100},
	"studio":  {AmountTRY: 4999, Outputs: 300},
}

type paymentRequestInput struct {
	FullName         string `json:"full_name"`
	ProductID        string `json:"product_id"`
	PaymentReference string `json:"payment_reference"`
	Note             string `json:"note"`
}

type paymentRequestReviewInput struct {
	PaymentRequestID string `json:"payment_request_id"`
	ID               string `json:"id"`
	Reason           string `json:"reason"`
}

func ensurePaymentSchema(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS payment_requests (id uuid PRIMARY KEY DEFAULT gen_random_uuid())`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS email text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS full_name text NOT NULL DEFAULT ''`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS product_id text`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS amount_try integer`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'TRY'`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending'`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now()`,
		`ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS reviewed_at timestamptz`,
		`ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS payment_request_id text`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

type paymentRequestRecord struct {
	ID         string         `json:"id"`
	Email      string         `json:"email"`
	FullName   string         `json:"full_name"`
	ProductID  string         `json:"product_id"`
	AmountTRY  int            `json:"amount_try"`
	Currency   string         `json:"currency"`
	Status     string         `json:"status"`
	RawPayload map[string]any `json:"raw_payload"`
	CreatedAt  time.Time      `json:"created_at"`
	ReviewedAt *time.Time     `json:"reviewed_at,omitempty"`
}

func (h *Handler) PaymentRequest(w http.ResponseWriter, r *http.Request) {
	if err := ensurePaymentSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment schema unavailable"})
		return
	}
	if !h.Limiter.allow("billing:"+clientIP(r), 10, 10_000_000_000) {
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
	req.FullName = strings.TrimSpace(req.FullName)
	req.ProductID = strings.ToLower(strings.TrimSpace(req.ProductID))
	req.PaymentReference = strings.TrimSpace(req.PaymentReference)
	req.Note = strings.TrimSpace(req.Note)
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	pack, ok := shopierPacks[req.ProductID]
	if email == "" || req.FullName == "" || !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	rawPayload, err := json.Marshal(map[string]string{
		"payment_reference": req.PaymentReference,
		"note":              req.Note,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payload encoding failed"})
		return
	}
	if _, err := h.DB.ExecContext(r.Context(), `
		INSERT INTO payment_requests (email, full_name, product_id, amount_try, currency, status, raw_payload, created_at)
		VALUES ($1, $2, $3, $4, 'TRY', 'pending', $5::jsonb, now())`, email, req.FullName, req.ProductID, pack.AmountTRY, string(rawPayload)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db insert failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "status": "pending", "message": paymentRequestMessage})
}

func (h *Handler) OwnerPaymentRequestsList(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensurePaymentSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "payment schema unavailable"})
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id::text, email, COALESCE(full_name, ''), COALESCE(product_id, ''), COALESCE(amount_try, 0), COALESCE(currency, 'TRY'), status,
		       COALESCE(raw_payload, '{}'::jsonb), created_at, reviewed_at
		FROM payment_requests
		ORDER BY created_at DESC
		LIMIT 200`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()

	requests := make([]paymentRequestRecord, 0)
	for rows.Next() {
		var request paymentRequestRecord
		var rawPayload []byte
		if err := rows.Scan(&request.ID, &request.Email, &request.FullName, &request.ProductID, &request.AmountTRY, &request.Currency, &request.Status, &rawPayload, &request.CreatedAt, &request.ReviewedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		if err := json.Unmarshal(rawPayload, &request.RawPayload); err != nil {
			request.RawPayload = map[string]any{}
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
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

	var email, productID, status string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT lower(email), product_id, status
		FROM payment_requests
		WHERE id = $1
		FOR UPDATE`, paymentRequestID).Scan(&email, &productID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "payment request not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	pack, ok := shopierPacks[productID]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown product_id"})
		return
	}
	if status != "pending" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "payment request already reviewed"})
		return
	}

	var paymentRequestIDColumnExists bool
	if err := tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'entitlements' AND column_name = 'payment_request_id'
		)`).Scan(&paymentRequestIDColumnExists); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "schema check failed"})
		return
	}
	if paymentRequestIDColumnExists {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO entitlements (email, plan_id, payment_request_id, outputs_total, outputs_remaining, status)
			VALUES (lower($1), $2, $3, $4, $4, 'active')`, email, productID, paymentRequestID, pack.Outputs)
	} else {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO entitlements (email, plan_id, outputs_total, outputs_remaining, status)
			VALUES (lower($1), $2, $3, $3, 'active')`, email, productID, pack.Outputs)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "entitlement insert failed"})
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "approved"})
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
