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

	// API routes
	http.HandleFunc("/api/health", handlers.Health)
	http.HandleFunc("/api/wallet/score", handlers.WalletScore)
	http.HandleFunc("/api/tx/decode", handlers.TxDecode)
	http.HandleFunc("/api/token/scan", handlers.TokenScan)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Koschei API running on port " + port)
	log.Fatal(http.ListenAndServe(":" + port, nil))
}