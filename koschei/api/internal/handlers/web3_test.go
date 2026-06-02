package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestWebhookSecretFromRequestPrefersKoscheiHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/web3/events/alchemy", nil)
	req.Header.Set("X-Koschei-Webhook-Secret", " koschei-secret ")
	req.Header.Set("Authorization", "Bearer bearer-secret")
	req.Header.Set("X-Alchemy-Token", "alchemy-token")

	if got := webhookSecretFromRequest(req); got != "koschei-secret" {
		t.Fatalf("webhookSecretFromRequest() = %q, want %q", got, "koschei-secret")
	}
}

func TestWebhookSecretFromRequestAcceptsBearerToken(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/web3/events/alchemy", nil)
	req.Header.Set("Authorization", "Bearer bearer-secret")
	req.Header.Set("X-Alchemy-Token", "alchemy-token")

	if got := webhookSecretFromRequest(req); got != "bearer-secret" {
		t.Fatalf("webhookSecretFromRequest() = %q, want %q", got, "bearer-secret")
	}
}

func TestWebhookSecretFromRequestAcceptsAlchemyTokenFallback(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/web3/events/alchemy", nil)
	req.Header.Set("X-Alchemy-Token", " alchemy-token ")

	if got := webhookSecretFromRequest(req); got != "alchemy-token" {
		t.Fatalf("webhookSecretFromRequest() = %q, want %q", got, "alchemy-token")
	}
}

func TestWebhookSecretFromRequestRejectsNonBearerAuthorization(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/web3/events/alchemy", nil)
	req.Header.Set("Authorization", "Basic bearer-secret")

	if got := webhookSecretFromRequest(req); got != "" {
		t.Fatalf("webhookSecretFromRequest() = %q, want empty string", got)
	}
}
