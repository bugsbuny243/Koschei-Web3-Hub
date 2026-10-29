package http

import (
	"koschei/api/internal/handlers"
	"net/http"
	"path/filepath"
)

func registerSOCOperationsRoutes(mux *http.ServeMux, h *handlers.Handler, staticDir string) {
	mux.HandleFunc("/api/owner/soc/queue", ownerOnly(h, method("GET", h.OwnerSOCQueue)))
	mux.HandleFunc("/api/owner/soc/case", ownerOnly(h, h.OwnerSOCCase))
	page := ownerOnly(h, method("GET", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, filepath.Join(staticDir, "owner-soc.html"))
	}))
	mux.HandleFunc("/owner/soc", page)
	// Protect the physical filename from the public FileServer as well.
	mux.HandleFunc("/owner-soc.html", page)
}
