package handlers

import (
	"strings"
	"testing"
	"time"
)

func TestOwnerChatTitle(t *testing.T) {
	if got := ownerChatTitle("   Radar   şu an sağlıklı mı?   "); got != "Radar şu an sağlıklı mı?" {
		t.Fatalf("ownerChatTitle() = %q", got)
	}
	long := strings.Repeat("a", 80)
	if got := ownerChatTitle(long); len([]rune(got)) != 55 || !strings.HasSuffix(got, "…") {
		t.Fatalf("long title was not safely truncated: %q", got)
	}
}

func TestNormalizeOwnerChatText(t *testing.T) {
	got := normalizeOwnerChatText("  hello\x00 world  ", 8)
	if got != "hello wo" {
		t.Fatalf("normalizeOwnerChatText() = %q", got)
	}
}

func TestOwnerChatWindowKeepsNewestMessages(t *testing.T) {
	messages := []ownerChatMessage{
		{ID: "1", Role: "user", Content: "12345"},
		{ID: "2", Role: "assistant", Content: "67890"},
		{ID: "3", Role: "user", Content: "abcde"},
	}
	got := ownerChatWindow(messages, 9)
	if len(got) != 1 || got[0].ID != "3" {
		t.Fatalf("ownerChatWindow() = %#v", got)
	}
}

func TestBuildOwnerChatPromptIncludesSnapshotAndConversation(t *testing.T) {
	snapshot := ownerChatSnapshot{
		GeneratedAt: "2026-06-22T12:00:00Z",
		Services: map[string]any{"database": "connected"},
		Business: map[string]any{"total_users": 4},
		Radar: map[string]any{"retryable": 0},
		GooglePlay: map[string]any{"aab_upload_ready": true},
	}
	messages := []ownerChatMessage{
		{Role: "user", Content: "Radar nasıl?", CreatedAt: time.Now()},
		{Role: "assistant", Content: "Kontrol ediyorum.", CreatedAt: time.Now()},
	}
	prompt := buildOwnerChatPrompt(snapshot, messages, map[string]any{"intent": "radar_health"})
	for _, expected := range []string{"CURRENT OPERATIONAL SNAPSHOT", `"retryable":0`, "USER: Radar nasıl?", "ASSISTANT: Kontrol ediyorum.", "DETERMINISTIC READ-ONLY RESULT"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q: %s", expected, prompt)
		}
	}
}

func TestOwnerChatSystemPromptEnforcesReadOnlyAndSecretSafety(t *testing.T) {
	for _, expected := range []string{"read-only", "Never reveal", "Never claim", "Auth is frozen"} {
		if !strings.Contains(ownerChatSystemPrompt, expected) {
			t.Fatalf("system prompt missing safety rule %q", expected)
		}
	}
}

func TestOwnerChatIdentityNormalizesOwnerWallet(t *testing.T) {
	t.Setenv("OWNER_WALLET", "OwnerWalletABC")
	if got := ownerChatIdentity(); got != "owner:ownerwalletabc" {
		t.Fatalf("ownerChatIdentity() = %q", got)
	}
}
