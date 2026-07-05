package router

import (
	"os"
	"reflect"
	"testing"
)

func TestProviderOrderDefaultsTogetherFirst(t *testing.T) {
	t.Setenv("AI_PROVIDER", "")
	t.Setenv("AI_MODEL_PROVIDER", "")
	got := providerOrder()
	want := []string{"together", "openai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("providerOrder() = %v, want %v", got, want)
	}
}

func TestProviderOrderAllowsOpenAIFirst(t *testing.T) {
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("AI_MODEL_PROVIDER", "")
	got := providerOrder()
	want := []string{"openai", "together"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("providerOrder() = %v, want %v", got, want)
	}
}

func TestProviderOrderAcceptsQwenAlias(t *testing.T) {
	t.Setenv("AI_PROVIDER", "qwen")
	t.Setenv("AI_MODEL_PROVIDER", "")
	got := providerOrder()
	want := []string{"together", "openai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("providerOrder() = %v, want %v", got, want)
	}
}

func TestProviderOrderFallsBackToLegacyProviderEnv(t *testing.T) {
	_ = os.Unsetenv("AI_PROVIDER")
	t.Setenv("AI_MODEL_PROVIDER", "openai")
	got := providerOrder()
	want := []string{"openai", "together"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("providerOrder() = %v, want %v", got, want)
	}
}
