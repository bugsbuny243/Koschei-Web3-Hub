package handlers

import (
	"fmt"
	"testing"
	"time"

	"koschei/api/internal/services"
)

func serialLaunchFixture(count int, spacing time.Duration, fate string) creatorOutcomeHistoryReport {
	history := newCreatorOutcomeHistoryReport("ci-fixture-creator")
	start := time.Date(2026, 10, 26, 5, 41, 14, 0, time.UTC)
	for i := 0; i < count; i++ {
		at := start.Add(time.Duration(i) * spacing)
		history.Outcomes = append(history.Outcomes, creatorTokenOutcome{
			Mint: fmt.Sprintf("ci-fixture-mint-%02d", i), CreationSignature: fmt.Sprintf("ci-fixture-sig-%02d", i),
			CreationSlot: int64(461548986 + i), CreatedOnChainAt: &at, FateStatus: fate,
			LifecycleStatus: "inactive_without_prior_liquid_observation", EvidenceStatus: "verified_creator_mint_plus_market_snapshot",
		})
	}
	return history
}

func TestSerialLaunchShadowRuleTriggersOnVerifiedBurstOfInactiveTokens(t *testing.T) {
	// Mirrors the production scan: 40 verified prior tokens in 11 minutes, all inactive.
	rule := evaluateSerialLaunchShadowRule("ci-fixture-target", serialLaunchFixture(40, 16*time.Second, services.ActorTokenFateInactiveOrDead))
	if !rule.Triggered || rule.Status != "triggered_shadow" || rule.MaxLaunchesPerHour != 40 || rule.InactiveRatio != 1 {
		t.Fatalf("verified burst not reported: %+v", rule)
	}
	if rule.Mode != "shadow" || rule.GradeEffect != "none_shadow" || rule.PromotionStatus != "shadow_only_not_promoted" {
		t.Fatalf("shadow rule must not claim a grade effect: %+v", rule)
	}
	if rule.RugClaimed || rule.WrongdoingClaimed || len(rule.Evidence) != 40 || rule.Evidence[0].CreationSignature == "" {
		t.Fatalf("shadow rule must cite each creation and claim no wrongdoing: %+v", rule)
	}
}

func TestSerialLaunchShadowRuleStaysQuietWithoutTheFullPattern(t *testing.T) {
	spread := evaluateSerialLaunchShadowRule("ci-fixture-target", serialLaunchFixture(20, 24*time.Hour, services.ActorTokenFateInactiveOrDead))
	if spread.Triggered || spread.Status != "not_triggered" || spread.MaxLaunchesPerHour != 1 {
		t.Fatalf("tokens launched a day apart are not a burst: %+v", spread)
	}

	alive := serialLaunchFixture(20, time.Minute, services.ActorTokenFateInactiveOrDead)
	for i := 0; i < 5; i++ {
		alive.Outcomes[i].FateStatus = services.ActorTokenFateActive
	}
	if rule := evaluateSerialLaunchShadowRule("ci-fixture-target", alive); rule.Triggered || rule.InactiveRatio != 0.75 {
		t.Fatalf("a 75%% inactive sample is below the threshold: %+v", rule)
	}

	few := evaluateSerialLaunchShadowRule("ci-fixture-target", serialLaunchFixture(9, time.Minute, services.ActorTokenFateInactiveOrDead))
	if few.Triggered || few.Status != "insufficient_evidence" {
		t.Fatalf("nine tokens are insufficient evidence: %+v", few)
	}

	unknown := serialLaunchFixture(30, time.Minute, "market_data_unavailable")
	if rule := evaluateSerialLaunchShadowRule("ci-fixture-target", unknown); rule.Triggered || rule.Status != "insufficient_evidence" || rule.KnownFateTokens != 0 {
		t.Fatalf("tokens without a known fate must not count as inactive: %+v", rule)
	}
}

func TestSerialLaunchShadowRuleExcludesTheScannedMintAndUnverifiedRows(t *testing.T) {
	history := serialLaunchFixture(10, time.Minute, services.ActorTokenFateInactiveOrDead)
	history.Outcomes[3].EvidenceStatus = "observed_only"
	rule := evaluateSerialLaunchShadowRule(history.Outcomes[0].Mint, history)
	if rule.VerifiedPriorTokens != 8 || rule.Triggered {
		t.Fatalf("scanned mint and unverified rows must be excluded: %+v", rule)
	}
}

// Creation transactions without blockTime leave outcomes undated. Those must
// keep the burst unknown instead of reporting "at most 0 within one hour".
func TestSerialLaunchShadowRuleKeepsBurstUnknownWithoutCreationTimes(t *testing.T) {
	history := serialLaunchFixture(12, time.Minute, services.ActorTokenFateInactiveOrDead)
	for i := range history.Outcomes {
		history.Outcomes[i].CreatedOnChainAt = nil
	}
	if rule := evaluateSerialLaunchShadowRule("ci-fixture-target", history); rule.Triggered || rule.Status != "insufficient_evidence" {
		t.Fatalf("undated outcomes must leave the burst unknown: %+v", rule)
	}

	partly := serialLaunchFixture(12, 24*time.Hour, services.ActorTokenFateInactiveOrDead)
	for i := 0; i < 9; i++ {
		partly.Outcomes[i].CreatedOnChainAt = nil
	}
	if rule := evaluateSerialLaunchShadowRule("ci-fixture-target", partly); rule.Status != "insufficient_evidence" {
		t.Fatalf("nine undated tokens could complete the burst: %+v", rule)
	}

	spread := serialLaunchFixture(20, 24*time.Hour, services.ActorTokenFateInactiveOrDead)
	spread.Outcomes[0].CreatedOnChainAt = nil
	if rule := evaluateSerialLaunchShadowRule("ci-fixture-target", spread); rule.Status != "not_triggered" {
		t.Fatalf("one undated token cannot complete a burst of ten here: %+v", rule)
	}
}
