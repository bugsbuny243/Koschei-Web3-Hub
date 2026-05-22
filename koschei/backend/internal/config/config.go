package config

import (
	"log"
	"os"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	TogetherAPIKey         string
	TogetherModelCode      string
	TogetherModelChat      string
	TogetherModelReasoning string
	TogetherModelImage     string
	TogetherModelImageEdit string
	TogetherModelVideo     string
	TogetherModelVideoCine string
	TogetherModelTTS       string
	TogetherModelSTT       string
	CloudinaryName         string
	CloudinaryKey          string
	CloudinarySecret       string
	JWTSecret              string
	PythonWorkerURL        string
	OwnerGodModeKey        string
}

func Load() Config {
	cfg := Config{
		Port:                   getEnv("PORT", "8080"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		TogetherAPIKey:         os.Getenv("TOGETHER_API_KEY"),
		TogetherModelCode:      getEnv("TOGETHER_MODEL_CODE", "Qwen/Qwen3-Coder-480B-A35B-Instruct"),
		TogetherModelChat:      getEnv("TOGETHER_MODEL_CHAT", "meta-llama/Llama-3.3-70B-Instruct-Turbo"),
		TogetherModelReasoning: getEnv("TOGETHER_MODEL_REASONING", "deepseek-ai/DeepSeek-V4-Pro"),
		TogetherModelImage:     getEnv("TOGETHER_MODEL_IMAGE", "black-forest-labs/FLUX.2-pro"),
		TogetherModelImageEdit: getEnv("TOGETHER_MODEL_IMAGE_EDIT", "black-forest-labs/FLUX.1-kontext-pro"),
		TogetherModelVideo:     getEnv("TOGETHER_MODEL_VIDEO", "google/veo-3.0"),
		TogetherModelVideoCine: getEnv("TOGETHER_MODEL_VIDEO_CINEMA", "kling/kling-2.1-pro"),
		TogetherModelTTS:       getEnv("TOGETHER_MODEL_TTS", "hexgrad/Kokoro-82M"),
		TogetherModelSTT:       getEnv("TOGETHER_MODEL_STT", "openai/whisper-large-v3"),
		CloudinaryName:         os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudinaryKey:          os.Getenv("CLOUDINARY_API_KEY"),
		CloudinarySecret:       os.Getenv("CLOUDINARY_API_SECRET"),
		JWTSecret:              getEnv("JWT_SECRET", "dev-secret-change-me"),
		PythonWorkerURL:        getEnv("PYTHON_WORKER_URL", "http://localhost:8001"),
		OwnerGodModeKey:        os.Getenv("OWNER_GOD_MODE_KEY"),
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
