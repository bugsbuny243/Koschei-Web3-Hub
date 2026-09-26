package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"koschei/api/internal/web3"
)

func TestPlanSecurityRadarReplayPageStopsAtExactWatermark(t *testing.T) {
	cursor := securityRadarReplayCursor{WatermarkSignature: "old", WatermarkSlot: 100}
	page := []securityRadarReplaySignature{
		{Signature: "new-3", Slot: 103},
		{Signature: "new-2", Slot: 102},
		{Signature: "new-1", Slot: 101},
		{Signature: "old", Slot: 100},
		{Signature: "older", Slot: 99},
	}
	plan := planSecurityRadarReplayPage(cursor, page, 100)
	if !plan.ReachedWatermark {
		t.Fatal("expected exact recovery watermark to be reached")
	}
	if len(plan.Replay) != 3 {
		t.Fatalf("expected 3 missing signatures, got %d", len(plan.Replay))
	}
	if plan.NextBefore != "" {
		t.Fatalf("caught-up page must not request pagination, got %q", plan.NextBefore)
	}
	if plan.HeadSignature != "new-3" || plan.HeadSlot != 103 {
		t.Fatalf("unexpected scan head %q@%d", plan.HeadSignature, plan.HeadSlot)
	}
}

func TestPlanSecurityRadarReplayPageConsumesWholeWatermarkSlot(t *testing.T) {
	cursor := securityRadarReplayCursor{WatermarkSignature: "forked-away", WatermarkSlot: 100}
	page := []securityRadarReplaySignature{
		{Signature: "new", Slot: 101},
		{Signature: "same-slot-a", Slot: 100},
		{Signature: "same-slot-b", Slot: 100},
		{Signature: "older", Slot: 99},
	}
	plan := planSecurityRadarReplayPage(cursor, page, 100)
	if !plan.ReachedWatermark {
		t.Fatal("expected slot boundary fallback to close the gap")
	}
	if len(plan.Replay) != 3 {
		t.Fatalf("same-slot signatures must be replayed before stopping; got %d", len(plan.Replay))
	}
	if plan.Replay[1].Signature != "same-slot-a" || plan.Replay[2].Signature != "same-slot-b" {
		t.Fatalf("watermark slot was not preserved: %#v", plan.Replay)
	}
}

func TestPlanSecurityRadarReplayPageCarriesIndependentScanHeadAcrossPages(t *testing.T) {
	cursor := securityRadarReplayCursor{
		WatermarkSignature: "old",
		WatermarkSlot:      10,
		ScanHeadSignature:  "head-from-page-one",
		ScanHeadSlot:       30,
	}
	page := []securityRadarReplaySignature{
		{Signature: "page-two-a", Slot: 20},
		{Signature: "page-two-b", Slot: 19},
	}
	plan := planSecurityRadarReplayPage(cursor, page, 2)
	if plan.HeadSignature != cursor.ScanHeadSignature || plan.HeadSlot != cursor.ScanHeadSlot {
		t.Fatalf("scan head changed during pagination: %q@%d", plan.HeadSignature, plan.HeadSlot)
	}
	if plan.NextBefore != "page-two-b" {
		t.Fatalf("expected deterministic pagination cursor, got %q", plan.NextBefore)
	}
	if plan.ReachedWatermark {
		t.Fatal("watermark should not be reached yet")
	}
}

func TestReplaySignatureFailedDistinguishesNullFromFailure(t *testing.T) {
	if replaySignatureFailed(json.RawMessage("null")) {
		t.Fatal("null err must represent a successful signature")
	}
	if replaySignatureFailed(nil) {
		t.Fatal("missing err field must not be treated as a failure")
	}
	if !replaySignatureFailed(json.RawMessage(`{"InstructionError":[1,"Custom"]}`)) {
		t.Fatal("non-null transaction error must be treated as failed")
	}
}

func TestGapHealerFetchSignaturePageHonorsBackgroundRPCBudget(t *testing.T) {
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "true")
	t.Setenv("SOLANA_RPC_BUDGET_MAX_REQUESTS", "1")
	t.Setenv("SOLANA_RPC_BUDGET_WINDOW_SECONDS", "3600")
	resetSolanaRPCBudgetForTest()
	t.Cleanup(resetSolanaRPCBudgetForTest)

	if err := reserveSolanaRPCBudget(context.Background(), "exhaust-budget"); err != nil {
		t.Fatal(err)
	}

	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":[],"id":1}`))
	}))
	defer server.Close()

	healer := &securityRadarGapHealer{RPCURL: server.URL, HTTPClient: server.Client()}
	_, err := healer.fetchSignaturePage(context.Background(), "11111111111111111111111111111111", "", 1)
	if err == nil {
		t.Fatal("expected background RPC budget exhaustion")
	}
	if _, ok := solanaRPCBudgetResetAt(err); !ok {
		t.Fatalf("expected typed RPC budget error, got %v", err)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("gap healer must not hit upstream after budget exhaustion, hits=%d", got)
	}
}


func TestGapHealerPublishes429CooldownToSharedProviderGovernor(t *testing.T) {
	t.Setenv("SOLANA_RPC_GOVERNOR_ENABLED", "true")
	t.Setenv("SOLANA_RPC_LIMIT_SAVER_ENABLED", "false")
	t.Setenv("SOLANA_RPC_429_COOLDOWN_SECONDS", "30")
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "false")
	web3.ResetSolanaRPCProviderGovernorForTest()
	t.Cleanup(web3.ResetSolanaRPCProviderGovernorForTest)

	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	healer := &securityRadarGapHealer{RPCURL: server.URL, HTTPClient: server.Client()}
	_, err := healer.fetchSignaturePage(context.Background(), "11111111111111111111111111111111", "", 1)
	if err == nil {
		t.Fatal("expected 429 error")
	}
	until, cooling := web3.SolanaRPCProviderCooldown(server.URL)
	if !cooling || time.Until(until) < 20*time.Second {
		t.Fatalf("provider cooldown not published: cooling=%t until=%s", cooling, until)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err = healer.fetchSignaturePage(ctx, "11111111111111111111111111111111", "", 1)
	if err == nil {
		t.Fatal("expected second request to be blocked by shared cooldown")
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("shared cooldown must block second upstream hit, hits=%d", got)
	}
}
