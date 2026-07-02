package http

import (
	"net/http"

	"koschei/api/internal/handlers"
)

func registerWatchlistRoutes(mux *http.ServeMux, h *handlers.Handler, premium func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/api/public/token/status", method(http.MethodGet, h.PublicTokenStatus))
	mux.HandleFunc("/api/public/token/readiness", method(http.MethodGet, h.PublicTokenLaunchReadiness))

	mux.HandleFunc("/api/auth/wallet/challenge", requiresDB(h, handlers.RequireAuth(method(http.MethodPost, h.CreateWalletChallenge))))
	mux.HandleFunc("/api/auth/wallet/verify", requiresDB(h, handlers.RequireAuth(method(http.MethodPost, h.VerifyWalletChallenge))))
	mux.HandleFunc("/api/auth/wallet/status", requiresDB(h, handlers.RequireAuth(method(http.MethodGet, h.WalletLinkStatus))))
	mux.HandleFunc("/api/auth/wallet/unlink", requiresDB(h, handlers.RequireAuth(method(http.MethodPost, h.UnlinkWallet))))
	mux.HandleFunc("/api/auth/token-access", requiresDB(h, handlers.RequireAuth(method(http.MethodGet, h.TokenAccessStatus))))
	mux.HandleFunc("/api/auth/premium-access", requiresDB(h, handlers.RequireAuth(method(http.MethodGet, h.PremiumAccessStatus))))

	mux.HandleFunc("/api/watchlist", requiresDB(h, premium(h.WatchlistCollection)))
	mux.HandleFunc("/api/watchlist/refresh", requiresDB(h, premium(method(http.MethodPost, h.WatchlistRefresh))))
	mux.HandleFunc("/api/watchlist/alerts", requiresDB(h, premium(h.WatchlistAlerts)))
	mux.HandleFunc("/api/watchlist/", requiresDB(h, premium(h.WatchlistItem)))

	mux.HandleFunc("/api/webhooks/deliveries", requiresDB(h, premium(h.WebhookDeliveries)))
	mux.HandleFunc("/api/webhooks/deliveries/", requiresDB(h, premium(h.WebhookDeliveryItem)))
	mux.HandleFunc("/api/webhooks", requiresDB(h, premium(h.WebhookEndpoints)))
	mux.HandleFunc("/api/webhooks/", requiresDB(h, premium(h.WebhookEndpointItem)))
}
