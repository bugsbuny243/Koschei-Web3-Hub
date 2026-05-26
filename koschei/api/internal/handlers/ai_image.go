package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

const imageCreditCost = 10

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
	if os.Getenv("ENABLE_MEDIA_MODULES") != "true" {
		writeJSON(w, http.StatusGone, map[string]any{
			"error":           "feature_paused",
			"detail":          "Media Factory is paused. Koschei is focused on Runtime Factory, Artifact Forge, and Owner God Mode.",
			"credits_charged": false,
		})
		return
	}

	claims, ok := userFromContext(r.Context())
	if !ok || claims.Sub == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

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

	if _, err := h.DB.ExecContext(
		r.Context(),
		`INSERT INTO app_user_profiles (auth_subject, email)
		 VALUES ($1, $2)
		 ON CONFLICT (auth_subject) DO NOTHING`,
		claims.Sub,
		claims.Email,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile_upsert_failed"})
		return
	}

	var creditsLeft int
	err := h.DB.QueryRowContext(
		r.Context(),
		`UPDATE app_user_profiles
		 SET credits = credits - $1
		 WHERE auth_subject = $2 AND credits >= $1
		 RETURNING credits`,
		imageCreditCost,
		claims.Sub,
	).Scan(&creditsLeft)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": "insufficient_credits"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "credit_deduct_failed"})
		return
	}

	model := os.Getenv("TOGETHER_MODEL_IMAGE")
	if model == "" {
		_, _ = h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits = credits + $1 WHERE auth_subject = $2`, imageCreditCost, claims.Sub)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "image_model_not_configured"})
		return
	}

	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		_, _ = h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits = credits + $1 WHERE auth_subject = $2`, imageCreditCost, claims.Sub)
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
		_, _ = h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits = credits + $1 WHERE auth_subject = $2`, imageCreditCost, claims.Sub)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "marshal_failed"})
		return
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.together.xyz/v1/images/generations", bytes.NewReader(body))
	if err != nil {
		_, _ = h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits = credits + $1 WHERE auth_subject = $2`, imageCreditCost, claims.Sub)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "request_build_failed"})
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		_, _ = h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits = credits + $1 WHERE auth_subject = $2`, imageCreditCost, claims.Sub)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_request_failed"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits = credits + $1 WHERE auth_subject = $2`, imageCreditCost, claims.Sub)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_error"})
		return
	}

	var providerResp togetherImageGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&providerResp); err != nil {
		_, _ = h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits = credits + $1 WHERE auth_subject = $2`, imageCreditCost, claims.Sub)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_invalid_response"})
		return
	}
	if len(providerResp.Data) == 0 {
		_, _ = h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits = credits + $1 WHERE auth_subject = $2`, imageCreditCost, claims.Sub)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_empty_response"})
		return
	}

	if providerResp.Data[0].URL != "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"url": providerResp.Data[0].URL, "credits_left": creditsLeft})
		return
	}
	if providerResp.Data[0].B64JSON != "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"b64_json": providerResp.Data[0].B64JSON, "credits_left": creditsLeft})
		return
	}

	_, _ = h.DB.ExecContext(r.Context(), `UPDATE app_user_profiles SET credits = credits + $1 WHERE auth_subject = $2`, imageCreditCost, claims.Sub)
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_invalid_response"})
}
