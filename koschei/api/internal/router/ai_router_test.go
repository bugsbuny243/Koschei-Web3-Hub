package router

import "testing"

func TestProviderFromEnvUsesTogetherOnly(t *testing.T) {
	oldGetEnv := getEnv
	defer func() { getEnv = oldGetEnv }()
	getEnv = func(key string) string {
		if key == "TOGETHER_API_KEY" {
			return "test-key"
		}
		return ""
	}
	if got := providerFromEnv(); got != "together" {
		t.Fatalf("got %q, want together", got)
	}
}

func TestProviderFromEnvUnconfiguredWithoutTogether(t *testing.T) {
	oldGetEnv := getEnv
	defer func() { getEnv = oldGetEnv }()
	getEnv = func(key string) string { return "" }
	if got := providerFromEnv(); got != "unconfigured" {
		t.Fatalf("got %q, want unconfigured", got)
	}
}
