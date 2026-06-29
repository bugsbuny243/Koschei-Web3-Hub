package http

import (
	"testing"
	"time"
)

func TestSensitiveRuleForLiveRiskBadgeRoute(t *testing.T) {
	rule, ok := sensitiveRuleForPath("/api/v1/risk/badge")
	if !ok {
		t.Fatal("live risk badge route is not rate limited")
	}
	if rule.Limit != 20 {
		t.Fatalf("risk badge limit = %d, want 20", rule.Limit)
	}
	if rule.Window != time.Minute {
		t.Fatalf("risk badge window = %s, want %s", rule.Window, time.Minute)
	}
}

func TestSensitiveRuleKeepsLegacyRiskBadgeAlias(t *testing.T) {
	if _, ok := sensitiveRuleForPath("/api/v1/security/risk-badge"); !ok {
		t.Fatal("legacy risk badge alias should remain rate limited")
	}
}
