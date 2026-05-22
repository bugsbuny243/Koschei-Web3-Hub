package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type TogetherClient struct {
	APIKey string
}

type FalClient struct {
	APIKey string
}

func (c TogetherClient) Chat(model, message string) (string, error) {
	payload := map[string]any{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": message}},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.together.xyz/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("together api error: %s", string(data))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}
	return out.Choices[0].Message.Content, nil
}

func (f FalClient) GenerateImage(prompt string) (string, error) {
	return f.callFal("https://fal.run/fal-ai/flux-pro/v1.1", prompt)
}

func (f FalClient) GenerateVideo(prompt string) (string, error) {
	return f.callFal("https://fal.run/fal-ai/kling-video/v2.5/standard/text-to-video", prompt)
}

func (f FalClient) callFal(url, prompt string) (string, error) {
	payload := map[string]string{"prompt": prompt}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Key "+f.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("fal api error: %s", string(data))
	}
	return string(data), nil
}
