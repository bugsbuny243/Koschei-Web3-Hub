package handlers

import (
	"database/sql"
	"net/http"
	"strings"
)

type manualPaymentRequest struct {
	Email            string `json:"email"`
	Plan             string `json:"plan"`
	PaymentProvider  string `json:"payment_provider"`
	PaymentReference string `json:"payment_reference"`
	Note             string `json:"note"`
}

func (h *Handler) ManualPaymentRequest(w http.ResponseWriter, r *http.Request) {
	var req manualPaymentRequest
	if !h.Limiter.allow("billing:"+clientIP(r), 10, 10_000_000_000) {
		writeJSON(w, 429, map[string]string{"error": "rate limited"})
		return
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Plan = strings.TrimSpace(req.Plan)
	req.PaymentProvider = strings.TrimSpace(req.PaymentProvider)
	req.PaymentReference = strings.TrimSpace(req.PaymentReference)
	req.Note = strings.TrimSpace(req.Note)

	if !validEmail(req.Email) || !validPlan(req.Plan) || req.PaymentProvider == "" || req.PaymentReference == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}

	var duplicateID string
	err := h.DB.QueryRow(`SELECT id FROM payment_requests WHERE payment_provider=$1 AND payment_reference=$2 AND status IN ('pending','approved') LIMIT 1`, req.PaymentProvider, req.PaymentReference).Scan(&duplicateID)
	if err == nil {
		writeJSON(w, 409, map[string]string{"error": "duplicate payment request: payment_provider + payment_reference already exists with pending/approved status"})
		return
	}
	if err != nil && err != sql.ErrNoRows {
		writeJSON(w, 500, map[string]string{"error": "db check failed"})
		return
	}

	_, err = h.DB.Exec(`INSERT INTO payment_requests (email, plan, payment_provider, payment_reference, note) VALUES ($1,$2,$3,$4,$5)`, req.Email, req.Plan, req.PaymentProvider, req.PaymentReference, req.Note)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db insert failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": "Payment request created. Your plan will be activated manually after payment confirmation."})
}
