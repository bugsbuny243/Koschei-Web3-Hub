package alerts

import (
	"context"
	"errors"
	"net/url"
	"testing"
)

func TestNormalizeEventProducesStableDedupeKey(t *testing.T) {
	first := normalizeEvent(Event{AuthSubject: "customer-a", Source: " arvis ", EventType: "ARVIS.VERDICT.CREATED", Severity: "warning", Target: "mint", Title: "Risk", Message: "Evidence"})
	second := normalizeEvent(Event{AuthSubject: "customer-a", Source: "arvis", EventType: "arvis.verdict.created", Severity: "medium", Target: "mint", Title: "Risk", Message: "Evidence"})
	if first.DedupeKey == "" || first.DedupeKey != second.DedupeKey {
		t.Fatalf("dedupe keys differ: %q %q", first.DedupeKey, second.DedupeKey)
	}
	if first.Severity != "medium" {
		t.Fatalf("severity = %q", first.Severity)
	}
}

func TestDefaultDedupeKeyIsTenantScoped(t *testing.T) {
	base := Event{Source: "arvis", EventType: EventARVISVerdictCreated, Severity: "high", Target: "mint", Title: "Risk", EvidenceRef: "signature"}
	first := base
	first.AuthSubject = "customer-a"
	second := base
	second.AuthSubject = "customer-b"
	if defaultDedupeKey(first) == defaultDedupeKey(second) {
		t.Fatal("default alert dedupe key collided across tenants")
	}
}

func TestCustomDedupeKeyIsStableWithinTenantAndScopedAcrossTenants(t *testing.T) {
	first := normalizeEvent(Event{AuthSubject: "customer-a", Source: "arvis", EventType: EventARVISVerdictCreated, Severity: "high", Title: "Risk", DedupeKey: "signed-verdict:abc"})
	repeat := normalizeEvent(Event{AuthSubject: "customer-a", Source: "arvis", EventType: EventARVISVerdictCreated, Severity: "critical", Title: "Updated risk", DedupeKey: " signed-verdict:abc "})
	second := normalizeEvent(Event{AuthSubject: "customer-b", Source: "arvis", EventType: EventARVISVerdictCreated, Severity: "high", Title: "Risk", DedupeKey: "signed-verdict:abc"})
	if first.DedupeKey != repeat.DedupeKey {
		t.Fatalf("same-tenant custom key was not stable: %q %q", first.DedupeKey, repeat.DedupeKey)
	}
	if first.DedupeKey == second.DedupeKey {
		t.Fatal("caller-supplied alert dedupe key collided across tenants")
	}
	if first.DedupeKey != scopedCustomDedupeKey("customer-a", "signed-verdict:abc") {
		t.Fatalf("unexpected tenant key: %q", first.DedupeKey)
	}
	if got := scopedCustomDedupeKey("", "operator-global-key"); got != "operator-global-key" {
		t.Fatalf("unscoped operator key changed: %q", got)
	}
}

func TestShouldQueueSystemChannelsUsesHighDefault(t *testing.T) {
	t.Setenv("SECURITY_ALERT_MIN_SEVERITY", "")
	if shouldQueueSystemChannels("medium") {
		t.Fatal("medium alert should not reach system channels by default")
	}
	if !shouldQueueSystemChannels("high") || !shouldQueueSystemChannels("critical") {
		t.Fatal("high and critical alerts should reach system channels by default")
	}
}

func TestAllowedDiscordHostAcceptsOnlyOfficialHosts(t *testing.T) {
	for _, host := range []string{"discord.com", "www.discord.com", "discordapp.com"} {
		if !allowedDiscordHost(host) {
			t.Fatalf("official host %q was rejected", host)
		}
	}
	if allowedDiscordHost("chat.invalid") {
		t.Fatal("non-Discord host was accepted")
	}
}

func TestAllowedDiscordWebhookURLRequiresOfficialWebhookPath(t *testing.T) {
	valid, _ := url.Parse("https://discord.com/api/webhooks/123/token")
	if !allowedDiscordWebhookURL(valid) {
		t.Fatal("valid Discord webhook URL was rejected")
	}
	for _, raw := range []string{
		"http://discord.com/api/webhooks/123/token",
		"https://discord.com/channels/123",
		"https://chat.invalid/api/webhooks/123/token",
		"https://user@discord.com/api/webhooks/123/token",
	} {
		candidate, _ := url.Parse(raw)
		if allowedDiscordWebhookURL(candidate) {
			t.Fatalf("unsafe Discord URL was accepted: %s", raw)
		}
	}
}

func TestSafeProviderFailureNeverReturnsProviderURLOrToken(t *testing.T) {
	raw := errors.New("Post https://api.telegram.org/bot123456:SECRET/sendMessage: connection refused")
	if got := safeProviderFailure(raw); got != "alert provider request failed" {
		t.Fatalf("provider failure = %q", got)
	}
	if got := safeProviderFailure(context.DeadlineExceeded); got != "alert provider request timed out" {
		t.Fatalf("deadline failure = %q", got)
	}
}
