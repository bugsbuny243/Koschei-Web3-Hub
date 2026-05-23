package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type TogetherClient struct{ APIKey string }

type PythonWorkerClient struct{ BaseURL string }

func (c TogetherClient) Chat(model, message string) (string, error) {
	payload := map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": message}}}
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

func (c PythonWorkerClient) Generate(task TaskType, model string, input map[string]any) (map[string]any, error) {
	payload := map[string]any{"task": task, "model": model, "input": input}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, c.BaseURL+"/worker/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("worker error: %s", string(data))
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
