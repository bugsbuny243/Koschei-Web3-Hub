package http

import (
	"net/http"

	"koschei/api/internal/handlers"
)

func registerDossierRoutes(mux *http.ServeMux, h *handlers.Handler) {
	mux.HandleFunc("/api/v1/dossier/", requiresDB(h, h.DossierAccess(method(http.MethodPost, h.DossierExport))))
	mux.HandleFunc("/dossier/", requiresDB(h, method(http.MethodGet, h.DossierPage)))
	mux.HandleFunc("/api/owner/arvis/acceptance", requiresDB(h, ownerOnly(h, method(http.MethodPost, h.OwnerInvestigationAcceptance))))
}
