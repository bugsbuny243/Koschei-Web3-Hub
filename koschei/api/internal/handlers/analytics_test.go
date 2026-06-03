package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeAnalyticsEvent(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/analytics/event", nil)
	r.Header.Set("Referer", "https://example.com/source")
	r.Header.Set("User-Agent", "KoscheiTest/1.0")

	event, err := normalizeAnalyticsEvent(analyticsEventRequest{
		EventName: " Login_Success ",
		Email:     " USER@Example.COM ",
		Path:      "/login?next=hub",
		Metadata:  []byte(`{"method":"password"}`),
	}, r)
	if err != nil {
		t.Fatalf("normalizeAnalyticsEvent() error = %v", err)
	}
	if event.EventName != "login_success" {
		t.Fatalf("EventName = %q, want login_success", event.EventName)
	}
	if !event.Email.Valid || event.Email.String != "user@example.com" {
		t.Fatalf("Email = %#v, want valid lowercase email", event.Email)
	}
	if event.Path != "/login?next=hub" {
		t.Fatalf("Path = %q, want /login?next=hub", event.Path)
	}
	if event.Referrer != "https://example.com/source" {
		t.Fatalf("Referrer = %q, want request referer", event.Referrer)
	}
	if event.UserAgent != "KoscheiTest/1.0" {
		t.Fatalf("UserAgent = %q, want request user agent", event.UserAgent)
	}
}

func TestNormalizeAnalyticsEventRejectsInvalidEvent(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/analytics/event", nil)
	_, err := normalizeAnalyticsEvent(analyticsEventRequest{EventName: "unknown_event"}, r)
	if err != errInvalidAnalyticsEvent {
		t.Fatalf("error = %v, want %v", err, errInvalidAnalyticsEvent)
	}
}

func TestLimitAnalyticsField(t *testing.T) {
	value := strings.Repeat("x", 2050)
	if got := limitAnalyticsField(value); len(got) != 2048 {
		t.Fatalf("len(limitAnalyticsField()) = %d, want 2048", len(got))
	}
}
