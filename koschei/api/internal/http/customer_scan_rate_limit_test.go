package http

import (
	"testing"
	"time"
)

func TestSensitiveRuleCoversPublicCustomerScanRoutes(t *testing.T) {
	for _, path := range []string{"/api/scan", "/api/scan/approval"} {
		rule, ok := sensitiveRuleForPath(path)
		if !ok {
			t.Fatalf("public customer scan route %s is not rate limited", path)
		}
		if rule.Limit != 10 || rule.Window != time.Minute {
			t.Fatalf("public customer scan route %s rate limit = %+v, want 10/minute", path, rule)
		}
	}
}
