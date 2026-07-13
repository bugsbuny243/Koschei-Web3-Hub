package services

import (
	"strings"
	"testing"
)

func TestRepeatDominantRiskWeightRequiresTwentyPercentInTwoTokens(t *testing.T) {
	if got := RepeatDominantRiskWeight(19.99, 2); got != 0 {
		t.Fatalf("risk=%d", got)
	}
	if got := RepeatDominantRiskWeight(58.71, 2); got < 70 {
		t.Fatalf("risk=%d", got)
	}
	if got := RepeatDominantRiskWeight(78.67, 3); got <= RepeatDominantRiskWeight(58.71, 2) {
		t.Fatalf("risk did not scale: %d", got)
	}
}

func TestRepeatDominantEvidenceNamesBothObservedMintsWithoutIdentityClaim(t *testing.T) {
	matches := []RepeatDominantHolderMatch{
		{Mint: "9cRCn9rGT8V2imeM2BaKs13yhMEais3ruM3rPvTGpump", Percentage: 58.71, Rank: 1, ScannedAt: "2026-07-12T10:00:00Z"},
		{Mint: "6QPvGr1L7aXGybpGKvvG8LtFDV9dRzK6QbSpRNJJonYM", Percentage: 78.67, Rank: 1, ScannedAt: "2026-07-13T10:00:00Z"},
	}
	line := RepeatDominantEvidenceLine("GV6UUmNxxVdC52", matches, 30)
	for _, expected := range []string{"REPEAT DOMINANT HOLDER", "2 farklı token", "9cRCn9rG", "6QPvGr1L", "kimlik veya niyet iddiası değildir"} {
		if !strings.Contains(line, expected) {
			t.Fatalf("missing %q: %s", expected, line)
		}
	}
}

func TestRepeatDominantEvidenceReachesOwnerRowBadgeFields(t *testing.T) {
	holder := HolderIntelligence{Available: true, Rows: []HolderIntelligenceRow{{OwnerWallet: "GV6", OwnerResolved: true, RiskBearing: true}}}
	evidence := []RepeatDominantHolderEvidence{{OwnerWallet: "GV6", TokenCount: 2, ObservationWindow: "son 30 gün Koschei gözlemi", RiskWeight: 72, EvidenceLine: "REPEAT", Matches: []RepeatDominantHolderMatch{{Mint: "A", Percentage: 58.71}, {Mint: "B", Percentage: 78.67}}}}
	got := ApplyRepeatDominantHolderEvidenceToHolderIntelligence(holder, evidence)
	row := got.Rows[0]
	if !row.RepeatDominantHolder || row.RepeatDominantTokenCount != 2 || row.RepeatDominantRiskWeight != 72 || got.RepeatDominantHolderCount != 1 {
		t.Fatalf("row=%#v holder=%#v", row, got)
	}
}
