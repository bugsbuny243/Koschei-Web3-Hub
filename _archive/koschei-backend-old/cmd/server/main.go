package main

import (
	"log"

	"koschei/backend/internal/config"
	"koschei/backend/internal/db"
	"koschei/backend/internal/handlers"
	"koschei/backend/internal/router"
	"koschei/backend/internal/services"
)

func main() {
	cfg := config.Load()
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.RunMigrations(database); err != nil {
		log.Fatal(err)
	}

	h := handlers.Handler{
		DB:              database,
		JWTSecret:       cfg.JWTSecret,
		Router:          services.AIRouter{Cfg: cfg},
		Together:        services.TogetherClient{APIKey: cfg.TogetherAPIKey},
		Worker:          services.PythonWorkerClient{BaseURL: cfg.PythonWorkerURL},
		OwnerGodModeKey: cfg.OwnerGodModeKey,
	}
	r := router.New(h)
	log.Fatal(r.Run(":" + cfg.Port))
}
