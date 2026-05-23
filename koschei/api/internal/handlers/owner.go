package handlers

import (
	"net/http"
	"strings"
)

func (h *Handler) OwnerPaymentRequests(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	rows, err := h.DB.Query(`SELECT id, email, plan, payment_provider, payment_reference, note, status, created_at, reviewed_at FROM payment_requests ORDER BY created_at DESC`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, email, plan, provider, status, created string
		var ref, note, reviewed *string
		if err := rows.Scan(&id, &email, &plan, &provider, &ref, &note, &status, &created, &reviewed); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		out = append(out, map[string]any{"id": id, "email": email, "plan": plan, "payment_provider": provider, "payment_reference": ref, "note": note, "status": status, "created_at": created, "reviewed_at": reviewed})
	}
	writeJSON(w, 200, map[string]any{"payment_requests": out})
}

type activateReq struct {
	Email, Plan, Note string
	Credits           int
}
type grantReq struct {
	Email, Reason string
	Credits       int
}

type jobStatusReq struct {
	Status string `json:"status"`
	Result string `json:"result"`
}

func (h *Handler) OwnerActivatePlan(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	var req activateReq
	if err := decodeJSON(r, &req); err != nil || !validEmail(req.Email) || !validPaidActivationPlan(req.Plan) || req.Credits == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	_, err := h.DB.Exec(`INSERT INTO credits_ledger (email, amount, reason) VALUES ($1,$2,$3)`, req.Email, req.Credits, "plan activation: "+req.Note)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	_, err = h.DB.Exec(`UPDATE payment_requests SET status='approved', reviewed_at=now() WHERE email=$1 AND plan=$2 AND status='pending'`, req.Email, req.Plan)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	writeJSON(w, 200, map[string]string{"message": "plan activated"})
}

func (h *Handler) OwnerGrantCredits(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	var req grantReq
	if err := decodeJSON(r, &req); err != nil || !validEmail(req.Email) || req.Credits == 0 || req.Reason == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	_, err := h.DB.Exec(`INSERT INTO credits_ledger (email, amount, reason) VALUES ($1,$2,$3)`, req.Email, req.Credits, req.Reason)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]string{"message": "credits granted"})
}

func (h *Handler) OwnerUpdateJobStatus(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/owner/jobs/")
	id = strings.TrimSuffix(id, "/status")
	var req jobStatusReq
	if err := decodeJSON(r, &req); err != nil || id == "" || !validStatus(req.Status) {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	_, err := h.DB.Exec(`UPDATE generation_jobs SET status=$1, result=$2, updated_at=now() WHERE id=$3`, req.Status, req.Result, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]string{"message": "job updated"})
}
