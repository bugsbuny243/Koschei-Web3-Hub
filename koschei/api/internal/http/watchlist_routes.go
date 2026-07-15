package http

import (
	"net/http"

	"koschei/api/internal/handlers"
)

func registerWatchlistRoutes(
	mux *http.ServeMux,
	h *handlers.Handler,
	koschTierAccess func(string, http.HandlerFunc) http.HandlerFunc,
	koschTier func(string, http.HandlerFunc) http.HandlerFunc,
) {
	mux.HandleFunc("/api/public/token/status", method(http.MethodGet, h.PublicTokenStatus))
	mux.HandleFunc("/api/public/token/readiness", method(http.MethodGet, h.PublicTokenLaunchReadiness))
	mux.HandleFunc("/api/public/scan-history", method(http.MethodGet, h.PublicScanHistory))
	mux.HandleFunc("/api/public/transaction-simulate", method(http.MethodPost, h.PublicTransactionSimulate))

	mux.HandleFunc("/api/auth/wallet/challenge", requiresDB(h, handlers.RequireAuth(method(http.MethodPost, h.CreateWalletChallenge))))
	mux.HandleFunc("/api/auth/wallet/verify", requiresDB(h, handlers.RequireAuth(method(http.MethodPost, h.VerifyWalletChallenge))))
	mux.HandleFunc("/api/auth/wallet/status", requiresDB(h, handlers.RequireAuth(method(http.MethodGet, h.WalletLinkStatus))))
	mux.HandleFunc("/api/auth/wallet/unlink", requiresDB(h, handlers.RequireAuth(method(http.MethodPost, h.UnlinkWallet))))
	mux.HandleFunc("/api/auth/token-access", requiresDB(h, handlers.RequireAuth(method(http.MethodGet, h.TokenAccessStatus))))
	mux.HandleFunc("/api/auth/premium-access", requiresDB(h, handlers.RequireAuth(method(http.MethodGet, h.PremiumAccessStatus))))

	// Pro eligibility protects persistent monitoring surfaces. Only an explicit
	// refresh consumes one daily scan quota; read and CRUD operations do not.
	mux.HandleFunc("/api/watchlist", requiresDB(h, koschTierAccess("pro", h.WatchlistCollection)))
	mux.HandleFunc("/api/watchlist/refresh", requiresDB(h, koschTier("pro", method(http.MethodPost, h.WatchlistRefresh))))
	mux.HandleFunc("/api/watchlist/alerts", requiresDB(h, koschTierAccess("pro", h.WatchlistAlerts)))
	mux.HandleFunc("/api/watchlist/", requiresDB(h, koschTierAccess("pro", h.WatchlistItem)))

	// Webhook management is an Enterprise capability. Delivery history and CRUD
	// do not consume scan quota; the scans that produce events are metered at the
	// product/developer route boundary.
	mux.HandleFunc("/api/webhooks/deliveries", requiresDB(h, koschTierAccess("enterprise", h.WebhookDeliveries)))
	mux.HandleFunc("/api/webhooks/deliveries/", requiresDB(h, koschTierAccess("enterprise", h.WebhookDeliveryItem)))
	mux.HandleFunc("/api/webhooks", requiresDB(h, koschTierAccess("enterprise", h.WebhookEndpoints)))
	mux.HandleFunc("/api/webhooks/", requiresDB(h, koschTierAccess("enterprise", h.WebhookEndpointItem)))
}
