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

const (
	commonAIInstruction = "Default language is Turkish. Understand Turkish slang and typos. Do not explain the user’s language unless asked. Be practical, production-focused, and direct. Use clean markdown headings."
	chatSystemPrompt    = "You are Koschei Chat Core. " + commonAIInstruction + " Answer short, friendly, Turkish-first. For casual messages like \"nbr lan kanka\", answer naturally in Turkish. Do not produce long architecture unless user asks."
	codeSystemPrompt    = "You are Koschei Code Engine. " + commonAIInstruction + " For technical build requests, always respond using exactly this structure:\n\n## Teknik Hedef\n\n## Mimari\n\n## Dosya Planı\n\n## API / Endpoint Planı\n\n## DB / Migration Planı\n\n## Uygulama Adımları\n\n## Örnek Kod\n\n## Test Planı\n\nRules:\n- Do not provide random standalone code without context.\n- Include code only when useful and specify target file path for each snippet.\n- Keep implementation details concrete (client/backend/db/worker/provider as needed).\n- If a section is not relevant, write 'Bu istek için gerekli değil.'"
	reasonSystemPrompt  = "You are Koschei Reason Matrix. " + commonAIInstruction + " For serious project/product/game/app requests, always respond using exactly this structure:\n\n## Ne İstiyorsun?\n\n## Gerçekçilik Analizi\n\n## MVP Sürüm\n\n## Gerekli Altyapı\n\n## Büyük Sürüm\n\n## Riskler\n\n## Üretim Sırası\n\n## Sonraki Net Adım\n\nRules:\n- Be honest but constructive.\n- Do not say 'impossible' for PUBG-like, GTA-like, TikTok-like, YouTube-like, marketplace-like, social app-like requests.\n- Do not promise instant full clone.\n- Clearly state it must be original and not a brand/IP clone.\n- Always provide MVP-first staged production plan and infrastructure details."
)

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
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_tool", "credits_charged": false})
		return
	}

	if strings.ToLower(strings.TrimSpace(os.Getenv("TOGETHER_AI_ENABLED"))) != "true" || strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "ai_provider_not_configured", "credits_charged": false})
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
		_ = h.insertModelRouteLog(claims.Email, tool, route.Route, route.Model, req.Prompt, "failed")
		writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "tool_not_implemented_yet", "tool": tool, "credits_charged": false})
		return
	}

	resultText, usedModel, callErr := h.generateWithRoute(tool, route, req.Prompt)
	if callErr != nil {
		log.Printf("ai generation failed: email=%s tool=%s err=%v", claims.Email, tool, callErr)
		_ = h.finishGenerationJob(jobID, "failed", shortError(callErr.Error()))
		_ = h.insertModelRouteLog(claims.Email, tool, route.Route, usedModel, req.Prompt, "failed")
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "generation_failed", "detail": shortError(callErr.Error()), "credits_charged": false})
		return
	}

	if err := h.completeGenerationAndCharge(jobID, claims.Sub, claims.Email, tool, route.Route, usedModel, req.Prompt, resultText, isPrivileged); err != nil {
		_ = h.finishGenerationJob(jobID, "failed", shortError(err.Error()))
		_ = h.insertModelRouteLog(claims.Email, tool, route.Route, usedModel, req.Prompt, "failed")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	log.Printf("ai generation completed: email=%s tool=%s model=%s", claims.Email, tool, usedModel)
	writeJSON(w, http.StatusOK, map[string]any{
		"job_id":          jobID,
		"provider":        "together",
		"route":           route.Route,
		"model":           usedModel,
		"status":          "completed",
		"credits_charged": true,
		"result":          resultText,
	})
}

func (h *Handler) generateWithRoute(tool string, route aiRoute, prompt string) (string, string, error) {
	resultText, err := h.callTogetherChat(route.Model, tool, prompt)
	if err == nil {
		return resultText, route.Model, nil
	}
	if tool != "reason" {
		return "", route.Model, err
	}
	fallbackModel := firstEnv("TOGETHER_MODEL_COMPLEX", "TOGETHER_MODEL")
	if strings.TrimSpace(fallbackModel) == "" || fallbackModel == route.Model {
		return "", route.Model, err
	}
	log.Printf("reason route fallback: primary model failed, retrying fallback model=%s", fallbackModel)
	resultText, fallbackErr := h.callTogetherChat(fallbackModel, "reason", prompt)
	if fallbackErr != nil {
		return "", fallbackModel, fmt.Errorf("primary: %v; fallback: %v", err, fallbackErr)
	}
	return resultText, fallbackModel, nil
}

func systemPromptForTool(tool string) string {
	switch tool {
	case "code":
		return codeSystemPrompt
	case "reason":
		return reasonSystemPrompt
	default:
		return chatSystemPrompt
	}
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

func (h *Handler) insertModelRouteLog(email, tool, route, model, prompt, status string) error {
	_, err := h.DB.Exec(`INSERT INTO model_route_logs (email, tool, route, model, provider, prompt, status) VALUES ($1,$2,$3,$4,'together',$5,$6)`, email, tool, route, model, prompt, status)
	return err
}

func (h *Handler) applyCreditChargeTx(tx *sql.Tx, authSub, email string) error {
	res, err := tx.Exec(`UPDATE app_user_profiles SET credits=credits-1, updated_at=now() WHERE auth_subject=$1 AND credits>0`, authSub)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("insufficient credits")
	}
	_, err = tx.Exec(`INSERT INTO credit_events (email, amount, reason, created_at) VALUES ($1,-1,'ai_generation',now())`, email)
	return err
}

func (h *Handler) completeGenerationAndCharge(jobID, authSub, email, tool, route, model, prompt, resultText string, isPrivileged bool) error {
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE generation_jobs SET status='completed', result=$2, updated_at=now() WHERE id=$1`, jobID, resultText); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO model_route_logs (email, tool, route, model, provider, prompt, status) VALUES ($1,$2,$3,$4,'together',$5,'completed')`, email, tool, route, model, prompt); err != nil {
		return err
	}
	if !isPrivileged {
		if err := h.applyCreditChargeTx(tx, authSub, email); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (h *Handler) callTogetherChat(model string, tool string, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		return "", errors.New("together model is empty")
	}
	systemPrompt := systemPromptForTool(tool)
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "User message follows. Answer in Turkish unless the user explicitly requests another language.\n\n" + prompt},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.together.xyz/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")))
	req.Header.Set("Content-Type", "application/json")

	timeout := 45 * time.Second
	if v := strings.TrimSpace(os.Getenv("TOGETHER_TIMEOUT_SECONDS")); v != "" {
		if parsed, parseErr := time.ParseDuration(v + "s"); parseErr == nil && parsed >= 5*time.Second {
			timeout = parsed
		}
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("together status %d: %s", resp.StatusCode, shortError(string(respBody)))
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

func (h *Handler) AIJobs(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	rows, err := h.DB.Query(`SELECT id, tool, prompt, route, provider, status, result, error, created_at, updated_at FROM generation_jobs WHERE email=$1 ORDER BY created_at DESC LIMIT 20`, claims.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()
	jobs := make([]map[string]any, 0, 20)
	for rows.Next() {
		var id, tool, prompt, route, provider, status string
		var result, jobError sql.NullString
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &tool, &prompt, &route, &provider, &status, &result, &jobError, &createdAt, &updatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		jobs = append(jobs, map[string]any{
			"id":         id,
			"tool":       tool,
			"prompt":     prompt,
			"route":      route,
			"provider":   provider,
			"status":     status,
			"result":     nullableString(result),
			"error":      nullableString(jobError),
			"created_at": createdAt,
			"updated_at": updatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func nullableString(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}

func shortError(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 240 {
		return v[:240]
	}
	return v
}
