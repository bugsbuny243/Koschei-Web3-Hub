package http

import (
	"koschei/api/internal/cryptobrief"
	"koschei/api/internal/handlers"
	"net/http"
	"path/filepath"
)

func registerCryptoBriefRoutes(mux *http.ServeMux, h *handlers.Handler, staticDir string) {
	service := cryptobrief.New(h.DB)
	mux.HandleFunc("/api/crypto-brief/feed", requiresDB(h, method("GET", service.FeedHTTP)))
	mux.HandleFunc("/api/customer/crypto-brief", requiresDB(h, handlers.RequireAuth(service.CustomerHTTP(handlers.AuthenticatedSubject))))
	mux.HandleFunc("/api/customer/crypto-brief/pair", requiresDB(h, handlers.RequireAuth(method("POST", service.PairHTTP(handlers.AuthenticatedSubject)))))
	mux.HandleFunc("/api/owner/crypto-brief", requiresDB(h, ownerOnly(h, method("GET", service.StatusHTTP))))
	mux.HandleFunc("/owner/crypto-brief", ownerOnly(h, method("GET", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, filepath.Join(staticDir, "crypto-brief-operations.html"))
	})))
	mux.HandleFunc("/integrations/crypto-brief/telegram", requiresDB(h, service.TelegramHTTP))
	mux.HandleFunc("/api/customer/arvis/telegram", requiresDB(h, handlers.RequireAuth(service.ARVISTelegramHTTP(handlers.AuthenticatedSubject))))
	mux.HandleFunc("/api/customer/arvis/telegram/pair", requiresDB(h, handlers.RequireAuth(service.ARVISTelegramPairHTTP(handlers.AuthenticatedSubject))))
	mux.HandleFunc("/api/owner/arvis/telegram", requiresDB(h, ownerOnly(h, method("GET", service.ARVISTelegramStatusHTTP))))
	mux.HandleFunc("/integrations/arvis/telegram", requiresDB(h, service.ARVISTelegramWebhookHTTP))
}
