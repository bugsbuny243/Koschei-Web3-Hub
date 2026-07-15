package handlers

import (
	"testing"
	"time"
)

func TestPremiumAccessReportsTierQuota(t *testing.T) {
	reset := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	status := decidePremiumAccess(tokenAccessEvaluation{
		GateEnabled: true, Configured: true, WalletVerified: true,
		Tier: "pro", Amount: "250000",
	}, scanQuotaStatus{Tier: "pro", Limit: 100, Used: 7, Remaining: 93, ResetsAt: reset})
	if !status.Active || status.QuotaDaily != 100 || status.QuotaUsedToday != 7 || status.QuotaRemainingToday != 93 {
		t.Fatalf("unexpected quota status: %+v", status)
	}
	if status.QuotaResetsAt == nil || !status.QuotaResetsAt.Equal(reset) {
		t.Fatalf("unexpected reset: %+v", status.QuotaResetsAt)
	}
}
