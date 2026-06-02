package http

import (
	"net/http"

	"koschei-bridge/services/auth-api/internal/handlers"
)

type Server struct{ mux *http.ServeMux }

func New() (*Server, error) {
	handler, err := handlers.New()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handlers.JSONMethod(http.MethodGet, handler.Health))
	mux.HandleFunc("/api/web3/health", handlers.JSONMethod(http.MethodGet, handler.Web3ChainHealth))
	mux.HandleFunc("/api/config", handlers.JSONMethod(http.MethodGet, handler.Web3Config))
	mux.HandleFunc("/api/auth/provision", handlers.JSONMethod(http.MethodPost, handler.Web3Provision))
	mux.HandleFunc("/auth/signup", handlers.JSONMethod(http.MethodPost, handler.Signup))
	mux.HandleFunc("/auth/login", handlers.JSONMethod(http.MethodPost, handler.Login))
	mux.HandleFunc("/auth/me", handlers.JSONMethod(http.MethodGet, handler.Me))
	mux.HandleFunc("/auth/logout", handlers.JSONMethod(http.MethodPost, handler.Logout))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		handlers.WriteError(w, http.StatusNotFound, "Not found.")
	})
	return &Server{mux: mux}, nil
}
func (server *Server) ListenAndServe(addr string) error { return http.ListenAndServe(addr, server.mux) }
