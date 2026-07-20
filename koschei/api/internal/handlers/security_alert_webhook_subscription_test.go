package handlers

import (
	"reflect"
	"testing"

	"koschei/api/internal/alerts"
)

func TestNormalizeSecurityAlertEventsDefaultsToWildcard(t *testing.T) {
	got, err := normalizeSecurityAlertEvents(nil)
	if err != nil {
		t.Fatalf("normalize default events: %v", err)
	}
	want := []string{alerts.EventSecurityAlertCreated}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events=%#v want=%#v", got, want)
	}
}

func TestNormalizeSecurityAlertEventsDeduplicatesAndSorts(t *testing.T) {
	got, err := normalizeSecurityAlertEvents([]string{
		alerts.EventTransactionGuardDecision,
		alerts.EventARVISVerdictCreated,
		alerts.EventTransactionGuardDecision,
	})
	if err != nil {
		t.Fatalf("normalize selected events: %v", err)
	}
	want := []string{alerts.EventARVISVerdictCreated, alerts.EventTransactionGuardDecision}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events=%#v want=%#v", got, want)
	}
}

func TestNormalizeSecurityAlertEventsRejectsUnknownType(t *testing.T) {
	if _, err := normalizeSecurityAlertEvents([]string{"customer.secret.event"}); err == nil {
		t.Fatal("unknown security event type was accepted")
	}
}
