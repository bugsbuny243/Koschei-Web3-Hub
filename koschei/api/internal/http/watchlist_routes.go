package http

import (
	"net/http"

	"koschei/api/internal/handlers"
)

func registerWatchlistRoutes(mux *http.ServeMux, h *handlers.Handler, premium func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/api/watchlist", requiresDB(h, premium(h.WatchlistCollection)))
	mux.HandleFunc("/api/watchlist/refresh", requiresDB(h, premium(method(http.MethodPost, h.WatchlistRefresh))))
	mux.HandleFunc("/api/watchlist/alerts", requiresDB(h, premium(h.WatchlistAlerts)))
	mux.HandleFunc("/api/watchlist/", requiresDB(h, premium(h.WatchlistItem)))
}
