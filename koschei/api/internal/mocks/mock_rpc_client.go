package mocks

import (
	"context"
	"encoding/json"
	"sync"
)

type MockRPCClient struct {
	mu        sync.Mutex
	Calls     map[string]int
	Responses map[string]any
	Errors    map[string]error
	Provider  string
}

func NewMockRPCClient() *MockRPCClient {
	return &MockRPCClient{Calls: map[string]int{}, Responses: map[string]any{}, Errors: map[string]error{}, Provider: "mock"}
}

func (m *MockRPCClient) Call(_ context.Context, method string, _ any, target any) (string, error) {
	m.mu.Lock()
	m.Calls[method]++
	err := m.Errors[method]
	resp := m.Responses[method]
	provider := m.Provider
	m.mu.Unlock()
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(resp)
	return provider, json.Unmarshal(b, target)
}
