package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"
)

func (h *Handler) entitlementAvailable(ctx context.Context) error {
	store := h.entitlementStore()
	if store == nil {
		return errors.New("entitlement store unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return store.PingContext(ctx)
}

// RequireEntitlementStore gates only commercial authorization/quota surfaces.
// It must never be used as evidence that application persistence is available.
func (h *Handler) RequireEntitlementStore(w http.ResponseWriter) bool {
	if err := h.entitlementAvailable(context.Background()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "entitlement_store_unavailable"})
		return false
	}
	return true
}
