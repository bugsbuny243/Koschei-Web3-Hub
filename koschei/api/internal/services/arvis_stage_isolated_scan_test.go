package services

import (
	"context"
	"testing"
	"time"
)

func TestForegroundRadarCollectorsUseIndependentStageContexts(t *testing.T) {
	req := SecurityRadarRequest{Target: "Mint111", Network: "solana-mainnet", Mode: "owner_unified_manual_scan"}
	parent, parentCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer parentCancel()

	accountCtx, accountCancel := radarEvidenceStageContext(parent, req, "account")
	accountDeadline, ok := accountCtx.Deadline()
	if !ok {
		t.Fatal("account stage missing deadline")
	}
	accountCancel()
	if parent.Err() != nil {
		t.Fatalf("canceling account stage canceled parent: %v", parent.Err())
	}

	signatureCtx, signatureCancel := radarEvidenceStageContext(parent, req, "signatures")
	defer signatureCancel()
	if signatureCtx.Err() != nil {
		t.Fatalf("later stage inherited canceled/expired state: %v", signatureCtx.Err())
	}
	signatureDeadline, ok := signatureCtx.Deadline()
	if !ok {
		t.Fatal("signature stage missing deadline")
	}
	if !signatureDeadline.After(accountDeadline) {
		t.Fatalf("expected later stage to receive a fresh deadline: account=%s signatures=%s", accountDeadline, signatureDeadline)
	}
}

func TestForegroundRadarStageBudgetsUseExistingScanBudgets(t *testing.T) {
	t.Setenv("ARVIS_WALLET_SCAN_TIMEOUT_SECONDS", "31")
	t.Setenv("ARVIS_LAUNCH_SCAN_TIMEOUT_SECONDS", "27")
	req := SecurityRadarRequest{Mode: "customer_token_scan"}

	if got := radarEvidenceStageTimeout(req, "holder_roles"); got != 31*time.Second {
		t.Fatalf("holder roles timeout=%s want=31s", got)
	}
	if got := radarEvidenceStageTimeout(req, "holder_cluster"); got != 31*time.Second {
		t.Fatalf("holder cluster timeout=%s want=31s", got)
	}
	if got := radarEvidenceStageTimeout(req, "signatures"); got != 27*time.Second {
		t.Fatalf("signatures timeout=%s want=27s", got)
	}
	if got := radarEvidenceStageTimeout(req, "account"); got != 18*time.Second {
		t.Fatalf("account timeout=%s want=18s", got)
	}
}

func TestBackgroundRadarKeepsBoundedLegacyPath(t *testing.T) {
	req := SecurityRadarRequest{Mode: "polling"}
	if useStageIsolatedRadarScan(context.Background(), req) {
		t.Fatal("background polling must not opt into foreground stage budgets")
	}
	if got := radarEvidenceStageTimeout(req, "signatures"); got != 6500*time.Millisecond {
		t.Fatalf("background timeout=%s want=6.5s", got)
	}
}
