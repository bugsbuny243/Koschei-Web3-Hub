package http

import (
	"net/http"

	"koschei/api/internal/handlers"
)

func registerWatchlistRoutes(
	mux *http.ServeMux,
	h *handlers.Handler,
	koschTier func(string, http.HandlerFunc) http.HandlerFunc,
	koschTierNoQuota func(string, http.HandlerFunc) http.HandlerFunc,
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

	// Continuous watch/alert activity is a Pro surface and consumes daily quota.
	mux.HandleFunc("/api/watchlist", requiresDB(h, koschTier("pro", h.WatchlistCollection)))
	mux.HandleFunc("/api/watchlist/refresh", requiresDB(h, koschTier("pro", method(http.MethodPost, h.WatchlistRefresh))))
	mux.HandleFunc("/api/watchlist/alerts", requiresDB(h, koschTier("pro", h.WatchlistAlerts)))
	mux.HandleFunc("/api/watchlist/", requiresDB(h, koschTier("pro", h.WatchlistItem)))

	// Webhook management is Enterprise eligibility but does not consume a scan
	// unit until an actual metered scan/shield request is made.
	mux.HandleFunc("/api/webhooks/deliveries", requiresDB(h, koschTierNoQuota("enterprise", h.WebhookDeliveries)))
	mux.HandleFunc("/api/webhooks/deliveries/", requiresDB(h, koschTierNoQuota("enterprise", h.WebhookDeliveryItem)))
	mux.HandleFunc("/api/webhooks", requiresDB(h, koschTierNoQuota("enterprise", h.WebhookEndpoints)))
	mux.HandleFunc("/api/webhooks/", requiresDB(h, koschTierNoQuota("enterprise", h.WebhookEndpointItem)))
}
