package router

import (
	"os"
	"testing"
)

func TestOrderedProvidersDefaultsToTogetherFirst(t *testing.T) {
	t.Setenv("AI_PROVIDER", "")
	t.Setenv("ARVIS_AI_PROVIDER", "")
	got := orderedProviders()
	if len(got) != 2 || got[0] != "together" || got[1] != "openai" {
		t.Fatalf("got %v, want together then openai", got)
	}
}

func TestOrderedProvidersAllowsOpenAIOverride(t *testing.T) {
	old := os.Getenv("AI_PROVIDER")
	defer os.Setenv("AI_PROVIDER", old)
	os.Setenv("AI_PROVIDER", "openai")
	got := orderedProviders()
	if len(got) != 2 || got[0] != "openai" || got[1] != "together" {
		t.Fatalf("got %v, want openai then together", got)
	}
}
