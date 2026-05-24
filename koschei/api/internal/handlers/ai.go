package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type aiGenerateRequest struct {
	Tool   string `json:"tool"`
	Prompt string `json:"prompt"`
}

type aiRoute struct {
	Route string
	Model string
}

func (h *Handler) AIGenerate(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req aiGenerateRequest
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Tool) == "" || strings.TrimSpace(req.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	route, err := resolveAIRoute(tool)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_tool"})
		return
	}

	if strings.ToLower(strings.TrimSpace(os.Getenv("TOGETHER_AI_ENABLED"))) != "true" || strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ai_provider_not_configured"})
		return
	}

	isPrivileged, credits, err := h.userCreditsAndRole(claims.Sub)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if !isPrivileged && credits <= 0 {
		writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": "insufficient_credits"})
		return
	}

	log.Printf("ai generation requested: email=%s tool=%s", claims.Email, tool)

	jobID, err := h.insertGenerationJob(claims.Email, tool, req.Prompt, route.Route)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	if !isTextTool(tool) {
		_ = h.finishGenerationJob(jobID, "failed", "tool_not_implemented_yet")
		_ = h.insertModelRouteLog(claims.Email, tool, route.Route, req.Prompt, "failed")
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "tool_not_implemented_yet", "tool": tool})
		return
	}

	resultText, callErr := h.callTogetherChat(route.Model, req.Prompt)
	if callErr != nil {
		log.Printf("ai generation failed: email=%s tool=%s err=%v", claims.Email, tool, callErr)
		_ = h.finishGenerationJob(jobID, "failed", shortError(callErr.Error()))
		_ = h.insertModelRouteLog(claims.Email, tool, route.Route, req.Prompt, "failed")
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "generation_failed"})
		return
	}

	if err := h.finishGenerationJob(jobID, "completed", resultText); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if !isPrivileged {
		if err := h.applyCreditCharge(claims.Sub, claims.Email); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	_ = h.insertModelRouteLog(claims.Email, tool, route.Route, req.Prompt, "completed")

	log.Printf("ai generation completed: email=%s tool=%s model=%s", claims.Email, tool, route.Model)
	writeJSON(w, http.StatusOK, map[string]any{
		"job_id":   jobID,
		"provider": "together",
		"route":    route.Route,
		"model":    route.Model,
		"status":   "completed",
		"result":   resultText,
	})
}

func resolveAIRoute(tool string) (aiRoute, error) {
	switch tool {
	case "chat":
		return aiRoute{Route: "chat", Model: firstEnv("TOGETHER_MODEL")}, nil
	case "code":
		return aiRoute{Route: "code", Model: firstEnv("TOGETHER_MODEL_COMPLEX", "TOGETHER_MODEL")}, nil
	case "reason":
		return aiRoute{Route: "reason", Model: firstEnv("TOGETHER_MODEL_REASONING", "TOGETHER_MODEL_COMPLEX", "TOGETHER_MODEL")}, nil
	case "image":
		return aiRoute{Route: "image", Model: firstEnv("TOGETHER_MODEL_IMAGE")}, nil
	case "image_edit":
		return aiRoute{Route: "image_edit", Model: firstEnv("TOGETHER_MODEL_IMAGE_EDIT")}, nil
	case "video":
		return aiRoute{Route: "video", Model: firstEnv("TOGETHER_MODEL_VIDEO")}, nil
	case "video_cinema":
		return aiRoute{Route: "video_cinema", Model: firstEnv("TOGETHER_MODEL_VIDEO_CINEMA")}, nil
	case "tts":
		return aiRoute{Route: "tts", Model: firstEnv("TOGETHER_MODEL_TTS")}, nil
	case "stt":
		return aiRoute{Route: "stt", Model: firstEnv("TOGETHER_MODEL_STT")}, nil
	default:
		return aiRoute{}, errors.New("unsupported tool")
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func isTextTool(tool string) bool { return tool == "chat" || tool == "code" || tool == "reason" }

func (h *Handler) userCreditsAndRole(authSub string) (bool, int, error) {
	var role string
	var credits int
	if err := h.DB.QueryRow(`SELECT role, credits FROM app_user_profiles WHERE auth_subject=$1`, authSub).Scan(&role, &credits); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, nil
		}
		return false, 0, err
	}
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "owner" || role == "admin", credits, nil
}

func (h *Handler) insertGenerationJob(email, tool, prompt, route string) (string, error) {
	var id string
	err := h.DB.QueryRow(`INSERT INTO generation_jobs (email, tool, prompt, status, provider, route, result) VALUES ($1,$2,$3,'running','together',$4,NULL) RETURNING id`, email, tool, prompt, route).Scan(&id)
	return id, err
}

func (h *Handler) finishGenerationJob(id, status, result string) error {
	_, err := h.DB.Exec(`UPDATE generation_jobs SET status=$2, result=$3, updated_at=now() WHERE id=$1`, id, status, result)
	return err
}

func (h *Handler) insertModelRouteLog(email, tool, route, prompt, status string) error {
	_, err := h.DB.Exec(`INSERT INTO model_route_logs (email, tool, route, provider, prompt, status) VALUES ($1,$2,$3,'together',$4,$5)`, email, tool, route, prompt, status)
	return err
}

func (h *Handler) applyCreditCharge(authSub, email string) error {
	res, err := h.DB.Exec(`UPDATE app_user_profiles SET credits=credits-1, updated_at=now() WHERE auth_subject=$1 AND credits>0`, authSub)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("insufficient credits")
	}
	_, err = h.DB.Exec(`INSERT INTO credit_events (email, amount, reason, created_at) VALUES ($1,-1,'ai_generation',now())`, email)
	return err
}

func (h *Handler) callTogetherChat(model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		return "", errors.New("together model is empty")
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are Koschei AI, a production-focused AI command center assistant.\nDefault language: Turkish.\nIf the user writes Turkish, Turkish slang, Turkish typos, or mixed Turkish-English, respond in Turkish.\nDo not identify, classify, or explain the user's language unless the user explicitly asks.\nTreat casual words like \"kanka\", \"lan\", \"aga\", \"olm\", \"reis\", \"naber\", \"nbr\", and typo-like short messages as Turkish conversation.\nBe direct, practical, and helpful.\nFor code, produce clean production-ready output.\nFor business or product planning, give actionable steps.\nFor unclear short messages, ask a short Turkish clarification instead of guessing another language."},
			{"role": "user", "content": "User message follows. Answer in Turkish unless the user explicitly requests another language.\n\n" + prompt},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.together.xyz/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("together status %d", resp.StatusCode)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errors.New("empty ai response")
	}
	return parsed.Choices[0].Message.Content, nil
}

func shortError(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 240 {
		return v[:240]
	}
	return v
}
