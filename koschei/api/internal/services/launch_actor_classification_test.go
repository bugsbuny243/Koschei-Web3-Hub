package services

import (
	"testing"
	"time"
)

func TestClassifyLaunchActorsDeterministicLabels(t *testing.T) {
	launch := time.Unix(1_700_000_000, 0).UTC()
	trades := []LaunchTrade{
		{Trader: "sniper", Side: "buy", Slot: 101, BlockTime: launch.Add(20 * time.Second), TokenAmount: 100},
		{Trader: "rhythm", Side: "buy", Slot: 110, BlockTime: launch.Add(2 * time.Minute), TokenAmount: 10},
		{Trader: "rhythm", Side: "sell", Slot: 111, BlockTime: launch.Add(2*time.Minute + 10*time.Second), TokenAmount: 1},
		{Trader: "rhythm", Side: "buy", Slot: 112, BlockTime: launch.Add(2*time.Minute + 20*time.Second), TokenAmount: 2},
		{Trader: "rhythm", Side: "sell", Slot: 113, BlockTime: launch.Add(2*time.Minute + 30*time.Second), TokenAmount: 1},
		{Trader: "rhythm", Side: "buy", Slot: 114, BlockTime: launch.Add(2*time.Minute + 40*time.Second), TokenAmount: 2},
		{Trader: "flipper", Side: "buy", Slot: 120, BlockTime: launch.Add(5 * time.Minute), TokenAmount: 100},
		{Trader: "flipper", Side: "sell", Slot: 121, BlockTime: launch.Add(10 * time.Minute), TokenAmount: 90},
		{Trader: "acc", Side: "buy", Slot: 130, BlockTime: launch.Add(10 * time.Minute), TokenAmount: 10},
		{Trader: "acc", Side: "buy", Slot: 131, BlockTime: launch.Add(25 * time.Minute), TokenAmount: 10},
		{Trader: "acc", Side: "buy", Slot: 132, BlockTime: launch.Add(45 * time.Minute), TokenAmount: 10},
	}
	profiles := classifyLaunchActors(trades, 100, launch, 3)
	byWallet := map[string]LaunchActorProfile{}
	for _, profile := range profiles {
		byWallet[profile.OwnerWallet] = profile
	}
	if got := byWallet["sniper"].Label; got != LaunchLabelSniperBot {
		t.Fatalf("sniper label=%s", got)
	}
	if got := byWallet["rhythm"].Label; got != LaunchLabelRhythmBot {
		t.Fatalf("rhythm label=%s", got)
	}
	if got := byWallet["flipper"].Label; got != LaunchLabelFlipper {
		t.Fatalf("flipper label=%s", got)
	}
	if got := byWallet["acc"].Label; got != LaunchLabelAccumulator {
		t.Fatalf("acc label=%s", got)
	}
}

func TestLaunchForensicsRiskAbsenceIsNotSafetySignal(t *testing.T) {
	contribution, floor := launchForensicsRisk([]LaunchActorProfile{{OwnerWallet: "A", Label: "HISTORY_NOT_CAPTURED"}})
	if contribution != 0 || floor != 0 {
		t.Fatalf("missing history contribution=%d floor=%d", contribution, floor)
	}
	contribution, floor = launchForensicsRisk([]LaunchActorProfile{{OwnerWallet: "A", TradeCount: 2, Label: LaunchLabelOrganic}})
	if contribution != 0 || floor != 0 {
		t.Fatalf("organic contribution=%d floor=%d", contribution, floor)
	}
}

func TestClassifyLaunchActorsRanksOnlyBuyersContiguously(t *testing.T) {
	launch := time.Unix(1_700_000_000, 0).UTC()
	profiles := classifyLaunchActors([]LaunchTrade{
		{Trader: "sell-only", Side: "sell", Slot: 99, BlockTime: launch.Add(-time.Second), TokenAmount: 1},
		{Trader: "buyer-a", Side: "buy", Slot: 101, BlockTime: launch.Add(time.Second), TokenAmount: 1},
		{Trader: "buyer-b", Side: "buy", Slot: 102, BlockTime: launch.Add(2 * time.Second), TokenAmount: 1},
	}, 100, launch, 3)
	byWallet := map[string]LaunchActorProfile{}
	for _, profile := range profiles {
		byWallet[profile.OwnerWallet] = profile
	}
	if byWallet["sell-only"].EntryRank != 0 || byWallet["buyer-a"].EntryRank != 1 || byWallet["buyer-b"].EntryRank != 2 {
		t.Fatalf("unexpected entry ranks: %#v", byWallet)
	}
}

func TestClassifyLaunchActorDoesNotCallPreReferenceBuySniper(t *testing.T) {
	launch := time.Unix(1_700_000_000, 0).UTC()
	profile := classifyLaunchActor("buyer", []LaunchTrade{{
		Trader: "buyer", Side: "buy", Slot: 99, BlockTime: launch.Add(-10 * time.Second), TokenAmount: 1,
	}}, 100, launch, 3)
	if profile.Sniper || profile.Label == LaunchLabelSniperBot {
		t.Fatalf("pre-reference buy must not be a sniper: %#v", profile)
	}
	if !profile.LaunchSlotKnown || profile.SlotOffsetFromLaunch != -1 || !profile.LaunchTimeKnown || profile.MinutesAfterLaunch >= 0 {
		t.Fatalf("negative timing evidence should be preserved honestly: %#v", profile)
	}
}
