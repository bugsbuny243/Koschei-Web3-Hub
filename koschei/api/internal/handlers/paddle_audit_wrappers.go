package handlers

import (
	"net/http"

	"koschei/api/internal/services"
)

func (h *Handler) CreateCheckoutAudited(w http.ResponseWriter, r *http.Request) {
	cfg := services.LoadPaddleConfigFromEnv()
	if !cfg.Enabled {
		writePaymentAudit(r, h, "payment_config_missing", "warning", map[string]any{"paddle": cfg.PublicStatus()})
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "paddle_not_configured"})
		return
	}
	h.CreateCheckout(w, r)
	writePaymentAudit(r, h, "checkout_created", "info", map[string]any{"environment": cfg.Environment})
}

func (h *Handler) HandleWebhookAudited(w http.ResponseWriter, r *http.Request) {
	writePaymentAudit(r, h, "payment_webhook_received", "info", map[string]any{"provider": "paddle"})
	if !h.ValidateWebhookSignature(r) {
		writePaymentAudit(r, h, "payment_webhook_invalid", "warning", map[string]any{"provider": "paddle", "reason": "signature_invalid"})
		writeAPIError(w, http.StatusUnauthorized, "WEBHOOK_INVALID", "Invalid webhook")
		return
	}
	h.HandleWebhook(w, r)
	writePaymentAudit(r, h, "payment_webhook_processed", "info", map[string]any{"provider": "paddle"})
}
