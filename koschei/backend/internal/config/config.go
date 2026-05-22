package config

import (
	"log"
	"os"
)

type Config struct {
	Port             string
	DatabaseURL      string
	TogetherAPIKey   string
	FalKey           string
	CloudinaryName   string
	CloudinaryKey    string
	CloudinarySecret string
	JWTSecret        string
}

func Load() Config {
	cfg := Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		TogetherAPIKey:   os.Getenv("TOGETHER_API_KEY"),
		FalKey:           os.Getenv("FAL_KEY"),
		CloudinaryName:   os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudinaryKey:    os.Getenv("CLOUDINARY_API_KEY"),
		CloudinarySecret: os.Getenv("CLOUDINARY_API_SECRET"),
		JWTSecret:        getEnv("JWT_SECRET", "dev-secret-change-me"),
	}

	if cfg.DatabaseURL == "" {
		log.Println("warning: DATABASE_URL is empty")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
