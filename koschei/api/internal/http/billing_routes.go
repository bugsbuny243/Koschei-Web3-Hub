package http

import (
	"net/http"

	"koschei/api/internal/handlers"
)

// Billing-provider routes are intentionally narrow. Checkout requires a
// customer session and passes through a fail-closed commercial-readiness gate;
// webhook access is public at the HTTP edge but authorizes no product access
// until its Polar signature and server-side binding evidence are verified by
// the handler. Commercial state uses the entitlement ledger, not application
// investigation persistence.
func registerBillingRoutes(mux *http.ServeMux, h *handlers.Handler) {
	billing := h
	if h != nil && h.EntitlementDB != nil && h.DB != h.EntitlementDB {
		clone := *h
		clone.DB = h.EntitlementDB
		clone.DBRead = h.EntitlementDB
		billing = &clone
	}
	mux.HandleFunc("/api/polar/checkout", requiresEntitlementStore(h, handlers.RequireAuth(method(http.MethodPost, billing.PolarCheckoutCommercial))))
	mux.HandleFunc("/api/polar/webhook", requiresEntitlementStore(h, method(http.MethodPost, billing.PolarWebhook)))
}
