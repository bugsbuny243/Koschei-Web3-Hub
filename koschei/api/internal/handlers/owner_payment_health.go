package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

func (h *Handler) OwnerPaymentHealth(w http.ResponseWriter, r *http.Request) {
	cfg := services.LoadPaddleConfigFromEnv()
	response := map[string]any{
		"ok":     true,
		"paddle": cfg.PublicStatus(),
		"summary": map[string]any{
			"active_entitlements":      0,
			"latest_payment_events":    []map[string]any{},
			"failed_webhook_events_24h": 0,
		},
	}
	if h == nil || h.DBRead == nil {
		response["ok"] = false
		response["error"] = "Payment health unavailable."
		writeJSON(w, http.StatusOK, response)
		return
	}
	summary := map[string]any{"active_entitlements": h.countActivePaddleEntitlements(r), "latest_payment_events": h.latestPaymentEvents(r), "failed_webhook_events_24h": h.failedWebhookEvents24h(r)}
	response["summary"] = summary
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) countActivePaddleEntitlements(r *http.Request) int64 {
	var count int64
	_ = h.DBRead.QueryRowContext(r.Context(), `SELECT count(*) FROM entitlements WHERE status='active' AND COALESCE(payment_provider,'')='paddle' AND (expires_at IS NULL OR expires_at > now())`).Scan(&count)
	return count
}

func (h *Handler) failedWebhookEvents24h(r *http.Request) int64 {
	var count int64
	_ = h.DBRead.QueryRowContext(r.Context(), `SELECT count(*) FROM security_audit_events WHERE event_type='payment_webhook_invalid' AND created_at >= now() - interval '24 hours'`).Scan(&count)
	return count
}

func (h *Handler) latestPaymentEvents(r *http.Request) []map[string]any {
	rows, err := h.DBRead.QueryContext(r.Context(), `SELECT provider, COALESCE(provider_order_id,''), COALESCE(provider_payment_id,''), status, currency, amount_try_cents, created_at FROM orders WHERE provider='paddle' ORDER BY created_at DESC LIMIT 10`)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var provider, orderID, paymentID, status, currency string
		var amount int64
		var createdAt time.Time
		if err := rows.Scan(&provider, &orderID, &paymentID, &status, &currency, &amount, &createdAt); err != nil {
			continue
		}
		items = append(items, map[string]any{"provider": provider, "provider_order_id": orderID, "provider_payment_id": paymentID, "status": status, "currency": currency, "amount_cents": amount, "created_at": createdAt})
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
