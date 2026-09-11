package http

import (
	"net/http"

	"koschei/api/internal/handlers"
)

func registerMetadataRoutes(mux *http.ServeMux, h *handlers.Handler) {
	// GenerateMetadata performs its own atomic entitlement output decrement in the
	// same transaction as the saved output. Gate plan access here without wrapping
	// EnforcePlanOutput, otherwise one request would be charged twice.
	mux.HandleFunc("/api/metadata/generate", requiresDB(h, handlers.RequireAuth(h.RequirePlanTier("professional", method(http.MethodPost, h.GenerateMetadata)))))
}
