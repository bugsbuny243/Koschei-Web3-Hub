package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	b2bTokenScanEndpoint = "/api/v1/scan/token"
	maxBatchTokenScans   = 20
	batchScanWorkers     = 4
)

type b2bTokenScanRequest struct {
	Mint      string   `json:"mint"`
	Address   string   `json:"address"`
	Mints     []string `json:"mints"`
	Network   string   `json:"network"`
	IncludeAI bool     `json:"include_ai"`
}

type b2bBatchScanItem struct {
	Mint   string `json:"mint"`
	Status string `json:"status"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type b2bExistingRequest struct {
	RequestID       string
	Status          string
	CreditsReserved int
}

// B2BTokenScan godoc
// @Summary Token risk analizi
// @Description API anahtarı ile tekli veya en fazla 20 Solana token için risk analizi başlatır.
// @Tags B2B API
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body b2bTokenScanRequest true "Token analiz isteği"
// @Success 202 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 402 {object} map[string]string
// @Failure 429 {object} map[string]string
// @Router /api/v1/scan/token [post]
func (h *Handler) B2BTokenScan(w http.ResponseWriter, r *http.Request) {
	principal, ok := apiPrincipalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req b2bTokenScanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	targets := normalizeB2BTokenTargets(req)
	if len(targets) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mint_required"})
		return
	}
	if len(targets) > maxBatchTokenScans {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":       "batch_too_large",
			"max_targets": maxBatchTokenScans,
		})
		return
	}
	if req.Network == "" {
		req.Network = "solana-mainnet"
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "idempotency_key_too_long"})
		return
	}

	requestID, err := newUUID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "request_id_failed"})
		return
	}
	cost := len(targets)
	existing, err := h.reserveB2BRequest(r.Context(), principal, requestID, idempotencyKey, cost)
	if err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	if existing != nil {
		statusCode := http.StatusAccepted
		if existing.Status == "completed" || existing.Status == "refunded" {
			statusCode = http.StatusOK
		}
		writeJSON(w, statusCode, map[string]any{
			"request_id":        existing.RequestID,
			"status":            existing.Status,
			"cost_credits":      existing.CreditsReserved,
			"idempotent_replay": true,
			"result_url":        "/api/v1/usage?request_id=" + existing.RequestID,
		})
		return
	}

	if len(targets) == 1 {
		go h.runB2BTokenScan(requestID, req.Network, targets[0])
		writeJSON(w, http.StatusAccepted, map[string]any{
			"request_id":   requestID,
			"status":       "queued",
			"mode":         "single",
			"target_count": 1,
			"cost_credits": cost,
			"result_url":   "/api/v1/usage?request_id=" + requestID,
			"message":      "Token analizi kuyruğa alındı.",
		})
		return
	}

	go h.runB2BTokenBatch(requestID, req.Network, targets)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"request_id":   requestID,
		"status":       "queued",
		"mode":         "batch",
		"target_count": len(targets),
		"cost_credits": cost,
		"result_url":   "/api/v1/usage?request_id=" + requestID,
		"message":      "Toplu token analizi kuyruğa alındı.",
	})
}

func normalizeB2BTokenTargets(req b2bTokenScanRequest) []string {
	candidates := req.Mints
	if len(candidates) == 0 {
		candidates = []string{firstNonEmptyString(req.Mint, req.Address)}
	}
	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func (h *Handler) reserveB2BRequest(ctx context.Context, principal apiPrincipal, requestID, idempotencyKey string, cost int) (*b2bExistingRequest, error) {
	if idempotencyKey != "" {
		existing, err := h.lookupB2BIdempotentRequest(ctx, principal.KeyID, idempotencyKey)
		if err == nil {
			return existing, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}
	if err := h.ensureB2BMonthlyCapacity(ctx, principal, cost); err != nil {
		return nil, err
	}
	if idempotencyKey == "" {
		return nil, h.reserveAPICredits(ctx, principal, b2bTokenScanEndpoint, requestID, cost)
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE app_user_profiles SET credits=credits-$1, updated_at=now() WHERE auth_subject=$2 AND credits >= $1`, cost, principal.AuthSubject)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	if rows != 1 {
		return nil, sql.ErrNoRows
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO api_usage_events (api_key_id,auth_subject,email,endpoint,request_id,credits_reserved,status,idempotency_key)
		VALUES ($1,$2,lower($3),$4,$5,$6,'reserved',$7)`,
		principal.KeyID, principal.AuthSubject, principal.Email, b2bTokenScanEndpoint, requestID, cost, idempotencyKey)
	if err != nil {
		_ = tx.Rollback()
		existing, lookupErr := h.lookupB2BIdempotentRequest(ctx, principal.KeyID, idempotencyKey)
		if lookupErr == nil {
			return existing, nil
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO api_credit_ledger (api_key_id,auth_subject,email,amount,event_type,reason,request_id) VALUES ($1,$2,lower($3),$4,'reserve',$5,$6)`, principal.KeyID, principal.AuthSubject, principal.Email, -cost, b2bTokenScanEndpoint, requestID); err != nil {
		return nil, err
	}
	return nil, tx.Commit()
}

func (h *Handler) lookupB2BIdempotentRequest(ctx context.Context, apiKeyID, idempotencyKey string) (*b2bExistingRequest, error) {
	var existing b2bExistingRequest
	err := h.DB.QueryRowContext(ctx, `
		SELECT request_id::text,status,credits_reserved
		FROM api_usage_events
		WHERE api_key_id=$1 AND endpoint=$2 AND idempotency_key=$3
		LIMIT 1`, apiKeyID, b2bTokenScanEndpoint, idempotencyKey).
		Scan(&existing.RequestID, &existing.Status, &existing.CreditsReserved)
	if err != nil {
		return nil, err
	}
	return &existing, nil
}

func (h *Handler) ensureB2BMonthlyCapacity(ctx context.Context, principal apiPrincipal, cost int) error {
	limit := principal.MonthlyLimit
	if limit <= 0 {
		limit = 1000
	}
	monthStart := time.Now().UTC().Format("2006-01-") + "01"
	var used int
	if err := h.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(GREATEST(credits_reserved, credits_charged)),0) FROM api_usage_events WHERE api_key_id=$1 AND created_at >= $2::date`, principal.KeyID, monthStart).Scan(&used); err != nil {
		return err
	}
	if used+cost > limit {
		return sql.ErrNoRows
	}
	return nil
}

func (h *Handler) runB2BTokenScan(requestID, network, mint string) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := h.tokenService().ScanToken(ctx, network, mint)
	if err != nil {
		h.refundAPICredits(ctx, requestID, "rpc_scan_failed")
		return
	}
	payload, _ := json.Marshal(result)
	_, _ = h.DB.ExecContext(ctx, `UPDATE api_usage_events SET metadata=$1::jsonb WHERE request_id=$2`, string(payload), requestID)
	h.finalizeAPIUsage(ctx, requestID, int(time.Since(start).Milliseconds()))
}

func (h *Handler) runB2BTokenBatch(requestID, network string, mints []string) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	items := make([]b2bBatchScanItem, len(mints))
	jobs := make(chan int)
	workerCount := batchScanWorkers
	if len(mints) < workerCount {
		workerCount = len(mints)
	}
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer wg.Done()
			for index := range jobs {
				mint := mints[index]
				result, err := h.tokenService().ScanToken(ctx, network, mint)
				if err != nil {
					items[index] = b2bBatchScanItem{Mint: mint, Status: "failed", Error: "scan_failed"}
					continue
				}
				items[index] = b2bBatchScanItem{Mint: mint, Status: "completed", Result: result}
			}
		}()
	}
	for index := range mints {
		jobs <- index
	}
	close(jobs)
	wg.Wait()

	succeeded := 0
	for _, item := range items {
		if item.Status == "completed" {
			succeeded++
		}
	}
	payload, _ := json.Marshal(map[string]any{
		"mode":      "batch",
		"network":   network,
		"total":     len(items),
		"succeeded": succeeded,
		"failed":    len(items) - succeeded,
		"items":     items,
	})

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()
	h.finalizeB2BBatchUsage(dbCtx, requestID, succeeded, string(payload), int(time.Since(start).Milliseconds()))
}

func (h *Handler) finalizeB2BBatchUsage(ctx context.Context, requestID string, succeeded int, metadata string, latencyMS int) {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	var principal apiPrincipal
	var reserved int
	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT api_key_id::text,auth_subject,email,credits_reserved,status FROM api_usage_events WHERE request_id=$1 FOR UPDATE`, requestID).
		Scan(&principal.KeyID, &principal.AuthSubject, &principal.Email, &reserved, &currentStatus); err != nil || currentStatus != "reserved" {
		return
	}
	charged := succeeded
	if charged > reserved {
		charged = reserved
	}
	if charged < 0 {
		charged = 0
	}
	refund := reserved - charged
	status := "completed"
	errorCode := ""
	if charged == 0 {
		status = "refunded"
		errorCode = "batch_failed"
	} else if refund > 0 {
		errorCode = "partial_failure"
	}
	if refund > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE app_user_profiles SET credits=credits+$1, updated_at=now() WHERE auth_subject=$2`, refund, principal.AuthSubject); err != nil {
			return
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO api_credit_ledger (api_key_id,auth_subject,email,amount,event_type,reason,request_id) VALUES ($1,$2,lower($3),$4,'refund',$5,$6)`, principal.KeyID, principal.AuthSubject, principal.Email, refund, firstNonEmptyString(errorCode, "batch_partial_refund"), requestID); err != nil {
			return
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE api_usage_events
		SET status=$1, credits_charged=$2, error_code=NULLIF($3,''), metadata=$4::jsonb, latency_ms=$5, completed_at=now()
		WHERE request_id=$6`, status, charged, errorCode, metadata, latencyMS, requestID); err != nil {
		return
	}
	_ = tx.Commit()
}
