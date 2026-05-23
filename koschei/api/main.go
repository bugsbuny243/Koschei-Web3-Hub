package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"koschei/api/internal/db"
	apihttp "koschei/api/internal/http"
)

func main() {
	log.Printf("koschei api starting")
	log.Printf("migrations path: /app/migrations")
	log.Printf("static public path: /app/public")

	databaseURL := os.Getenv("DATABASE_URL")
	var conn *sql.DB
	var dbInitError string

	if databaseURL == "" {
		dbInitError = "DATABASE_URL is not set"
		log.Printf(dbInitError)
	} else {
		var err error
		conn, err = db.Connect(databaseURL)
		if err != nil {
			dbInitError = err.Error()
			log.Printf("database initialization failed: %v", err)
		} else {
			log.Printf("database connected")
		}
	}
	if conn != nil {
		defer conn.Close()
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if os.Getenv("JWT_SECRET") == "" {
		log.Printf("JWT_SECRET is not set")
	}
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "/app/public"
	}
	srv := apihttp.NewServer(conn, dbInitError, os.Getenv("ADMIN_PASSWORD"), os.Getenv("CORS_ALLOWED_ORIGIN"), staticDir)
	log.Printf("api listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		log.Fatal(err)
	}
}
