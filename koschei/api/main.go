package main

import (
	"log"
	"net/http"
	"os"

	"github.com/bugsbuny243/Koschei-Web3-Hub/koschei/api/handlers"
)

func main() {
	// Statik dosyalar
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)

	// Public routes
	http.HandleFunc("/api/health", handlers.Health)
	http.HandleFunc("/api/auth/neon-callback", handlers.NeonCallback)
	http.HandleFunc("/api/auth/neon-login", handlers.NeonLogin)
	http.HandleFunc("/api/auth/neon-register", handlers.NeonRegister)

	// Protected routes (require auth)
	http.HandleFunc("/api/wallet/score", handlers.RequireAuth(handlers.WalletScore))
	http.HandleFunc("/api/tx/decode", handlers.RequireAuth(handlers.TxDecode))
	http.HandleFunc("/api/token/scan", handlers.RequireAuth(handlers.TokenScan))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Koschei API running on port " + port)
	log.Fatal(http.ListenAndServe(":" + port, nil))
}