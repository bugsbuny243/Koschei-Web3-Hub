package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAgentDemoPublicRateLimitConfigured(t *testing.T) {
	rule, ok := sensitiveRuleForPath("/api/agents/demo")
	if !ok || rule.Limit > 15 || rule.Window > time.Minute {
		t.Fatalf("public agent demo missing bounded shared rate limit: %+v enabled=%v", rule, ok)
	}
}

func TestAgentDemoRejectsUnboundedUserFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/agents/demo",
		strings.NewReader(`{"user_id":"`+strings.Repeat("u", 129)+`","text":"hello"}`))
	res := httptest.NewRecorder()
	tradePIAgentDemo(res, req)
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized demo identity accepted: %d %s", res.Code, res.Body.String())
	}
}
