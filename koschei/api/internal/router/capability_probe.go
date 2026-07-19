package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultTogetherBaseURL       = "https://api.together.xyz/v1"
	maxCapabilityProbeBodyBytes  = 512 * 1024
	capabilityBasicToken         = "KOSCHEI_CAPABILITY_BASIC_OK"
	capabilityStructuredToken    = "KOSCHEI_CAPABILITY_STRUCTURED_OK"
	capabilityToolName           = "koschei_capability_probe"
	capabilityToolToken          = "KOSCHEI_CAPABILITY_TOOL_OK"
)

type CapabilityHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type TogetherCapabilityProbeRequest struct {
	Model                 string
	APIKey                string
	Endpoint              string
	Timeout               time.Duration
	Client                CapabilityHTTPClient
	AllowInsecureEndpoint bool
}

type CapabilityProbeStep struct {
	Supported  bool           `json:"supported"`
	StatusCode int            `json:"status_code,omitempty"`
	LatencyMS  int            `json:"latency_ms"`
	Result     map[string]any `json:"result"`
	Error      string         `json:"error,omitempty"`
}

type TogetherCapabilityProbeResult struct {
	Provider                  string              `json:"provider"`
	Model                     string              `json:"model"`
	Endpoint                  string              `json:"endpoint"`
	Available                 bool                `json:"available"`
	StructuredOutputSupported bool                `json:"structured_output_supported"`
	ToolCallingSupported      bool                `json:"tool_calling_supported"`
	Basic                     CapabilityProbeStep `json:"basic"`
	Structured                CapabilityProbeStep `json:"structured"`
	Tool                      CapabilityProbeStep `json:"tool"`
	ObservedAt                time.Time           `json:"observed_at"`
}

type capabilityChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   any `json:"content"`
			ToolCalls []struct {
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage map[string]any `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

// ProbeTogetherCapabilities performs three bounded owner-triggered calls:
// basic chat availability, JSON Schema output and function calling.
func ProbeTogetherCapabilities(ctx context.Context, input TogetherCapabilityProbeRequest) (TogetherCapabilityProbeResult, error) {
	input.Model = strings.TrimSpace(input.Model)
	input.APIKey = strings.TrimSpace(input.APIKey)
	if input.Model == "" {
		return TogetherCapabilityProbeResult{}, errors.New("Together model is required")
	}
	if input.APIKey == "" {
		input.APIKey = strings.TrimSpace(os.Getenv("TOGETHER_API_KEY"))
	}
	if input.APIKey == "" {
		return TogetherCapabilityProbeResult{}, errors.New("TOGETHER_API_KEY is not configured")
	}
	endpoint, err := normalizeTogetherChatEndpoint(input.Endpoint, input.AllowInsecureEndpoint)
	if err != nil {
		return TogetherCapabilityProbeResult{}, err
	}
	if input.Timeout <= 0 || input.Timeout > 90*time.Second {
		input.Timeout = 45 * time.Second
	}
	if input.Client == nil {
		input.Client = &http.Client{Timeout: input.Timeout}
	}
	probeCtx, cancel := context.WithTimeout(ctx, input.Timeout)
	defer cancel()

	result := TogetherCapabilityProbeResult{
		Provider:   "together",
		Model:      input.Model,
		Endpoint:   endpoint,
		ObservedAt: time.Now().UTC(),
	}
	result.Basic = runTogetherBasicProbe(probeCtx, input.Client, endpoint, input.APIKey, input.Model)
	result.Structured = runTogetherStructuredProbe(probeCtx, input.Client, endpoint, input.APIKey, input.Model)
	result.Tool = runTogetherToolProbe(probeCtx, input.Client, endpoint, input.APIKey, input.Model)
	result.Available = result.Basic.Supported
	result.StructuredOutputSupported = result.Structured.Supported
	result.ToolCallingSupported = result.Tool.Supported
	return result, nil
}

func runTogetherBasicProbe(ctx context.Context, client CapabilityHTTPClient, endpoint, apiKey, model string) CapabilityProbeStep {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{{"role": "user", "content": "Reply with exactly " + capabilityBasicToken}},
		"temperature": 0,
		"max_tokens": 16,
	}
	response, status, latency, err := postTogetherCapabilityProbe(ctx, client, endpoint, apiKey, payload)
	step := CapabilityProbeStep{StatusCode: status, LatencyMS: latency, Result: map[string]any{}}
	if err != nil {
		step.Error = err.Error()
		return step
	}
	content := firstMessageContent(response)
	step.Result = map[string]any{
		"model": response.Model,
		"finish_reason": firstFinishReason(response),
		"token_observed": strings.TrimSpace(content) == capabilityBasicToken,
		"usage": response.Usage,
	}
	step.Supported = strings.TrimSpace(content) == capabilityBasicToken
	if !step.Supported {
		step.Error = "basic capability token was not returned exactly"
	}
	return step
}

func runTogetherStructuredProbe(ctx context.Context, client CapabilityHTTPClient, endpoint, apiKey, model string) CapabilityProbeStep {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ok": map[string]any{"type": "boolean", "const": true},
			"token": map[string]any{"type": "string", "enum": []string{capabilityStructuredToken}},
		},
		"required": []string{"ok", "token"},
		"additionalProperties": false,
	}
	schemaRaw, _ := json.Marshal(schema)
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "Respond only in JSON matching this exact schema: " + string(schemaRaw)},
			{"role": "user", "content": "Return the capability acknowledgement object."},
		},
		"temperature": 0,
		"max_tokens": 64,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{"name": "koschei_capability_probe", "schema": schema},
		},
	}
	response, status, latency, err := postTogetherCapabilityProbe(ctx, client, endpoint, apiKey, payload)
	step := CapabilityProbeStep{StatusCode: status, LatencyMS: latency, Result: map[string]any{}}
	if err != nil {
		step.Error = err.Error()
		return step
	}
	content := firstMessageContent(response)
	var decoded struct {
		OK    bool   `json:"ok"`
		Token string `json:"token"`
	}
	decodeErr := DecodeJSONObject(content, &decoded)
	step.Result = map[string]any{
		"model": response.Model,
		"finish_reason": firstFinishReason(response),
		"schema_valid": decodeErr == nil,
		"token_observed": decoded.OK && decoded.Token == capabilityStructuredToken,
		"usage": response.Usage,
	}
	step.Supported = decodeErr == nil && decoded.OK && decoded.Token == capabilityStructuredToken
	if decodeErr != nil {
		step.Error = "structured output was not valid for the probe schema: " + decodeErr.Error()
	} else if !step.Supported {
		step.Error = "structured capability token was not returned"
	}
	return step
}

func runTogetherToolProbe(ctx context.Context, client CapabilityHTTPClient, endpoint, apiKey, model string) CapabilityProbeStep {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{{"role": "user", "content": "Call the provided capability probe tool with its required token."}},
		"temperature": 0,
		"max_tokens": 64,
		"tools": []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name": capabilityToolName,
				"description": "A no-op Koschei capability probe. It does not execute external actions.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{"token": map[string]any{"type": "string", "enum": []string{capabilityToolToken}}},
					"required": []string{"token"},
					"additionalProperties": false,
				},
			},
		}},
		"tool_choice": map[string]any{"type": "function", "function": map[string]string{"name": capabilityToolName}},
	}
	response, status, latency, err := postTogetherCapabilityProbe(ctx, client, endpoint, apiKey, payload)
	step := CapabilityProbeStep{StatusCode: status, LatencyMS: latency, Result: map[string]any{}}
	if err != nil {
		step.Error = err.Error()
		return step
	}
	functionName, arguments := firstToolCall(response)
	var decoded struct {
		Token string `json:"token"`
	}
	decodeErr := json.Unmarshal([]byte(arguments), &decoded)
	step.Result = map[string]any{
		"model": response.Model,
		"finish_reason": firstFinishReason(response),
		"function_name": functionName,
		"arguments_valid": decodeErr == nil,
		"token_observed": decoded.Token == capabilityToolToken,
		"usage": response.Usage,
	}
	step.Supported = functionName == capabilityToolName && decodeErr == nil && decoded.Token == capabilityToolToken
	if functionName == "" {
		step.Error = "model returned no tool call"
	} else if decodeErr != nil {
		step.Error = "tool arguments were not valid JSON"
	} else if !step.Supported {
		step.Error = "tool name or capability token did not match"
	}
	return step
}

func postTogetherCapabilityProbe(ctx context.Context, client CapabilityHTTPClient, endpoint, apiKey string, payload map[string]any) (capabilityChatResponse, int, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return capabilityChatResponse{}, 0, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return capabilityChatResponse{}, 0, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	resp, err := client.Do(req)
	latency := int(time.Since(started).Milliseconds())
	if err != nil {
		return capabilityChatResponse{}, 0, latency, fmt.Errorf("Together capability request failed: %w", err)
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxCapabilityProbeBodyBytes+1))
	if readErr != nil {
		return capabilityChatResponse{}, resp.StatusCode, latency, readErr
	}
	if len(data) > maxCapabilityProbeBodyBytes {
		return capabilityChatResponse{}, resp.StatusCode, latency, errors.New("Together capability response exceeded the size limit")
	}
	var decoded capabilityChatResponse
	if len(data) > 0 {
		if err := json.Unmarshal(data, &decoded); err != nil {
			return capabilityChatResponse{}, resp.StatusCode, latency, errors.New("Together capability response was not valid JSON")
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := "Together capability endpoint returned HTTP " + fmt.Sprint(resp.StatusCode)
		if decoded.Error != nil && strings.TrimSpace(decoded.Error.Message) != "" {
			message += ": " + boundedCapabilityError(decoded.Error.Message)
		}
		return decoded, resp.StatusCode, latency, errors.New(message)
	}
	if len(decoded.Choices) == 0 {
		return decoded, resp.StatusCode, latency, errors.New("Together capability response contained no choices")
	}
	return decoded, resp.StatusCode, latency, nil
}

func normalizeTogetherChatEndpoint(raw string, allowInsecure bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("TOGETHER_BASE_URL"))
	}
	if raw == "" {
		raw = defaultTogetherBaseURL
	}
	raw = strings.TrimRight(raw, "/")
	if !strings.HasSuffix(raw, "/chat/completions") {
		if !strings.HasSuffix(raw, "/v1") {
			raw += "/v1"
		}
		raw += "/chat/completions"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return "", errors.New("invalid Together capability endpoint")
	}
	if allowInsecure {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", errors.New("capability endpoint must use HTTP or HTTPS")
		}
		return parsed.String(), nil
	}
	if parsed.Scheme != "https" {
		return "", errors.New("Together capability endpoint must use HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "api.together.xyz" && host != "api.together.ai" {
		return "", errors.New("Together capability endpoint host is not allowed")
	}
	return parsed.String(), nil
}

func firstMessageContent(response capabilityChatResponse) string {
	if len(response.Choices) == 0 {
		return ""
	}
	switch value := response.Choices[0].Message.Content.(type) {
	case string:
		return value
	case nil:
		return ""
	default:
		encoded, _ := json.Marshal(value)
		return string(encoded)
	}
}

func firstFinishReason(response capabilityChatResponse) string {
	if len(response.Choices) == 0 {
		return ""
	}
	return response.Choices[0].FinishReason
}

func firstToolCall(response capabilityChatResponse) (string, string) {
	if len(response.Choices) == 0 || len(response.Choices[0].Message.ToolCalls) == 0 {
		return "", ""
	}
	call := response.Choices[0].Message.ToolCalls[0]
	return strings.TrimSpace(call.Function.Name), strings.TrimSpace(call.Function.Arguments)
}

func boundedCapabilityError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 500 {
		value = value[:500]
	}
	return value
}
