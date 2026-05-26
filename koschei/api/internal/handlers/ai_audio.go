package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

type aiAudioGenerateRequest struct {
	Input string `json:"input"`
	Voice string `json:"voice"`
}

type togetherAudioGenerateRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	ResponseFormat string `json:"response_format"`
}

func (h *Handler) AIAudioGenerate(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("ENABLE_MEDIA_MODULES") != "true" {
		writeJSON(w, http.StatusGone, map[string]any{
			"error":           "feature_paused",
			"detail":          "Media Factory is paused. Koschei is focused on Runtime Factory, Artifact Forge, and Owner God Mode.",
			"credits_charged": false,
		})
		return
	}

	var req aiAudioGenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if req.Input == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "input_required"})
		return
	}
	if req.Voice == "" {
		req.Voice = "tara"
	}

	model := os.Getenv("TOGETHER_MODEL_TTS")
	if model == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "tts_model_not_configured"})
		return
	}

	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ai_provider_not_configured"})
		return
	}

	payload := togetherAudioGenerateRequest{
		Model:          model,
		Input:          req.Input,
		Voice:          req.Voice,
		ResponseFormat: "mp3",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "marshal_failed"})
		return
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.together.xyz/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "request_build_failed"})
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_request_failed"})
		return
	}
	defer resp.Body.Close()

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_invalid_response"})
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"audio_base64": base64.StdEncoding.EncodeToString(audioBytes),
		"format":       "mp3",
		"model":        model,
	})
}
