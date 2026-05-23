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
	databaseURL := os.Getenv("DATABASE_URL")
	var conn = (*sql.DB)(nil)
	if databaseURL == "" {
		log.Printf("warning: DATABASE_URL is not set; database-backed API routes will return 503")
	} else {
		var err error
		conn, err = db.Connect(databaseURL)
		if err != nil {
			log.Printf("warning: database connection failed: %v", err)
		}
	}
	if conn != nil {
		defer conn.Close()
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	staticDir := os.Getenv("STATIC_DIR")
	srv := apihttp.NewServer(conn, os.Getenv("ADMIN_PASSWORD"), os.Getenv("CORS_ALLOWED_ORIGIN"), staticDir)
	log.Printf("api listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		log.Fatal(err)
	}
}
