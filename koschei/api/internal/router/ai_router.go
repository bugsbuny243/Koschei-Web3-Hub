package router

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

type ChatRequest struct {
	System      string
	Prompt      string
	Model       string
	MaxTokens   int
	Temperature float64
	Timeout     time.Duration
}

type ChatResponse struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Content  string `json:"content"`
}

type chatCompletionPayload struct {
	Model       string              `json:"model"`
	Messages    []map[string]string `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature"`
}

// Chat routes every server-side ARVIS/LLM request through one policy layer.
// Koschei uses the configured Together model stack only.
func Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return ChatResponse{}, errors.New("prompt is required")
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 800
	}
	if req.Timeout <= 0 {
		req.Timeout = 20 * time.Second
	}
	if req.Temperature == 0 {
		req.Temperature = 0.2
	}
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	if strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) == "" {
		return ChatResponse{}, errors.New("TOGETHER_API_KEY is not configured")
	}
	return callTogether(ctx, req)
}

func callTogether(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" || strings.HasPrefix(strings.ToLower(model), "gpt-") {
		model = firstEnv("TOGETHER_MODEL_SECURITY", "TOGETHER_MODEL_ARVIS", "TOGETHER_MODEL", "TOGETHER_MODEL_CHAT")
	}
	if model == "" {
		model = "Qwen/Qwen3-235B-A22B-2507"
	}
	payload := chatCompletionPayload{Model: model, Messages: messages(req.System, req.Prompt), MaxTokens: req.MaxTokens, Temperature: req.Temperature}
	content, err := postChat(ctx, "https://api.together.xyz/v1/chat/completions", os.Getenv("TOGETHER_API_KEY"), payload)
	if err != nil {
		return ChatResponse{}, err
	}
	return ChatResponse{Provider: "together", Model: model, Content: content}, nil
}

func messages(system, prompt string) []map[string]string {
	out := []map[string]string{}
	if strings.TrimSpace(system) != "" {
		out = append(out, map[string]string{"role": "system", "content": system})
	}
	out = append(out, map[string]string{"role": "user", "content": prompt})
	return out
}

func postChat(ctx context.Context, endpoint, apiKey string, payload chatCompletionPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("provider returned %d", resp.StatusCode)
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("provider returned no choices")
	}
	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(decoded.Choices[0].Text)
	}
	if content == "" {
		return "", errors.New("provider returned empty content")
	}
	return content, nil
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}
