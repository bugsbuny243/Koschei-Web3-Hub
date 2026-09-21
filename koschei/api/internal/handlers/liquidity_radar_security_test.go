package handlers

import "testing"

func TestTrustedWebhookURL(t *testing.T) {
	t.Parallel()

	accepted := []struct {
		provider string
		url      string
	}{
		{provider: "telegram", url: "https://api.telegram.org/bot123/sendMessage"},
		{provider: "discord", url: "https://discord.com/api/webhooks/123/token"},
		{provider: "discord", url: "https://canary.discord.com/api/webhooks/123/token"},
	}
	for _, tc := range accepted {
		if _, err := trustedWebhookURL(tc.url, tc.provider); err != nil {
			t.Fatalf("expected %s URL %q to be accepted: %v", tc.provider, tc.url, err)
		}
	}

	rejected := []struct {
		provider string
		url      string
	}{
		{provider: "telegram", url: "http://api.telegram.org/bot123/sendMessage"},
		{provider: "telegram", url: "https://api.telegram.org.evil.example/bot123/sendMessage"},
		{provider: "telegram", url: "https://127.0.0.1/bot123/sendMessage"},
		{provider: "discord", url: "https://discord.com.evil.example/api/webhooks/123/token"},
		{provider: "discord", url: "https://discord.com:8443/api/webhooks/123/token"},
		{provider: "discord", url: "https://discord.com/not-a-webhook"},
		{provider: "unknown", url: "https://example.com/hook"},
	}
	for _, tc := range rejected {
		if _, err := trustedWebhookURL(tc.url, tc.provider); err == nil {
			t.Fatalf("expected %s URL %q to be rejected", tc.provider, tc.url)
		}
	}
}
