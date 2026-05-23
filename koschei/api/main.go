package main

import (
	"log"
	"net/http"
	"os"

	"koschei/api/internal/db"
	apihttp "koschei/api/internal/http"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" { log.Fatal("DATABASE_URL is required") }

	conn, err := db.Connect(databaseURL)
	if err != nil { log.Fatal(err) }
	defer conn.Close()

	port := os.Getenv("PORT")
	if port == "" { port = "8080" }
	srv := apihttp.NewServer(conn, os.Getenv("ADMIN_PASSWORD"), os.Getenv("CORS_ALLOWED_ORIGIN"))
	log.Printf("api listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil { log.Fatal(err) }
}
