package handlers

import (
	"net/http"
)

type manualPaymentRequest struct {
	Email string `json:"email"`
	Plan string `json:"plan"`
	PaymentProvider string `json:"payment_provider"`
	PaymentReference string `json:"payment_reference"`
	Note string `json:"note"`
}

func (h *Handler) ManualPaymentRequest(w http.ResponseWriter, r *http.Request) {
	var req manualPaymentRequest
	if err := decodeJSON(r, &req); err != nil || req.Email == "" || req.Plan == "" || req.PaymentProvider == "" { writeJSON(w,400,map[string]string{"error":"invalid body"}); return }
	_, err := h.DB.Exec(`INSERT INTO payment_requests (email, plan, payment_provider, payment_reference, note) VALUES ($1,$2,$3,$4,$5)`, req.Email, req.Plan, req.PaymentProvider, req.PaymentReference, req.Note)
	if err != nil { writeJSON(w,500,map[string]string{"error":"db insert failed"}); return }
	writeJSON(w, http.StatusCreated, map[string]string{"message":"Payment request created. Your plan will be activated manually after payment confirmation."})
}
