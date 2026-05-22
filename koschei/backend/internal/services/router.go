package services

import "strings"

type AIRouter struct{}

func (r AIRouter) SelectModel(message string, hasImage bool) string {
	if hasImage {
		return "meta-llama/Meta-Llama-3.1-405B-Instruct"
	}
	long := len(strings.Fields(message)) > 45
	deepKeywords := []string{"analyze", "mimari", "architecture", "deep", "strategy", "research", "explain"}
	for _, k := range deepKeywords {
		if strings.Contains(strings.ToLower(message), k) {
			return "meta-llama/Meta-Llama-3.1-405B-Instruct"
		}
	}
	if long {
		return "meta-llama/Meta-Llama-3.1-405B-Instruct"
	}
	return "Qwen/Qwen3-Coder-480B-A35B-Instruct"
}
