package router

import (
	"os"
	"strings"
)

type ModelRoute struct {
	Route    string `json:"route"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

const (
	liveStatus = "live"
)

var toolRouteMap = map[string]string{
	"code-generator":  "code_generation",
	"debug":           "code_debug",
	"refactor":        "code_refactor",
	"analysis":        "chat_analysis",
	"strategy":        "deep_reasoning",
	"image-studio":    "image_generation",
	"image-edit":      "image_editing",
	"video-studio":    "video_generation",
	"cinematic-video": "cinematic_video",
	"voice-lab":       "text_to_speech",
	"speech-to-text":  "speech_to_text",
}

func ResolveModelRoute(tool string) ModelRoute {
	route := toolRouteMap[strings.TrimSpace(strings.ToLower(tool))]
	if route == "" {
		route = "chat_analysis"
	}
	provider := providerFromEnv()
	model := defaultModel(provider)
	return ModelRoute{Route: model, Provider: provider, Status: liveStatus, Message: "Live provider route selected."}
}

func providerFromEnv() string {
	if strings.TrimSpace(getEnv("TOGETHER_API_KEY")) != "" {
		return "together"
	}
	return "unconfigured"
}

func defaultModel(provider string) string {
	switch provider {
	case "together":
		return firstNonEmptyEnv("TOGETHER_MODEL", "TOGETHER_MODEL_CHAT", "meta-llama/Llama-3.3-70B-Instruct-Turbo")
	default:
		return "unconfigured"
	}
}

var getEnv = os.Getenv

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if strings.HasPrefix(key, "TOGETHER_") {
			if value := strings.TrimSpace(getEnv(key)); value != "" {
				return value
			}
			continue
		}
		if strings.TrimSpace(key) != "" {
			return key
		}
	}
	return ""
}
