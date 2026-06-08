package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	// Statik dosyalar için
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)

	// API Endpointleri
	http.HandleFunc("/api/analyze", handleAnalyze)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Koschei Hub çalışıyor: http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
