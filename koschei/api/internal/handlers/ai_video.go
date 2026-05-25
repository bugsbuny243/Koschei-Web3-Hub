package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

func (h *Handler) AIVideoCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var in struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_json"})
		return
	}
	if in.Prompt == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "prompt_required"})
		return
	}

	model := os.Getenv("TOGETHER_MODEL_VIDEO")
	if model == "" {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "video_model_not_configured"})
		return
	}
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "ai_provider_not_configured"})
		return
	}

	body, _ := json.Marshal(map[string]string{"model": model, "prompt": in.Prompt})
	req, err := http.NewRequest(http.MethodPost, "https://api.together.xyz/v2/videos", bytes.NewReader(body))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "request_build_failed"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "provider_request_failed"})
		return
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "provider_invalid_response"})
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.WriteHeader(resp.StatusCode)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "provider_error"})
		return
	}

	jobID, _ := raw["id"].(string)
	if jobID == "" {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "provider_missing_job_id"})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"job_id": jobID, "status": "queued"})
}

func (h *Handler) AIVideoStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "id_required"})
		return
	}

	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "ai_provider_not_configured"})
		return
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.together.xyz/v2/videos/"+id, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "request_build_failed"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "provider_request_failed"})
		return
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "provider_invalid_response"})
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.WriteHeader(resp.StatusCode)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "provider_error"})
		return
	}

	status, _ := raw["status"].(string)
	videoURL := ""
	if v, ok := raw["output"].(string); ok {
		videoURL = v
	}
	if videoURL == "" {
		if v, ok := raw["url"].(string); ok {
			videoURL = v
		}
	}
	if videoURL == "" {
		if v, ok := raw["video_url"].(string); ok {
			videoURL = v
		}
	}
	if videoURL == "" {
		if d, ok := raw["data"].([]interface{}); ok && len(d) > 0 {
			if item, ok := d[0].(map[string]interface{}); ok {
				if v, ok := item["url"].(string); ok {
					videoURL = v
				}
			}
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"status": status, "video_url": videoURL})
}
