package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"koschei/api/internal/handlers"
)

var entitlementOnlyAPIPaths = map[string]struct{}{
	"/api/polar/checkout": {},
	"/api/polar/webhook":  {},
}

func init() {
	// These paths are optional only with respect to the application database.
	// Their route-local RequireEntitlementStore gate still fails closed when the
	// commercial authorization ledger is unavailable.
	for path := range entitlementOnlyAPIPaths {
		databaseOptionalAPIPaths[path] = struct{}{}
	}
}

func requiresEntitlementStore(h *handlers.Handler, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || !h.RequireEntitlementStore(w) {
			return
		}
		next(w, r)
	}
}

// apiReadinessWithEntitlement preserves the legacy application-persistence
// readiness contract while allowing narrowly scoped commercial authorization
// surfaces to depend on a separate entitlement ledger. It is available for the
// later migration of legacy paid routes without weakening the existing wrapper.
func apiReadinessWithEntitlement(db *sql.DB, entitlementDB *sql.DB, next http.Handler) http.Handler {
	setSecurityAuditDB(db)
	protected := bodyLimit(sensitiveRateLimit(db, next))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if requiresProfessionalLegacyOperation(path) {
			if entitlementDB == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "entitlement store unavailable"})
				return
			}
			access := &handlers.Handler{EntitlementDB: entitlementDB}
			target := func(gw http.ResponseWriter, gr *http.Request) { protected.ServeHTTP(gw, gr) }
			handlers.RequireAuth(access.RequirePlanTier("professional", access.EnforcePlanOutput(target)))(w, r)
			return
		}

		if _, ok := entitlementOnlyAPIPaths[path]; ok {
			protected.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(path, "/api/") && !allowedWithoutDatabase(path) && db == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "database unavailable"})
			return
		}
		protected.ServeHTTP(w, r)
	})
}
