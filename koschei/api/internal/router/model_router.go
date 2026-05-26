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
	if strings.EqualFold(strings.TrimSpace(getEnv("CLAUDE_API_KEY")), "") == false {
		return "claude"
	}
	if strings.EqualFold(strings.TrimSpace(getEnv("LLAMA_API_KEY")), "") == false {
		return "llama"
	}
	return "llama"
}

func defaultModel(provider string) string {
	if provider == "claude" {
		return "claude-sonnet-4"
	}
	return "llama-4.1-70b"
}

var getEnv = os.Getenv
