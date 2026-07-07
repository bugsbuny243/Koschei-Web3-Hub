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
		"provider": "kosch_token",
		"mode":     "token_payment_owner_approval",
		"summary": map[string]any{
			"active_entitlements":       int64(0),
			"pending_payment_requests":  int64(0),
			"approved_payments_30d":     int64(0),
			"revenue_kosch_30d":         "0",
			"latest_payment_events":     []map[string]any{},
			"failed_payment_events_24h": int64(0),
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
		"revenue_kosch_30d":         h.approvedRevenueKOSCH30d(r),
		"latest_payment_events":     h.latestManualPaymentEvents(r),
		"failed_payment_events_24h": h.failedPaymentEvents24h(r),
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

func (h *Handler) approvedRevenueKOSCH30d(r *http.Request) string {
	var total string
	_ = h.DBRead.QueryRowContext(r.Context(), `
		SELECT COALESCE(sum(CASE WHEN COALESCE(amount_kosch,'') ~ '^[0-9]+(\.[0-9]+)?$' THEN amount_kosch::numeric ELSE 0 END),0)::text
		FROM payment_requests
		WHERE status='approved'
		  AND COALESCE(payment_provider, raw_payload->>'payment_provider', 'kosch_token')='kosch_token'
		  AND COALESCE(reviewed_at,created_at) >= now() - interval '30 days'`).Scan(&total)
	if strings.TrimSpace(total) == "" {
		return "0"
	}
	return total
}

func (h *Handler) failedPaymentEvents24h(r *http.Request) int64 {
	var count int64
	_ = h.DBRead.QueryRowContext(r.Context(), `SELECT count(*) FROM security_audit_events WHERE event_type IN ('kosch_payment_invalid','kosch_payment_rejected') AND created_at >= now() - interval '24 hours'`).Scan(&count)
	return count
}

func (h *Handler) latestManualPaymentEvents(r *http.Request) []map[string]any {
	rows, err := h.DBRead.QueryContext(r.Context(), `SELECT id::text, COALESCE(email,''), COALESCE(product_slug,plan,''), COALESCE(amount_kosch,''), COALESCE(currency,'KOSCH'), COALESCE(payment_provider,'kosch_token'), status, created_at, reviewed_at FROM payment_requests ORDER BY created_at DESC LIMIT 10`)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, email, product, amountKOSCH, currency, provider, status string
		var createdAt time.Time
		var reviewedAt any
		if err := rows.Scan(&id, &email, &product, &amountKOSCH, &currency, &provider, &status, &createdAt, &reviewedAt); err != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "email": email, "product_id": product, "amount_kosch": amountKOSCH, "currency": currency, "payment_provider": provider, "status": status, "created_at": createdAt, "reviewed_at": reviewedAt})
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
