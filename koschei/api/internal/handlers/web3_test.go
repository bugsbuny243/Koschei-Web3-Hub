package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

func TestVerifyAlchemySignature(t *testing.T) {
	body := []byte(`{"ok":true}`)
	key := "signing-key"
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if !verifyAlchemySignature(body, key, sig) {
		t.Fatalf("verifyAlchemySignature() rejected a valid signature")
	}
	if !verifyAlchemySignature(body, key, "sha256="+sig) {
		t.Fatalf("verifyAlchemySignature() rejected a sha256-prefixed valid signature")
	}
	if verifyAlchemySignature(body, key, "bad") {
		t.Fatalf("verifyAlchemySignature() accepted an invalid signature")
	}
}

func TestNormalizeAlchemyActivities(t *testing.T) {
	payload := map[string]any{
		"webhookId": "wh_123",
		"id":        "evt_123",
		"createdAt": "2026-06-03T00:00:00Z",
		"type":      "ADDRESS_ACTIVITY",
		"event": map[string]any{
			"network": "base-sepolia",
			"activity": []any{
				map[string]any{
					"category":    "external",
					"fromAddress": "0xFrom",
					"toAddress":   "0xTo",
					"hash":        "0xHash",
					"value":       "1.5",
					"asset":       "ETH",
				},
			},
		},
	}
	events := normalizeAlchemyActivities(payload)
	if len(events) != 1 {
		t.Fatalf("normalizeAlchemyActivities() len = %d, want 1", len(events))
	}
	if events[0].Chain != "base" || events[0].Network != "base-sepolia" || events[0].TxHash != "0xHash" {
		t.Fatalf("unexpected normalized event: %+v", events[0])
	}
}
