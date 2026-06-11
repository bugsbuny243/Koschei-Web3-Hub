package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"koschei/api/internal/jobs"
)

type asyncWeb3Request struct {
	Mint      string `json:"mint"`
	Address   string `json:"address"`
	Signature string `json:"signature"`
	Network   string `json:"network"`
}

func (h *Handler) CreateWeb3Job(w http.ResponseWriter, r *http.Request) {
	if h.JobStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "job service unavailable"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	jobType := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	jobType = strings.Trim(jobType, "/")
	switch jobType {
	case "token-scan":
		jobType = "token_scan"
	case "wallet-score":
		jobType = "wallet_score"
	case "tx-decode":
		jobType = "tx_decode"
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown job type"})
		return
	}
	var req asyncWeb3Request
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if req.Network == "" {
		req.Network = defaultNetworkForJob(jobType)
	}
	target := strings.TrimSpace(req.Mint)
	if target == "" {
		target = strings.TrimSpace(req.Address)
	}
	if target == "" {
		target = strings.TrimSpace(req.Signature)
	}
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target required"})
		return
	}
	isPrivileged, credits, _ := h.userCreditsAndRole(claims.Sub)
	if !isPrivileged && credits < 1 {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	job, err := h.JobStore.Create(r.Context(), jobs.CreateInput{UserID: claims.Sub, Email: claims.Email, Type: jobType, Network: req.Network, Target: target, Request: req})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "job create failed"})
		return
	}
	if h.JobQueue != nil {
		_ = h.JobQueue.Publish(job)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"job_id": job.ID, "status": job.Status, "poll_url": "/api/jobs/" + job.ID, "credits_reserved": 1})
}

func (h *Handler) GetWeb3Job(w http.ResponseWriter, r *http.Request) {
	if h.JobStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "job service unavailable"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	job, err := h.JobStore.Get(r.Context(), id, claims.Sub)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "job read failed"})
		return
	}
	resp := map[string]any{"id": job.ID, "job_type": job.Type, "status": job.Status, "network": job.Network, "target": job.Target, "progress": job.Progress, "attempts": job.Attempts, "queued_at": job.QueuedAt, "updated_at": job.UpdatedAt}
	if len(job.ResultPayload) > 0 && string(job.ResultPayload) != "null" {
		var raw any
		if json.Unmarshal(job.ResultPayload, &raw) == nil {
			resp["result"] = raw
		}
	}
	if job.ErrorCode != "" {
		resp["error_code"] = job.ErrorCode
		resp["error_message"] = job.ErrorMessage
	}
	writeJSON(w, http.StatusOK, resp)
}

func defaultNetworkForJob(jobType string) string {
	if jobType == "token_scan" {
		return "solana-mainnet"
	}
	return "solana-devnet"
}
