package main

import (
	"log"
	"os"
	"strings"

	httpserver "koschei-bridge/services/auth-api/internal/http"
)

func main() {
	server, err := httpserver.New()
	if err != nil {
		log.Fatal(err)
	}
	addr := envOr("AUTH_API_ADDR", ":8080")
	log.Printf("auth-api listening on %s", addr)
	log.Fatal(server.ListenAndServe(addr))
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
