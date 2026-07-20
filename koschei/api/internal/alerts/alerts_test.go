package alerts

import "testing"

func TestNormalizeEventProducesStableDedupeKey(t *testing.T) {
	first := normalizeEvent(Event{Source: " arvis ", EventType: "ARVIS.VERDICT.CREATED", Severity: "warning", Target: "mint", Title: "Risk", Message: "Evidence"})
	second := normalizeEvent(Event{Source: "arvis", EventType: "arvis.verdict.created", Severity: "medium", Target: "mint", Title: "Risk", Message: "Evidence"})
	if first.DedupeKey == "" || first.DedupeKey != second.DedupeKey {
		t.Fatalf("dedupe keys differ: %q %q", first.DedupeKey, second.DedupeKey)
	}
	if first.Severity != "medium" {
		t.Fatalf("severity = %q", first.Severity)
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
