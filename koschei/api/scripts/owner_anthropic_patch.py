from pathlib import Path
import re


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

router_path = Path("internal/router/anthropic_owner.go")
router_path.write_text(r'''package router

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
	Model       string                   `json:"model"`
	System      string                   `json:"system,omitempty"`
	Messages    []anthropicOwnerMessage  `json:"messages"`
	MaxTokens   int                      `json:"max_tokens"`
	Temperature float64                  `json:"temperature,omitempty"`
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
''')

test_path = Path("internal/router/anthropic_owner_test.go")
test_path.write_text(r'''package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOwnerChatUsesAnthropicOnly(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "owner-secret")
	t.Setenv("ANTHROPIC_OWNER_MODEL", "claude-sonnet-5")
	t.Setenv("TOGETHER_API_KEY", "customer-secret")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("x-api-key"); got != "owner-secret" {
			t.Fatalf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != anthropicAPIVersion {
			t.Fatalf("anthropic-version = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("unexpected Authorization header: %q", got)
		}
		var payload anthropicOwnerPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "claude-sonnet-5" || payload.System != "owner-system" {
			t.Fatalf("payload = %#v", payload)
		}
		if len(payload.Messages) != 1 || payload.Messages[0].Content != "owner prompt" {
			t.Fatalf("messages = %#v", payload.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"claude-sonnet-5","content":[{"type":"text","text":"Owner cevabı"}]}`))
	}))
	defer server.Close()

	oldEndpoint, oldClient := anthropicOwnerEndpoint, anthropicOwnerHTTPClient
	anthropicOwnerEndpoint, anthropicOwnerHTTPClient = server.URL, server.Client()
	defer func() { anthropicOwnerEndpoint, anthropicOwnerHTTPClient = oldEndpoint, oldClient }()

	response, err := OwnerChat(context.Background(), ChatRequest{System: "owner-system", Prompt: "owner prompt", MaxTokens: 800, Temperature: 0.2, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if response.Provider != "anthropic" || response.Model != "claude-sonnet-5" || response.Content != "Owner cevabı" {
		t.Fatalf("response = %#v", response)
	}
}

func TestOwnerChatDoesNotFallbackToTogether(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("TOGETHER_API_KEY", "customer-secret")
	_, err := OwnerChat(context.Background(), ChatRequest{Prompt: "owner prompt"})
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("expected Anthropic configuration error, got %v", err)
	}
}

func TestOwnerChatRejectsNonClaudeModel(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "owner-secret")
	_, err := OwnerChat(context.Background(), ChatRequest{Prompt: "owner prompt", Model: "Qwen/Qwen3-235B-A22B-2507"})
	if err == nil || !strings.Contains(err.Error(), "Claude model") {
		t.Fatalf("expected model isolation error, got %v", err)
	}
}
''')

chat_path = Path("internal/handlers/owner_ai_chat.go")
chat = chat_path.read_text()
chat = replace_once(chat, '"time"\n)', '"time"\n\n\t"koschei/api/internal/router"\n)', "owner chat router import")
chat = chat.replace("aiProviderConfigured()", "ownerAIProviderConfigured()")
chat = replace_once(
    chat,
    '\treply, err := h.callTogetherWithSystemTimeoutAndMaxTokens(ownerChatModel(), ownerChatSystemPrompt, prompt, 65*time.Second, 1600)',
    '\taiReply, err := router.OwnerChat(ctx, router.ChatRequest{System: ownerChatSystemPrompt, Prompt: prompt, Model: ownerChatModel(), Timeout: 65 * time.Second, MaxTokens: 1600, Temperature: 0.2})\n\treply := aiReply.Content',
    "owner Anthropic generation call",
)
chat = replace_once(
    chat,
    '\t\t"ai_ready":  ownerAIProviderConfigured(),\n\t\t"model":     firstNonEmpty(ownerChatModel(), "router-default"),',
    '\t\t"ai_ready":  ownerAIProviderConfigured(),\n\t\t"provider":  "anthropic",\n\t\t"model":     ownerChatModel(),',
    "owner history provider metadata",
)
chat = replace_once(
    chat,
    '\t\t"created_thread": createdThread,\n\t\t"model":           firstNonEmpty(ownerChatModel(), "router-default"),',
    '\t\t"created_thread": createdThread,\n\t\t"provider":       aiReply.Provider,\n\t\t"model":          aiReply.Model,',
    "owner send provider metadata",
)
chat_path.write_text(chat)

context_path = Path("internal/handlers/owner_ai_chat_context.go")
context = context_path.read_text()
status_pattern = re.compile(r'func ownerAIProviderStatus\(\) map\[string\]any \{.*?\n\}', re.S)
status_replacement = '''func ownerAIProviderStatus() map[string]any {
	return map[string]any{
		"configured": ownerAIProviderConfigured(),
		"provider":   "anthropic",
		"model":      ownerChatModel(),
		"scope":      "owner_panel_only",
	}
}'''
context, count = status_pattern.subn(status_replacement, context, count=1)
if count != 1:
    raise SystemExit("ownerAIProviderStatus replacement failed")
model_pattern = re.compile(r'func ownerChatModel\(\) string \{.*?\n\}', re.S)
model_replacement = '''func ownerChatModel() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("ANTHROPIC_OWNER_MODEL")), "claude-sonnet-5")
}

func ownerAIProviderConfigured() bool {
	return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != ""
}'''
context, count = model_pattern.subn(model_replacement, context, count=1)
if count != 1:
    raise SystemExit("ownerChatModel replacement failed")
context_path.write_text(context)
