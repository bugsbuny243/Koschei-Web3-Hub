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

const (
	anthropicOwnerDefaultModel = "claude-sonnet-5"
	anthropicAPIVersion        = "2023-06-01"
)

var (
	anthropicOwnerEndpoint   = "https://api.anthropic.com/v1/messages"
	anthropicOwnerHTTPClient = &http.Client{Timeout: 90 * time.Second}
)

type anthropicOwnerPayload struct {
	Model       string                  `json:"model"`
	System      string                  `json:"system,omitempty"`
	Messages    []anthropicOwnerMessage `json:"messages"`
	MaxTokens   int                     `json:"max_tokens"`
	Temperature float64                 `json:"temperature,omitempty"`
}

type anthropicOwnerMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicOwnerResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// OwnerChat is a dedicated Anthropic-only route for the private owner panel.
// It deliberately does not fall back to Together/Qwen or any customer model.
func OwnerChat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return ChatResponse{}, errors.New("prompt is required")
	}
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		return ChatResponse{}, errors.New("ANTHROPIC_API_KEY is not configured")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(os.Getenv("ANTHROPIC_OWNER_MODEL"))
	}
	if model == "" {
		model = anthropicOwnerDefaultModel
	}
	if !strings.HasPrefix(strings.ToLower(model), "claude-") {
		return ChatResponse{}, fmt.Errorf("owner model must be a Claude model")
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 1600
	}
	if req.Timeout <= 0 {
		req.Timeout = 65 * time.Second
	}
	if req.Temperature < 0 || req.Temperature > 1 {
		req.Temperature = 0.2
	}
	if req.Temperature == 0 {
		req.Temperature = 0.2
	}

	callCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()
	payload := anthropicOwnerPayload{
		Model:       model,
		System:      strings.TrimSpace(req.System),
		Messages:    []anthropicOwnerMessage{{Role: "user", Content: req.Prompt}},
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, anthropicOwnerEndpoint, bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := anthropicOwnerHTTPClient.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		return ChatResponse{}, readErr
	}
	var decoded anthropicOwnerResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return ChatResponse{}, fmt.Errorf("anthropic returned HTTP %d", resp.StatusCode)
		}
		return ChatResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := "request failed"
		if decoded.Error != nil && strings.TrimSpace(decoded.Error.Message) != "" {
			message = compactAnthropicError(decoded.Error.Message)
		}
		return ChatResponse{}, fmt.Errorf("anthropic returned HTTP %d: %s", resp.StatusCode, message)
	}

	parts := make([]string, 0, len(decoded.Content))
	for _, block := range decoded.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	content := strings.TrimSpace(strings.Join(parts, "\n"))
	if content == "" {
		return ChatResponse{}, errors.New("anthropic returned empty content")
	}
	responseModel := strings.TrimSpace(decoded.Model)
	if responseModel == "" {
		responseModel = model
	}
	return ChatResponse{Provider: "anthropic", Model: responseModel, Content: content}, nil
}

func compactAnthropicError(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len(value) > 240 {
		value = value[:240]
	}
	return value
}
