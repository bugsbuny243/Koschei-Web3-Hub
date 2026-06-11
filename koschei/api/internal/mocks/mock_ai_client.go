package mocks

import "context"

type MockAIClient struct {
	Response     string
	Err          error
	InputTokens  int
	OutputTokens int
}

func (m MockAIClient) Analyze(_ context.Context, _ string, _ any) (string, int, int, error) {
	return m.Response, m.InputTokens, m.OutputTokens, m.Err
}
