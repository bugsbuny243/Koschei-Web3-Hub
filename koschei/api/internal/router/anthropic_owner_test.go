package router

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
