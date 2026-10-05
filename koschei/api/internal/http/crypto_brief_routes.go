package http

import (
	"net/http"

	"koschei/api/internal/cryptobrief"
	"koschei/api/internal/handlers"
)

// registerCryptoBriefRoutes keeps the historical registration function name so
// the server wiring stays stable while the old news product is retired. The
// registered surface is now ARVIS Telegram result delivery only.
func registerCryptoBriefRoutes(mux *http.ServeMux, h *handlers.Handler, _ string) {
	service := cryptobrief.New(h.DB)
	mux.HandleFunc("/api/customer/arvis/telegram", requiresDB(h, handlers.RequireAuth(service.ARVISTelegramHTTP(handlers.AuthenticatedSubject))))
	mux.HandleFunc("/api/customer/arvis/telegram/pair", requiresDB(h, handlers.RequireAuth(method("POST", service.ARVISTelegramPairHTTP(handlers.AuthenticatedSubject)))))
	mux.HandleFunc("/api/owner/arvis/telegram", requiresDB(h, ownerOnly(h, method("GET", service.ARVISTelegramStatusHTTP))))
	mux.HandleFunc("/integrations/arvis/telegram", requiresDB(h, service.ARVISTelegramWebhookHTTP))

	// One-deploy compatibility path for Telegram while setWebhook moves from the
	// retired Crypto Brief URL to the canonical ARVIS URL.
	mux.HandleFunc("/integrations/crypto-brief/telegram", requiresDB(h, service.ARVISTelegramWebhookHTTP))
}
