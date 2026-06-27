package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

func (h *Handler) OwnerPaymentHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]any{
		"ok":       true,
		"provider": "shopier",
		"mode":     "manual_owner_approval",
		"summary": map[string]any{
			"active_entitlements":       int64(0),
			"pending_payment_requests":  int64(0),
			"approved_payments_30d":     int64(0),
			"revenue_try_30d":           int64(0),
			"latest_payment_events":     []map[string]any{},
			"failed_webhook_events_24h": int64(0),
		},
	}
	if h == nil || h.DBRead == nil {
		response["ok"] = false
		response["error"] = "Payment health unavailable."
		writeJSON(w, http.StatusOK, response)
		return
	}
	response["summary"] = map[string]any{
		"active_entitlements":       h.countActiveEntitlements(r),
		"pending_payment_requests":  h.countPaymentRequests(r, "pending"),
		"approved_payments_30d":     h.countApprovedPayments30d(r),
		"revenue_try_30d":           h.approvedRevenueTRY30d(r),
		"latest_payment_events":     h.latestManualPaymentEvents(r),
		"failed_webhook_events_24h": h.failedWebhookEvents24h(r),
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) countActiveEntitlements(r *http.Request) int64 {
	var count int64
	_ = h.DBRead.QueryRowContext(r.Context(), `SELECT count(*) FROM entitlements WHERE status='active' AND (expires_at IS NULL OR expires_at > now())`).Scan(&count)
	return count
}

func (h *Handler) countPaymentRequests(r *http.Request, status string) int64 {
	var count int64
	_ = h.DBRead.QueryRowContext(r.Context(), `SELECT count(*) FROM payment_requests WHERE status=$1`, status).Scan(&count)
	return count
}

func (h *Handler) countApprovedPayments30d(r *http.Request) int64 {
	var count int64
	_ = h.DBRead.QueryRowContext(r.Context(), `SELECT count(*) FROM payment_requests WHERE status='approved' AND COALESCE(reviewed_at,created_at) >= now() - interval '30 days'`).Scan(&count)
	return count
}

func (h *Handler) approvedRevenueTRY30d(r *http.Request) int64 {
	var total int64
	_ = h.DBRead.QueryRowContext(r.Context(), `SELECT COALESCE(sum(amount_try),0) FROM payment_requests WHERE status='approved' AND COALESCE(reviewed_at,created_at) >= now() - interval '30 days'`).Scan(&total)
	return total
}

func (h *Handler) failedWebhookEvents24h(r *http.Request) int64 {
	var count int64
	_ = h.DBRead.QueryRowContext(r.Context(), `SELECT count(*) FROM security_audit_events WHERE event_type='shopier_webhook_invalid' AND created_at >= now() - interval '24 hours'`).Scan(&count)
	return count
}

func (h *Handler) latestManualPaymentEvents(r *http.Request) []map[string]any {
	rows, err := h.DBRead.QueryContext(r.Context(), `SELECT id::text, COALESCE(email,''), COALESCE(product_slug,plan,''), COALESCE(amount_try,0), COALESCE(currency,'TRY'), status, created_at, reviewed_at FROM payment_requests ORDER BY created_at DESC LIMIT 10`)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, email, product, currency, status string
		var amount int64
		var createdAt time.Time
		var reviewedAt any
		if err := rows.Scan(&id, &email, &product, &amount, &currency, &status, &createdAt, &reviewedAt); err != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "email": email, "product_id": product, "amount_try": amount, "currency": currency, "status": status, "created_at": createdAt, "reviewed_at": reviewedAt})
	}
	return items
}

func writePaymentAudit(ctxReq *http.Request, h *Handler, eventType, severity string, metadata map[string]any) {
	if h == nil || ctxReq == nil {
		return
	}
	services.WriteSecurityAuditEvent(ctxReq.Context(), h.DB, services.SecurityAuditEvent{EventType: eventType, ActorType: "payment", IP: requestAuditIP(ctxReq), UserAgent: ctxReq.UserAgent(), Path: ctxReq.URL.Path, Severity: severity, Metadata: sanitizePaymentAuditMetadata(metadata)})
}

func sanitizePaymentAuditMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for k, v := range metadata {
		if services.IsSensitiveEnvName(k) || strings.Contains(strings.ToLower(k), "body") {
			out[k] = "redacted"
			continue
		}
		if raw, ok := v.(json.RawMessage); ok {
			out[k] = len(raw)
			continue
		}
		out[k] = v
	}
	return out
}
