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

// Chat routes every server-side LLM request through a single policy. Together is
// the default primary provider for Koschei security intelligence, so Qwen and
// other open-weight models can carry daily traffic. Set AI_PROVIDER=openai to
// force OpenAI-first, or AI_PROVIDER=auto/together to keep Together-first with
// OpenAI as fallback.
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

	var errs []string
	for _, provider := range providerOrder() {
		switch provider {
		case "together":
			if strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) == "" {
				continue
			}
			resp, err := callTogether(ctx, req)
			if err == nil {
				return resp, nil
			}
			errs = append(errs, "together: "+err.Error())
		case "openai":
			if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
				continue
			}
			resp, err := callOpenAI(ctx, req)
			if err == nil {
				return resp, nil
			}
			errs = append(errs, "openai: "+err.Error())
		}
	}
	if len(errs) == 0 {
		return ChatResponse{}, errors.New("no AI provider configured")
	}
	return ChatResponse{}, errors.New(strings.Join(errs, "; "))
}

func providerOrder() []string {
	switch strings.ToLower(strings.TrimSpace(firstEnv("AI_PROVIDER", "AI_MODEL_PROVIDER"))) {
	case "openai":
		return []string{"openai", "together"}
	case "together", "auto", "", "qwen":
		return []string{"together", "openai"}
	default:
		return []string{"together", "openai"}
	}
}

func callOpenAI(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" || strings.HasPrefix(strings.ToLower(model), "meta-") || strings.HasPrefix(strings.ToLower(model), "qwen") {
		model = firstEnv("OPENAI_MODEL", "OPENAI_CHAT_MODEL")
	}
	if model == "" {
		model = "gpt-4.1-mini"
	}
	payload := chatCompletionPayload{Model: model, Messages: messages(req.System, req.Prompt), MaxTokens: req.MaxTokens, Temperature: req.Temperature}
	content, err := postChat(ctx, "https://api.openai.com/v1/chat/completions", os.Getenv("OPENAI_API_KEY"), payload)
	if err != nil {
		return ChatResponse{}, err
	}
	return ChatResponse{Provider: "openai", Model: model, Content: content}, nil
}

func callTogether(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" || strings.HasPrefix(strings.ToLower(model), "gpt-") {
		model = firstEnv("TOGETHER_MODEL", "TOGETHER_MODEL_SECURITY", "TOGETHER_MODEL_CHAT")
	}
	if model == "" {
		model = "Qwen/Qwen3.7-Plus"
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
