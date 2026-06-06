package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"koschei/api/internal/db"
	apihttp "koschei/api/internal/http"
)

func main() {
	log.Printf("koschei api starting")
	log.Printf("migrations path: /app/migrations")

	databaseURL := os.Getenv("DATABASE_URL")
	var conn *sql.DB
	var dbInitError string

	if databaseURL == "" {
		dbInitError = "DATABASE_URL is not set"
		log.Printf("database unavailable: %s", dbInitError)
	} else {
		var err error
		conn, err = db.Connect(databaseURL)
		if err != nil {
			dbInitError = err.Error()
			log.Printf("database unavailable: %v", err)
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
	if os.Getenv("NEON_AUTH_JWKS_URL") == "" {
		log.Printf("NEON_AUTH_JWKS_URL is not set")
	}
	staticDir := resolveStaticDir(os.Getenv("STATIC_DIR"))
	log.Printf("static public path: %s", staticDir)
	srv := apihttp.NewServer(conn, dbInitError, os.Getenv("ADMIN_PASSWORD"), os.Getenv("CORS_ALLOWED_ORIGIN"), staticDir)
	log.Printf("api listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		log.Fatal(err)
	}
}

func resolveStaticDir(configured string) string {
	if configured != "" {
		return configured
	}
	for _, candidate := range []string{
		filepath.Join("koschei", "api", "public"),
		"public",
		filepath.Join("/app", "public"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join("koschei", "api", "public")
}
