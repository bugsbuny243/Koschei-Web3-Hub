package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

type aiImageGenerateRequest struct {
	Prompt string `json:"prompt"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type togetherImageGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	N      int    `json:"n"`
}

type togetherImageGenerateResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

func (h *Handler) AIImageGenerate(w http.ResponseWriter, r *http.Request) {
	var req aiImageGenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt_required"})
		return
	}
	if req.Width <= 0 {
		req.Width = 1024
	}
	if req.Height <= 0 {
		req.Height = 1024
	}

	model := os.Getenv("TOGETHER_MODEL_IMAGE")
	if model == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "image_model_not_configured"})
		return
	}

	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ai_provider_not_configured"})
		return
	}

	payload := togetherImageGenerateRequest{
		Model:  model,
		Prompt: req.Prompt,
		Width:  req.Width,
		Height: req.Height,
		N:      1,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "marshal_failed"})
		return
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.together.xyz/v1/images/generations", bytes.NewReader(body))
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_error"})
		return
	}

	var providerResp togetherImageGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&providerResp); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_invalid_response"})
		return
	}
	if len(providerResp.Data) == 0 {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_empty_response"})
		return
	}

	if providerResp.Data[0].URL != "" {
		writeJSON(w, http.StatusOK, map[string]string{"url": providerResp.Data[0].URL, "model": model})
		return
	}
	if providerResp.Data[0].B64JSON != "" {
		writeJSON(w, http.StatusOK, map[string]string{"b64_json": providerResp.Data[0].B64JSON, "model": model})
		return
	}

	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_invalid_response"})
}
