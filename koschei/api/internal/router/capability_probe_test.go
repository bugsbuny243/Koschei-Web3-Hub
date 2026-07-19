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

func TestProbeTogetherCapabilitiesPassed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header missing")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case payload["response_format"] != nil:
			_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true,\"token\":\"KOSCHEI_CAPABILITY_STRUCTURED_OK\"}"}}],"usage":{"total_tokens":4}}`))
		case payload["tools"] != nil:
			_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"finish_reason":"tool_calls","message":{"content":null,"tool_calls":[{"type":"function","function":{"name":"koschei_capability_probe","arguments":"{\"token\":\"KOSCHEI_CAPABILITY_TOOL_OK\"}"}}]}}],"usage":{"total_tokens":5}}`))
		default:
			_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"finish_reason":"stop","message":{"content":"KOSCHEI_CAPABILITY_BASIC_OK"}}],"usage":{"total_tokens":3}}`))
		}
	}))
	defer server.Close()

	result, err := ProbeTogetherCapabilities(context.Background(), TogetherCapabilityProbeRequest{
		Model: "test-model",
		APIKey: "test-key",
		Endpoint: server.URL,
		Timeout: 5 * time.Second,
		Client: server.Client(),
		AllowInsecureEndpoint: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || !result.StructuredOutputSupported || !result.ToolCallingSupported {
		t.Fatalf("unexpected capability result: %+v", result)
	}
	if result.Basic.StatusCode != http.StatusOK || result.Structured.StatusCode != http.StatusOK || result.Tool.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status codes: %+v", result)
	}
	if result.Model != "test-model" || result.Provider != "together" || result.ObservedAt.IsZero() {
		t.Fatalf("unexpected identity: %+v", result)
	}
}

func TestProbeTogetherCapabilitiesPartialWhenToolCallMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case payload["response_format"] != nil:
			_, _ = w.Write([]byte(`{"model":"partial-model","choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true,\"token\":\"KOSCHEI_CAPABILITY_STRUCTURED_OK\"}"}}]}`))
		case payload["tools"] != nil:
			_, _ = w.Write([]byte(`{"model":"partial-model","choices":[{"finish_reason":"stop","message":{"content":"I cannot call tools."}}]}`))
		default:
			_, _ = w.Write([]byte(`{"model":"partial-model","choices":[{"finish_reason":"stop","message":{"content":"KOSCHEI_CAPABILITY_BASIC_OK"}}]}`))
		}
	}))
	defer server.Close()

	result, err := ProbeTogetherCapabilities(context.Background(), TogetherCapabilityProbeRequest{
		Model: "partial-model", APIKey: "key", Endpoint: server.URL,
		Client: server.Client(), AllowInsecureEndpoint: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || !result.StructuredOutputSupported || result.ToolCallingSupported {
		t.Fatalf("unexpected partial result: %+v", result)
	}
	if !strings.Contains(result.Tool.Error, "no tool call") {
		t.Fatalf("missing tool limitation: %+v", result.Tool)
	}
}

func TestProbeTogetherCapabilitiesContinuesAfterBasicHTTPFailure(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.Header().Set("Content-Type", "application/json")
		if payload["response_format"] == nil && payload["tools"] == nil {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"model not found"}}`))
			return
		}
		if payload["response_format"] != nil {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true,\"token\":\"KOSCHEI_CAPABILITY_STRUCTURED_OK\"}"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"type":"function","function":{"name":"koschei_capability_probe","arguments":"{\"token\":\"KOSCHEI_CAPABILITY_TOOL_OK\"}"}}]}}]}`))
	}))
	defer server.Close()

	result, err := ProbeTogetherCapabilities(context.Background(), TogetherCapabilityProbeRequest{
		Model: "missing-model", APIKey: "key", Endpoint: server.URL,
		Client: server.Client(), AllowInsecureEndpoint: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if requestCount != 3 {
		t.Fatalf("expected all three probes, got %d", requestCount)
	}
	if result.Available || !result.StructuredOutputSupported || !result.ToolCallingSupported {
		t.Fatalf("unexpected independent probe result: %+v", result)
	}
	if result.Basic.StatusCode != http.StatusNotFound || !strings.Contains(result.Basic.Error, "model not found") {
		t.Fatalf("unexpected basic failure: %+v", result.Basic)
	}
}

func TestNormalizeTogetherChatEndpointRejectsUntrustedHost(t *testing.T) {
	if _, err := normalizeTogetherChatEndpoint("https://example.com/v1", false); err == nil {
		t.Fatal("untrusted Together endpoint host was accepted")
	}
	endpoint, err := normalizeTogetherChatEndpoint("https://api.together.xyz/v1", false)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "https://api.together.xyz/v1/chat/completions" {
		t.Fatalf("unexpected normalized endpoint: %s", endpoint)
	}
}
