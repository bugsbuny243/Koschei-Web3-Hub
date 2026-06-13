package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type intelligenceSummary struct {
	Summary         string   `json:"summary"`
	Recommendations []string `json:"recommendations"`
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
}

func (h *Handler) GenerateIntelligenceSummary(ctx context.Context, email, input string, realData any) (intelligenceSummary, error) {
	prompt := fmt.Sprintf("Summarize this Koschei Web3 analysis using only the provided real data. If data is unavailable, say it is unavailable; do not invent facts. Return JSON with summary and recommendations array.\nInput: %s\nReal data JSON: %s", input, string(jsonBytes(realData)))
	providers := []struct{ name, key, model, url string }{
		{"openai", strings.TrimSpace(os.Getenv("OPENAI_API_KEY")), firstNonEmptyString(os.Getenv("OPENAI_MODEL"), "gpt-4o-mini"), "https://api.openai.com/v1/chat/completions"},
		{"together", strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")), firstNonEmptyString(os.Getenv("TOGETHER_MODEL"), "meta-llama/Llama-3.3-70B-Instruct-Turbo"), "https://api.together.xyz/v1/chat/completions"},
	}
	for _, p := range providers {
		if p.key == "" {
			continue
		}
		started := time.Now()
		out, err := callChatProvider(ctx, p.url, p.key, p.model, prompt)
		status := "ok"
		if err != nil {
			status = "error"
		}
		h.logModelRoute(email, p.name, p.model, status, time.Since(started).Milliseconds())
		if err != nil {
			continue
		}
		var parsed struct {
			Summary         string   `json:"summary"`
			Recommendations []string `json:"recommendations"`
		}
		if json.Unmarshal([]byte(stripMarkdownJSON(out)), &parsed) != nil || strings.TrimSpace(parsed.Summary) == "" {
			parsed.Summary = strings.TrimSpace(out)
		}
		if strings.TrimSpace(parsed.Summary) == "" {
			continue
		}
		return intelligenceSummary{Summary: parsed.Summary, Recommendations: parsed.Recommendations, Provider: p.name, Model: p.model}, nil
	}
	return intelligenceSummary{}, errors.New("ai provider unavailable")
}

func callChatProvider(ctx context.Context, url, key, model, prompt string) (string, error) {
	body, _ := json.Marshal(map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": prompt}}, "temperature": 0.2, "max_tokens": 500})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("provider status %d", resp.StatusCode)
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("empty provider response")
	}
	return decoded.Choices[0].Message.Content, nil
}

func stripMarkdownJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func (h *Handler) logModelRoute(email, provider, model, status string, latencyMS int64) {
	if h == nil || h.DB == nil {
		return
	}
	_, _ = h.DB.Exec(`INSERT INTO model_route_logs(email,tool,route,model,provider,prompt,status) VALUES(NULLIF($1,''),$2,$3,$4,$5,$6,$7)`, email, "unified_analyze", provider, model, provider, fmt.Sprintf("latency_ms=%d", latencyMS), status)
}
